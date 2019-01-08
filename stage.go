package main

import (
	"context"
	"fmt"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/errors"
	"sync"
)

type Context struct {
	ctx      context.Context
	request  mortarpb.FetchRequest
	response *mortarpb.FetchResponse
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
	output   chan Context

	sync.Mutex
}

type TimeseriesStageConfig struct {
	Upstream     Stage
	StageContext context.Context
}

func NewTimeseriesQueryStage(cfg *TimeseriesStageConfig) (*TimeseriesQueryStage, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("Need to specify Upstream in Timeseries config")
	}
	stage := &TimeseriesQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan Context),
	}

	// consume function
	go func() {
		for {
			select {
			case ctx := <-stage.upstream.GetQueue():
				fmt.Println("got", ctx)
				// TODO: process query context
			case <-cfg.StageContext.Done():
				// case that breaks the stage and releases resources
				fmt.Println("Ending Timeseries Queue")
				return
			}
		}
	}()

	return stage, nil
}

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
