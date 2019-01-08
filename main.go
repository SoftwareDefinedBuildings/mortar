package main

import (
	"context"
	"log"
	"time"
)

func main() {

	maincontext, cancel := context.WithCancel(context.Background())

	loadgen_stage := NewSimpleLoadGenStage()

	ts_stage_cfg := &TimeseriesStageConfig{
		Upstream:     loadgen_stage,
		StageContext: maincontext,
	}
	ts_stage, err := NewTimeseriesQueryStage(ts_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	var end Stage = ts_stage
	for end != nil {
		log.Println(end)
		end = end.GetUpstream()
	}

	time.Sleep(4 * time.Second)
	cancel()
}
