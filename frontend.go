package main

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"net"
	"sync"
)

func init() {
	encoding.RegisterCompressor(encoding.GetCompressor("gzip"))
}

var (
	unauthorizedErr = errors.New("Unauthorized")
)

type ApiFrontendBasicStage struct {
	ctx    context.Context
	output chan Context
	auth   *CognitoAuth
	sync.Mutex
}

type ApiFrontendBasicStageConfig struct {
	TLSHost      string
	TLSCacheDir  string
	ListenAddr   string
	AuthConfig   CognitoAuthConfig
	Upstream     Stage
	StageContext context.Context
}

func NewApiFrontendBasicStage(cfg *ApiFrontendBasicStageConfig) (*ApiFrontendBasicStage, error) {
	stage := &ApiFrontendBasicStage{
		output: make(chan Context),
		ctx:    cfg.StageContext,
	}

	auth, err := NewCognitoAuth(cfg.AuthConfig)
	if err != nil {
		return nil, err
	}
	stage.auth = auth

	var server *grpc.Server

	// handle TLS if it is configured
	if cfg.TLSHost != "" {
		tls, err := GetTLS(cfg.TLSHost, cfg.TLSCacheDir)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get TLS cert")
		}
		creds := credentials.NewTLS(tls)
		server = grpc.NewServer(
			grpc.Creds(creds),
		)
	} else {
		server = grpc.NewServer()
	}

	l, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not listen on address %s", cfg.ListenAddr)
	}
	mortarpb.RegisterMortarServer(server, stage)
	go server.Serve(l)
	log.Infof("Listening GRPC on %s", cfg.ListenAddr)

	return stage, nil
}

// get the stage we pull from
func (stage *ApiFrontendBasicStage) GetUpstream() Stage {
	return nil
}

// set the stage we pull from
func (stage *ApiFrontendBasicStage) SetUpstream(upstream Stage) {
	//has no upstream
}

func (stage *ApiFrontendBasicStage) GetQueue() chan Context {
	return stage.output
}
func (stage *ApiFrontendBasicStage) String() string {
	return "<| api frontend basic stage |>"
}

// identify which sites meet the requirements of the queries
func (stage *ApiFrontendBasicStage) Qualify(context.Context, *mortarpb.QualifyRequest) (*mortarpb.QualifyResponse, error) {
	// have a small problem in the design. Currently, this is the frontend stage that connects to the outside world via exposing a
	// GRPC server. it pushes requests into a channel to get consumed by the rest of the pipeline. The
	// want a "pipeline" struct:
	//   pipeline := MakePipeline(
	//	 	ApiFrontEndInstance,
	//	    BrickQueryInstance,
	//	    TimeseriesInstance,
	//		...,
	//	 )
	// The pipeline struct gives us a request/response interface that abstracts away the pipe and ties the inputs to the outputs.
	return nil, nil
}

// pull data from Mortar
// gets called from frontend by GRPC server
func (stage *ApiFrontendBasicStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	headers, ok := metadata.FromIncomingContext(client.Context())
	if !ok {
		return unauthorizedErr
	}
	if _tokens, ok := headers["token"]; ok && len(_tokens) > 0 && len(_tokens[0]) > 0 {
		token := _tokens[0]
		if _, authErr := stage.auth.verifyToken(token); authErr != nil {
			return authErr
		}
	} else {
		return errors.New("no auth key")
	}

	// here we are authenticated to the service.
	validateErr := validateFetchRequest(request)
	if validateErr != nil {
		return validateErr
	}

	responseChan := make(chan *mortarpb.FetchResponse)
	queryCtx := Context{
		ctx:     client.Context(),
		request: *request,
		done:    responseChan,
	}
	ret := make(chan error)
	go func() {
		var err error
		for resp := range responseChan {
			if err = client.Send(resp); err != nil {
				break
			}
		}
		ret <- err
	}()
	stage.output <- queryCtx
	e := <-ret
	return e
}

func (stage *ApiFrontendBasicStage) GetAPIKey(ctx context.Context, request *mortarpb.GetAPIKeyRequest) (*mortarpb.APIKeyResponse, error) {
	access, refresh, err := stage.auth.verifyUserPass(request.User, request.Pass)
	return &mortarpb.APIKeyResponse{
		Token:        access,
		Refreshtoken: refresh,
	}, err
}
