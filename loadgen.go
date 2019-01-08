package main

type SimpleLoadGenStage struct {
	output chan Context
}

func NewSimpleLoadGenStage() *SimpleLoadGenStage {
	stage := &SimpleLoadGenStage{
		output: make(chan Context),
	}

	go func() {
		for {
			ctx := Context{}
			stage.output <- ctx
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
