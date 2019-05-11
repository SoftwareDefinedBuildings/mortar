package stages

import (
	"sync"
	"sync/atomic"
	"time"
)

type SimpleLoadGenStage struct {
	output chan *Request
	count  int64
}

func NewSimpleLoadGenStage(contexts ...func() *Request) *SimpleLoadGenStage {
	stage := &SimpleLoadGenStage{
		output: make(chan *Request),
	}

	if len(contexts) == 0 {
		contexts = append(contexts, func() *Request { return &Request{} })
	}

	var notifier sync.Once

	go func() {
		for i := 0; i < len(contexts); i++ {
			stage.output <- contexts[i]()
			notifier.Do(func() {
				go func() {
					ticker := time.NewTicker(10 * time.Second)
					for range ticker.C {
						value := atomic.SwapInt64(&stage.count, 0)
						log.Debugf("Delivered %d/sec", value/10)
					}

				}()
			})
			atomic.AddInt64(&stage.count, 1)
			if i == len(contexts)-1 {
				i = -1
			}
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

// blocks on internal channel until next "*Request" is ready
func (stage *SimpleLoadGenStage) GetQueue() chan *Request {
	return stage.output
}

func (stage *SimpleLoadGenStage) String() string {
	return "<| loadgen |>"
}
