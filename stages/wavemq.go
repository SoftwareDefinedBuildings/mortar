package stages

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"time"

	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/gtfierro/xboswave/grpcserver"
	xbospb "github.com/gtfierro/xboswave/proto"
	"github.com/immesys/wavemq/mqpb"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type WAVEMQFrontendStageConfig struct {
	SiteRouter string
	Namespace  string
	EntityFile string

	BaseURI    string
	ServerName string
}

type WAVEMQFrontendStage struct {
	client         mqpb.WAVEMQClient
	output         chan Context
	perspective    *mqpb.Perspective
	namespaceBytes []byte
	sem            chan struct{}
}

func NewWAVEMQFrontendStage(cfg *WAVEMQFrontendStageConfig) (*WAVEMQFrontendStage, error) {

	ctx := context.Background()

	//setup namespace
	namespaceBytes, err := base64.URLEncoding.DecodeString(cfg.Namespace)
	if err != nil {
		log.Fatalf("failed to decode namespace: %v", err)
	}

	// load perspective
	perspectivefile, err := ioutil.ReadFile(cfg.EntityFile)
	if err != nil {
		log.Fatalf("could not load entity (%v) you might need to create one and grant it permissions\n", err)
	}
	perspective := &mqpb.Perspective{
		EntitySecret: &mqpb.EntitySecret{
			DER: perspectivefile,
		},
	}

	//setup connection to wavemq
	conn, err := grpc.DialContext(ctx, cfg.SiteRouter, grpc.WithInsecure(), grpc.FailOnNonTempDialError(true), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Could not connect to site router %v", err)
	}

	stage := &WAVEMQFrontendStage{
		output:         make(chan Context),
		client:         mqpb.NewWAVEMQClient(conn),
		perspective:    perspective,
		namespaceBytes: namespaceBytes,
		sem:            make(chan struct{}, 20),
	}
	for i := 0; i < 20; i++ {
		stage.sem <- struct{}{}
	}

	// setup WAVEMQ frontend to Mortar grpc server
	srv, err := grpcserver.NewWaveMQServer(&grpcserver.Config{
		SiteRouter: cfg.SiteRouter,
		EntityFile: cfg.EntityFile,
		Namespace:  cfg.Namespace,
		BaseURI:    cfg.BaseURI,
		ServerName: cfg.ServerName,
	})
	if err != nil {
		log.Fatalf("Could not create wavemq frontend: %v", err)
	}

	srv.OnUnary("Qualify", func(call *xbospb.UnaryCall) (*xbospb.UnaryResponse, error) {
		var request mortarpb.QualifyRequest
		err := grpcserver.GetUnaryPayload(call, &request)
		if err != nil {
			return nil, err
		}
		reply, err := stage.Qualify(context.Background(), &request)
		resp, err := grpcserver.MakeUnaryResponse(call, reply, err)
		return resp, err
	})

	srv.OnStream("Fetch", func(call *xbospb.StreamingCall, stream *grpcserver.StreamContext) error {
		var msg mortarpb.FetchRequest
		err := grpcserver.GetStreamingPayload(call, &msg)
		if err != nil {
			return err
		}
		err = stage.Fetch(&msg, stream)
		stream.Finish(call, err)
		return err
	})

	srv.Serve()
	return stage, nil
}

// get the stage we pull from
func (stage *WAVEMQFrontendStage) GetUpstream() Stage {
	return nil
}

// set the stage we pull from
func (stage *WAVEMQFrontendStage) SetUpstream(upstream Stage) {
	//has no upstream
}

func (stage *WAVEMQFrontendStage) GetQueue() chan Context {
	return stage.output
}
func (stage *WAVEMQFrontendStage) String() string {
	return "<| wavemq frontend stage |>"
}

func (stage *WAVEMQFrontendStage) Qualify(ctx context.Context, request *mortarpb.QualifyRequest) (*mortarpb.QualifyResponse, error) {
	t := time.Now()
	defer func() {
		log.Info("Qualify took ", time.Since(t))
		qualifyProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

	activeQueries.Inc()
	defer activeQueries.Dec()
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
func (stage *WAVEMQFrontendStage) Fetch(request *mortarpb.FetchRequest, client mortarpb.Mortar_FetchServer) error {
	t := time.Now()
	defer func() {
		log.Info("Fetch took ", time.Since(t))
		fetchProcessingTimes.Observe(float64(time.Since(t).Nanoseconds() / 1e6))
	}()

	activeQueries.Inc()
	defer activeQueries.Dec()

	ctx, cancel := context.WithTimeout(client.Context(), requestTimeout)
	defer cancel()

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
				err = errors.Wrap(ctx.Err(), "fetch timeout on response")
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
		return errors.New("timeout")
	}

	return nil
}
