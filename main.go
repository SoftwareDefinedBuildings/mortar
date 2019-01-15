package main

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/profile"
	"time"
)

func main() {
	defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()

	maincontext, cancel := context.WithCancel(context.Background())

	// do query with uuid: 5bd3a840-6ee6-5922-aeaf-2d7ec0bb4cff(
	makectx1 := func() Context {
		req := mortarpb.FetchRequest{
			Sites: []string{"test_me_site_name"},
			Streams: []*mortarpb.Stream{
				{
					Name:        "test1",
					Definition:  "SELECT ?vav FROM ciee WHERE { ?vav rdf:type brick:Zone_Temperature_Sensor};",
					Uuids:       []string{"2bde3736-1d93-59f4-8cdd-080d154c31be"},
					Aggregation: mortarpb.AggFunc_AGG_FUNC_RAW,
					Units:       "",
				},
			},
			Time: &mortarpb.TimeParams{
				Start: "1970-01-01T00:00:00Z",
				End:   "1970-01-10T00:00:00Z",
			},
		}
		ctx1 := Context{
			ctx:     context.Background(),
			request: req,
		}
		return ctx1
	}

	loadgen_stage := NewSimpleLoadGenStage(makectx1)

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
	c := ts_stage.GetQueue()
	for out := range c {
		//fmt.Printf("> %d\n", len(out.response.Times))
		_ = out
	}

	time.Sleep(30 * time.Second)
	cancel()
}
