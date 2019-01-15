package main

import (
	"sync/atomic"
	"time"
)

type SimpleLoadGenStage struct {
	output chan Context
	count  int64
}

func NewSimpleLoadGenStage(contexts ...func() Context) *SimpleLoadGenStage {
	stage := &SimpleLoadGenStage{
		output: make(chan Context),
	}

	if len(contexts) == 0 {
		contexts = append(contexts, func() Context { return Context{} })
	}

	go func() {
		for i := 0; i < len(contexts); i++ {
			stage.output <- contexts[i]()
			atomic.AddInt64(&stage.count, 1)
			if i == len(contexts)-1 {
				i = -1
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			value := atomic.SwapInt64(&stage.count, 0)
			log.Debugf("Delivered %d/sec", value/10)
		}

	}()
	return stage
}

func (stage *SimpleLoadGenStage) GetUpstream() Stage {
	return nil
}

// set the stage we pull from
func (stage *SimpleLoadGenStage) SetUpstream(upstream Stage) {
}

// blocks on internal channel until next "Context" is ready
func (stage *SimpleLoadGenStage) GetQueue() chan Context {
	return stage.output
}

func (stage *SimpleLoadGenStage) String() string {
	return "<| loadgen |>"
}
