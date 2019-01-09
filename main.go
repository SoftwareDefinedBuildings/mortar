package main

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"log"
	"time"
)

func main() {

	maincontext, cancel := context.WithCancel(context.Background())

	// do query with uuid: 5bd3a840-6ee6-5922-aeaf-2d7ec0bb4cff(
	req := mortarpb.FetchRequest{
		Sites: []string{"test_me_site_name"},
		Streams: []*mortarpb.Stream{
			{
				Name:        "test1",
				Definition:  "SELECT ?vav FROM soda WHERE { ?vav rdf:type brick:VAV };",
				Uuids:       []string{"5bd3a840-6ee6-5922-aeaf-2d7ec0bb4cff"},
				Aggregation: mortarpb.AggFunc_AGG_FUNC_RAW,
				Units:       "",
			},
		},
		Time: &mortarpb.TimeParams{
			Start: "2019-01-07T00:00:00Z",
			End:   "2019-01-10T00:00:00Z",
		},
	}
	ctx1 := Context{
		ctx:     context.Background(),
		request: req,
	}

	loadgen_stage := NewSimpleLoadGenStage(ctx1)

	md_stage_cfg := &BrickQueryStageConfig{
		Upstream:     loadgen_stage,
		StageContext: maincontext,
	}

	md_stage, err := NewBrickQueryStage(md_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	ts_stage_cfg := &TimeseriesStageConfig{
		Upstream:     md_stage,
		StageContext: maincontext,
		BTrDBAddress: "127.0.0.1:4410",
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

	log.Println("get output")
	for out := range ts_stage.GetQueue() {
		//log.Println(">", len(out.response.Times))
		_ = out
	}

	time.Sleep(30 * time.Second)
	cancel()
}
