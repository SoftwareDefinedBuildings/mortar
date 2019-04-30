package stages

import (
	"context"
	"io/ioutil"
	"net"
	"sync"
	"time"

	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/gtfierro/xboswave/grpcauth"
	eapi "github.com/immesys/wave/eapi/pb"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

type ApiFrontendWAVEAuthStage struct {
	ctx    context.Context
	output chan Context
	sem    chan struct{}
	sync.Mutex
}

type ApiFrontendWAVEAuthStageConfig struct {
	Namespace    string
	Agent        string
	EntityFile   string
	ProofFile    string
	ListenAddr   string
	Upstream     Stage
	StageContext context.Context
}

func NewApiFrontendWAVEAuthStage(cfg *ApiFrontendWAVEAuthStageConfig) (*ApiFrontendWAVEAuthStage, error) {

	stage := &ApiFrontendWAVEAuthStage{
		output: make(chan Context),
		ctx:    cfg.StageContext,
		sem:    make(chan struct{}, 20),
	}
	for i := 0; i < 20; i++ {
		stage.sem <- struct{}{}
	}

	// load perspective
	perspectivefile, err := ioutil.ReadFile(cfg.EntityFile)
	if err != nil {
		log.Fatalf("could not load entity (%v) you might need to create one and grant it permissions\n", err)
	}
	perspective := &eapi.Perspective{
		EntitySecret: &eapi.EntitySecret{
			DER: perspectivefile,
		},
	}

	serverwavecreds, err := grpcauth.NewServerCredentials(perspective, cfg.Agent)
	if err != nil {
		log.Fatalf("Could not use WAVE frontend: %v", err)
	}

	server := grpc.NewServer(grpc.Creds(serverwavecreds))

	l, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not listen on address %s", cfg.ListenAddr)
	}
	mortarpb.RegisterMortarServer(server, stage)
	serverwavecreds.AddServiceInfo(server)
	ns, _, err := serverwavecreds.AddGRPCProofFile(cfg.ProofFile)
	if err != nil {
		log.Fatalf("Could not load proof: %v", err)
	}

	log.Infof("Authorized for namespace %s", ns)

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
func (stage *ApiFrontendWAVEAuthStage) GetUpstream() Stage {
	return nil
}

// set the stage we pull from
func (stage *ApiFrontendWAVEAuthStage) SetUpstream(upstream Stage) {
	//has no upstream
}

func (stage *ApiFrontendWAVEAuthStage) GetQueue() chan Context {
	return stage.output
}
func (stage *ApiFrontendWAVEAuthStage) String() string {
	return "<| api frontend wave auth stage |>"
}

// identify which sites meet the requirements of the queries
func (stage *ApiFrontendWAVEAuthStage) Qualify(ctx context.Context, request *mortarpb.QualifyRequest) (*mortarpb.QualifyResponse, error) {

	t := time.Now()
	defer func() {
		log.Info("Qualify took ", time.Since(t))
		qualifyProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

	activeQueries.Inc()
	defer activeQueries.Dec()

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
		return nil, errors.Wrap(ctx.Err(), "qualify timeout on dispatching query")
	}

	select {
	case resp := <-responseChan:
		//close(responseChan)
		if resp.Error != "" {
			log.Warning(resp.Error)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "qualify timeout on getting query response")
	}

	return nil, errors.New("impossible error")

}

// pull data from Mortar
// gets called from frontend by GRPC server
func (stage *ApiFrontendWAVEAuthStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	t := time.Now()
	defer func() {
		log.Info("Fetch took ", time.Since(t))
		fetchProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

	activeQueries.Inc()
	defer activeQueries.Dec()

	ctx, cancel := context.WithTimeout(client.Context(), requestTimeout)
	defer cancel()

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

	responseChan := make(chan *mortarpb.FetchResponse)
	queryCtx := Context{
		ctx:     ctx,
		request: *request,
		done:    responseChan,
	}
	ret := make(chan error)
	go func() {
		var err error

	sendloop:
		for {
			select {
			case resp := <-responseChan:
				if resp == nil {
					// if this is nil then we are done, but there's no error (yet)
					break sendloop
				} else if err = client.Send(resp); err != nil {
					// we have an error on sending, so we tear it all down
					log.Error(errors.Wrap(err, "Error on sending"))
					// have to remember to call cancel() here
					finishResponse(resp)
					cancel()
					break sendloop
				} else {
					// happy path
					finishResponse(resp)
					messagesSent.Inc()
				}
			case <-ctx.Done():
				// this branch gets triggered because context gets cancelled
				err = errors.Wrapf(ctx.Err(), "fetch timeout on response %v", queryCtx.errors)
				break sendloop
			}
		}
		ret <- err
	}()

	select {
	case stage.output <- queryCtx:
	case <-ctx.Done():
		return errors.New("timeout")
	}

	select {
	case e := <-ret:
		log.Error("Got Error in ret ", e)
		return e
	case <-ctx.Done():
		log.Error("timing out on waiting for result in fetch")
		return errors.New("timeout")
	}

	return nil
}

func (stage *ApiFrontendWAVEAuthStage) GetAPIKey(ctx context.Context, request *mortarpb.GetAPIKeyRequest) (*mortarpb.APIKeyResponse, error) {
	return &mortarpb.APIKeyResponse{}, nil
}
