package stages

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
	requestTimeout  = 60 * time.Minute
)

type ApiFrontendBasicStage struct {
	ctx    context.Context
	output chan *Request
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
		output: make(chan *Request),
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
	go func() {
		for {
			e := server.Serve(l)
			log.Error(errors.Wrap(e, "Error GRPC serving. Restarting in 10 sec"))
			time.Sleep(10 * time.Second)
		}
	}()
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

func (stage *ApiFrontendBasicStage) GetQueue() chan *Request {
	return stage.output
}
func (stage *ApiFrontendBasicStage) String() string {
	return "<| api frontend basic stage |>"
}

// identify which sites meet the requirements of the queries
func (stage *ApiFrontendBasicStage) Qualify(ctx context.Context, request *mortarpb.QualifyRequest) (*mortarpb.QualifyResponse, error) {

	defer func() {
		if r := recover(); r != nil {
			log.Warning("Recovered in qualify", r)
		}
	}()

	t := time.Now()
	defer func() {
		log.Info("Qualify took ", time.Since(t))
		qualifyProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

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

	req := NewQualifyRequest(ctx, request)

	// send the request to the output of this stage so it
	// can be handled by the next stage
	select {
	case stage.output <- req:
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "qualify timeout on dispatching query")
	}

	select {
	case resp := <-req.qualify_responses:
		if resp.Error != "" {
			log.Warning(resp.Error)
		}
		if req.err != nil && resp.Error == "" {
			resp.Error = req.err.Error()
		}
		close(req.qualify_responses)
		return resp, nil
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "qualify timeout on getting query response")
	}

	return nil, errors.New("impossible error")

}

// pull data from Mortar
// gets called from frontend by GRPC server
func (stage *ApiFrontendBasicStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	t := time.Now()
	defer func() {
		log.Info("Fetch took ", time.Since(t))
		fetchProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

	authRequests.Inc()
	activeQueries.Inc()
	defer activeQueries.Dec()

	ctx := client.Context()

	headers, ok := metadata.FromIncomingContext(ctx)
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
		return errors.Wrap(ctx.Err(), "fetch timeout on getting semaphore")
	}

	req := NewFetchRequest(ctx, request)

	ret := make(chan error)
	go func() {
		var err error

	sendloop:
		for {
			select {
			case resp := <-req.fetch_responses:
				if resp == nil {
					// if this is nil then we are done, but there's no error (yet)
					break sendloop
				}
				if err = client.Send(resp); err != nil {
					// we have an error on sending, so we tear it all down
					log.Error(errors.Wrap(err, "Error on sending"))
					finishResponse(resp)
					break sendloop
				} else {
					// happy path
					finishResponse(resp)
					messagesSent.Inc()
				}
			case <-ctx.Done():
				// this branch gets triggered because context gets cancelled
				err = errors.Wrapf(ctx.Err(), "fetch timeout on response %v", req.err)
				break sendloop
			}
		}
		ret <- err
		close(req.fetch_responses)
	}()

	select {
	case stage.output <- req:
	case <-ctx.Done():
		return errors.New("timeout")
	}

	select {
	case e := <-ret:
		if e != nil {
			log.Error("Got Error in ret ", e)
		}
		return e
	case <-ctx.Done():
		log.Error("timing out on waiting for result in fetch")
		return errors.New("timeout")
	}

	return nil
}

func (stage *ApiFrontendBasicStage) GetAPIKey(ctx context.Context, request *mortarpb.GetAPIKeyRequest) (*mortarpb.APIKeyResponse, error) {
	access, refresh, err := stage.auth.verifyUserPass(request.Username, request.Password)
	return &mortarpb.APIKeyResponse{
		Token:        access,
		Refreshtoken: refresh,
	}, err
}
