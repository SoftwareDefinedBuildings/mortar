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
	requestTimeout  = 15 * time.Minute
)

type ApiFrontendBasicStage struct {
	ctx    context.Context
	output chan Context
	auth   *CognitoAuth
	sem    chan struct{}
	sync.Mutex
}

type ApiFrontendBasicStageConfig struct {
	TLSCrtFile   string
	TLSKeyFile   string
	ListenAddr   string
	AuthConfig   CognitoAuthConfig
	Upstream     Stage
	StageContext context.Context
}

func NewApiFrontendBasicStage(cfg *ApiFrontendBasicStageConfig) (*ApiFrontendBasicStage, error) {
	stage := &ApiFrontendBasicStage{
		output: make(chan Context),
		ctx:    cfg.StageContext,
		sem:    make(chan struct{}, 20),
	}
	for i := 0; i < 20; i++ {
		stage.sem <- struct{}{}
	}

	auth, err := NewCognitoAuth(cfg.AuthConfig)
	if err != nil {
		return nil, err
	}
	stage.auth = auth

	var server *grpc.Server

	// handle TLS if it is configured
	log.Infof("Cert file: %s, Key file: %s", cfg.TLSCrtFile, cfg.TLSKeyFile)
	if cfg.TLSCrtFile != "" && cfg.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLSCrtFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "could not load TLS keys")
		}
		// this is the lets-encrypt code that we aren't using
		//tls, err := GetTLS(cfg.TLSHost, cfg.TLSCacheDir)
		//if err != nil {
		//	return nil, errors.Wrap(err, "Could not get TLS cert")
		//}
		//creds := credentials.NewTLS(tls)
		log.Info("Using TLS")
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
	defer qualifyProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))

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

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	// prepare context for the execution
	responseChan := make(chan *mortarpb.QualifyResponse)
	queryCtx := Context{
		ctx:             ctx,
		qualify_request: *request,
		qualify_done:    responseChan,
	}

	select {
	case stage.output <- queryCtx:
	case <-ctx.Done():
		return nil, errors.New("timeout")
	}

	select {
	case resp := <-responseChan:
		//close(responseChan)
		if resp.Error != "" {
			log.Warning(resp.Error)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, errors.New("timeout")
	}

	return nil, errors.New("impossible error")

}

// pull data from Mortar
// gets called from frontend by GRPC server
func (stage *ApiFrontendBasicStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	t := time.Now()
	defer fetchProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	authRequests.Inc()
	activeQueries.Inc()
	defer activeQueries.Dec()

	ctx, cancel := context.WithTimeout(client.Context(), requestTimeout)
	defer cancel()

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

	select {
	case sem := <-stage.sem:
		defer func() { stage.sem <- sem }()
	case <-ctx.Done():
		return errors.New("timeout")
	}

	responseChan := make(chan *mortarpb.FetchResponse)
	queryCtx := Context{
		ctx:     ctx,
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
	return e
}

func (stage *ApiFrontendBasicStage) GetAPIKey(ctx context.Context, request *mortarpb.GetAPIKeyRequest) (*mortarpb.APIKeyResponse, error) {
	access, refresh, err := stage.auth.verifyUserPass(request.Username, request.Password)
	return &mortarpb.APIKeyResponse{
		Token:        access,
		Refreshtoken: refresh,
	}, err
}
