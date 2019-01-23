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
	"time"
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
func (stage *ApiFrontendBasicStage) Qualify(ctx context.Context, request *mortarpb.QualifyRequest) (*mortarpb.QualifyResponse, error) {

	t := time.Now()

	activeQueries.Inc()
	defer activeQueries.Dec()
	authRequests.Inc()
	headers, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, unauthorizedErr
	}
	if _tokens, ok := headers["token"]; ok && len(_tokens) > 0 && len(_tokens[0]) > 0 {
		token := _tokens[0]
		if _, authErr := stage.auth.verifyToken(token); authErr != nil {
			return nil, authErr
		}
	} else {
		return nil, errors.New("no auth key")
	}
	authRequestsSuccessful.Inc()
	// here we are authenticated to the service.
	validateErr := validateQualifyRequest(request)
	if validateErr != nil {
		return nil, validateErr
	}

	qualifyQueriesProcessed.Inc()

	// prepare context for the execution
	responseChan := make(chan *mortarpb.QualifyResponse)
	queryCtx := Context{
		ctx:             ctx,
		qualify_request: *request,
		qualify_done:    responseChan,
	}

	stage.output <- queryCtx
	resp := <-responseChan
	//close(responseChan)
	log.Warning(resp.Error)

	qualifyProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))

	return resp, nil
}

// pull data from Mortar
// gets called from frontend by GRPC server
func (stage *ApiFrontendBasicStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	t := time.Now()
	authRequests.Inc()
	activeQueries.Inc()
	defer activeQueries.Dec()
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
	authRequestsSuccessful.Inc()

	// here we are authenticated to the service.
	validateErr := validateFetchRequest(request)
	if validateErr != nil {
		return validateErr
	}

	fetchQueriesProcessed.Inc()

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
			messagesSent.Inc()
			if err = client.Send(resp); err != nil {
				break
			}
		}
		ret <- err
	}()
	stage.output <- queryCtx
	e := <-ret
	fetchProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	return e
}

func (stage *ApiFrontendBasicStage) GetAPIKey(ctx context.Context, request *mortarpb.GetAPIKeyRequest) (*mortarpb.APIKeyResponse, error) {
	access, refresh, err := stage.auth.verifyUserPass(request.Username, request.Password)
	return &mortarpb.APIKeyResponse{
		Token:        access,
		Refreshtoken: refresh,
	}, err
}
