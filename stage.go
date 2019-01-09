package main

import (
	"context"
	"fmt"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"gopkg.in/btrdb.v4"
	"log"
	"sync"
	"time"
)

var MAX_TIMEOUT = time.Second * 300
var TS_BATCH_SIZE = 10
var errStreamNotExist = errors.New("Stream does not exist")

type Context struct {
	ctx      context.Context
	request  mortarpb.FetchRequest
	response *mortarpb.FetchResponse
	errors   []error
	sync.Mutex
}

func (ctx *Context) addError(err error) {
	ctx.Lock()
	defer ctx.Unlock()
	ctx.errors = append(ctx.errors, err)
}

type Stage interface {
	// get the stage we pull from
	GetUpstream() Stage
	// set the stage we pull from
	SetUpstream(upstream Stage)
	// blocks on internal channel until next "Context" is ready
	GetQueue() chan Context
	String() string
}

type TimeseriesQueryStage struct {
	upstream Stage
	ctx      context.Context
	output   chan Context

	// timeseries database stuff
	conn        *btrdb.BTrDB
	streamCache sync.Map

	sync.Mutex
}

type TimeseriesStageConfig struct {
	Upstream     Stage
	StageContext context.Context
	BTrDBAddress string
}

func NewTimeseriesQueryStage(cfg *TimeseriesStageConfig) (*TimeseriesQueryStage, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("Need to specify Upstream in Timeseries config")
	}
	stage := &TimeseriesQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan Context),
		ctx:      cfg.StageContext,
	}

	conn, err := btrdb.Connect(stage.ctx, cfg.BTrDBAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not connect to BTrDB at address %s", cfg.BTrDBAddress)
	}
	stage.conn = conn

	// TODO: configure concurrent connections
	num_workers := 5
	// consume function
	for i := 0; i < num_workers; i++ {
		go func() {
			input := stage.upstream.GetQueue()
			for {
				select {
				case ctx := <-input:
					if err := stage.processQuery(ctx); err != nil {
						log.Println(err)
					}
					// TODO: process query context
				case <-stage.ctx.Done():
					// case that breaks the stage and releases resources
					fmt.Println("Ending Timeseries Queue")
					return
				}
			}
		}()
	}

	return stage, nil
}

// TODO: write the helper function that retrieves data fro mthe timeseries database and adds it to the output channel.
// TODO: the context needs t ocarry any errors forward

func (stage *TimeseriesQueryStage) GetUpstream() Stage {
	stage.Lock()
	defer stage.Unlock()
	return stage.upstream
}

func (stage *TimeseriesQueryStage) SetUpstream(upstream Stage) {
	stage.Lock()
	defer stage.Unlock()
	if stage != nil {
		stage.upstream = upstream
	}
	fmt.Println("Updated stage to ", upstream)
}

func (stage *TimeseriesQueryStage) GetQueue() chan Context {
	return stage.output
}

func (stage *TimeseriesQueryStage) String() string {
	return "<|ts stage|>"
}

func (stage *TimeseriesQueryStage) getStream(streamuuid uuid.UUID) (stream *btrdb.Stream, err error) {
	_stream, found := stage.streamCache.Load(streamuuid.Array())
	if found {
		//var ok bool
		stream = _stream.(*btrdb.Stream)
		//_units, _ := b.unitCache.Load(streamuuid.Array())
		//units, ok = _units.(Unit)
		//if !ok {
		//	units = NO_UNITS
		//}
		return
	}
	ctx, cancel := context.WithTimeout(stage.ctx, MAX_TIMEOUT)
	defer cancel()
	stream = stage.conn.StreamFromUUID(streamuuid)
	if exists, existsErr := stream.Exists(ctx); existsErr != nil {
		if existsErr != nil {
			err = errors.Wrap(existsErr, "Could not fetch stream")
			return
		}
	} else if exists {

		//// get the units
		//annotations, _, annotationErr := stream.CachedAnnotations(context.Background())
		//if annotationErr != nil {
		//	err = errors.Wrap(annotationErr, "Could not fetch stream annotations")
		//	return
		//}
		//if _units, found := annotations["unit"]; found {
		//	units = ParseUnit(_units)
		//	b.unitCache.Store(streamuuid.Array(), units)
		//} else {
		//	b.unitCache.Store(streamuuid.Array(), NO_UNITS)
		//	units = NO_UNITS
		//}

		stage.streamCache.Store(streamuuid.Array(), stream)
		return
	}

	// else where we return a nil stream and the errStreamNotExist
	err = errStreamNotExist
	return
}

func (stage *TimeseriesQueryStage) processQuery(ctx Context) error {
	start_time, err := time.Parse(time.RFC3339, ctx.request.Time.Start)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse Start time (%s)", ctx.request.Time.Start)
		ctx.addError(err)
		return err
	}
	end_time, err := time.Parse(time.RFC3339, ctx.request.Time.End)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse End time (%s)", ctx.request.Time.End)
		ctx.addError(err)
		return err
	}
	//ctx.request.TimeParams.window
	qctx, cancel := context.WithTimeout(ctx.ctx, MAX_TIMEOUT)
	defer cancel()

	// loop over all streams, and then over all UUIDs
	for _, stream := range ctx.request.Streams {
		for _, uuStr := range stream.Uuids {
			uu := uuid.Parse(uuStr)
			stream, err := stage.getStream(uu)
			if err != nil {
				ctx.addError(err)
				return err
			}
			// if raw data...
			rawpoints, generations, errchan := stream.RawValues(qctx, start_time.UnixNano(), end_time.UnixNano(), 0)

			resp := &mortarpb.FetchResponse{}
			var pcount = 0
			for p := range rawpoints {
				pcount += 1
				resp.Times = append(resp.Times, p.Time)
				resp.Values = append(resp.Values, p.Value)
				if pcount == TS_BATCH_SIZE {
					ctx.response = resp
					stage.output <- ctx
					resp = &mortarpb.FetchResponse{}
					pcount = 0
				}
			}
			if len(resp.Times) > 0 {
				ctx.response = resp
				stage.output <- ctx
			}

			<-generations
			if err := <-errchan; err != nil {
				ctx.addError(err)
				return err
			}
		}
	}

	return nil
}
