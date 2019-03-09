package main

import (
	"context"
	"errors"
	"git.sr.ht/~gabe/mortar/stages"
	"github.com/heptiolabs/healthcheck"
	"github.com/pkg/profile"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logrus "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

var log = logrus.New()

func init() {
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, ForceColors: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.DebugLevel)
}

func main() {
	doCPUprofile := false
	if doCPUprofile {
		defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	}
	doBlockprofile := false
	if doBlockprofile {
		defer profile.Start(profile.BlockProfile, profile.ProfilePath(".")).Stop()
	}

	maincontext, cancel := context.WithCancel(context.Background())

	// do query with uuid: 5bd3a840-6ee6-5922-aeaf-2d7ec0bb4cff(
	//	makectx1 := func() Context {
	//		req := mortarpb.FetchRequest{
	//			Sites: []string{"test_me_site_name"},
	//			Streams: []*mortarpb.Stream{
	//				{
	//					Name:        "test1",
	//					Definition:  "SELECT ?vav FROM ciee WHERE { ?vav rdf:type brick:Zone_Temperature_Sensor};",
	//					DataVars:    []string{"?vav"},
	//					Uuids:       []string{"2bde3736-1d93-59f4-8cdd-080d154c31be"},
	//					Aggregation: mortarpb.AggFunc_AGG_FUNC_RAW,
	//					Units:       "",
	//				},
	//			},
	//			Time: &mortarpb.TimeParams{
	//				Start: "1970-01-01T00:00:00Z",
	//				End:   "1970-01-10T00:00:00Z",
	//			},
	//		}
	//		ctx1 := Context{
	//			ctx:     context.Background(),
	//			request: req,
	//		}
	//		return ctx1
	//	}

	//loadgen_stage := NewSimpleLoadGenStage(makectx1)
	//testcognito()
	cfg, err := stages.ReadConfig("mortarconfig.yml")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("%+v", cfg)

	brickready := false
	health := healthcheck.NewHandler()
	health.AddReadinessCheck("brick", func() error {
		if !brickready {
			return errors.New("Brick not ready")
		}
		return nil
	})
	go http.ListenAndServe("0.0.0.0:8086", health)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Infof("Prometheus endpoint at %s", cfg.PrometheusAddr)
		if err := http.ListenAndServe(cfg.PrometheusAddr, nil); err != nil {
			log.Fatal(err)
		}
	}()

	frontend_stage_cfg := &stages.ApiFrontendBasicStageConfig{
		StageContext: maincontext,
		ListenAddr:   cfg.ListenAddr,
		AuthConfig:   cfg.Cognito,
		TLSCrtFile:   cfg.TLSCrtFile,
		TLSKeyFile:   cfg.TLSKeyFile,
	}
	frontend_stage, err := stages.NewApiFrontendBasicStage(frontend_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	md_stage_cfg := &stages.BrickQueryStageConfig{
		Upstream:          frontend_stage,
		StageContext:      maincontext,
		HodConfigLocation: cfg.HodConfig,
	}

	md_stage, err := stages.NewBrickQueryStage(md_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}
	brickready = true

	ts_stage_cfg := &stages.TimeseriesStageConfig{
		Upstream:     md_stage,
		StageContext: maincontext,
		BTrDBAddress: cfg.BTrDBAddr,
	}
	ts_stage, err := stages.NewTimeseriesQueryStage(ts_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = ts_stage

	var end stages.Stage = ts_stage
	for end != nil {
		log.Println(end)
		end = end.GetUpstream()
	}

	stages.Showtime(ts_stage.GetQueue())

	//	go func() {
	//		log.Println("get output")
	//		c := ts_stage.GetQueue()
	//		for out := range c {
	//			if out.response == nil {
	//				if out.done != nil {
	//					close(out.done)
	//				}
	//				if out.qualify_done != nil {
	//					close(out.qualify_done)
	//				}
	//			} else {
	//				out.done <- out.response
	//			}
	//		}
	//	}()

	select {}
	cancel()
}
