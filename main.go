package main

import (
	"context"
	"errors"
	"github.com/heptiolabs/healthcheck"
	"github.com/pkg/profile"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	logrus "github.com/sirupsen/logrus"
	"net/http"
	"runtime"
	"time"
)

var (
	qualifyQueriesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "qualify_queries_processed",
		Help: "total number of processed Qualify queries",
	})
	fetchQueriesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fetch_queries_processed",
		Help: "total number of processed Fetch queries",
	})
	messagesSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "msgs_sent",
		Help: "total number of sent messages",
	})
	qualifyProcessingTimes = promauto.NewSummary(prometheus.SummaryOpts{
		Name: "qualify_processing_time_milliseconds",
		Help: "amount of time it takes to process a qualify query",
	})
	fetchProcessingTimes = promauto.NewSummary(prometheus.SummaryOpts{
		Name: "fetch_processing_time_milliseconds",
		Help: "amount of time it takes to process a fetch query",
	})
	authRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_requests_received",
		Help: "number of authentication requests we get",
	})
	authRequestsSuccessful = promauto.NewCounter(prometheus.CounterOpts{
		Name: "successful_auth_requests_received",
		Help: "number of authentication requests we get that are successful",
	})
	activeQueries = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_queries",
		Help: "number of actively processed queries",
	})
)

func main() {
	doCPUprofile := false
	doBlockprofile := false
	doMemprofile := false

	// monitor the prometheus metrics and print them out periodically
	go func() {
		var (
			qualifyQueriesProcessed_old float64 = 0
			fetchQueriesProcessed_old   float64 = 0
			messagesSent_old            float64 = 0
			authRequests_old            float64 = 0
			authRequestsSuccessful_old  float64 = 0
		)

		var f = make(logrus.Fields)
		for range time.Tick(30 * time.Second) {
			var m dto.Metric

			if err := qualifyQueriesProcessed.Write(&m); err != nil {
				panic(err)
			} else {
				f["#qualify"] = *m.Counter.Value - qualifyQueriesProcessed_old
				qualifyQueriesProcessed_old = *m.Counter.Value
			}

			if err := fetchQueriesProcessed.Write(&m); err != nil {
				panic(err)
			} else {
				f["#fetch"] = *m.Counter.Value - fetchQueriesProcessed_old
				fetchQueriesProcessed_old = *m.Counter.Value
			}

			if err := messagesSent.Write(&m); err != nil {
				panic(err)
			} else {
				f["#msg"] = *m.Counter.Value - messagesSent_old
				messagesSent_old = *m.Counter.Value
			}

			if err := authRequests.Write(&m); err != nil {
				panic(err)
			} else {
				f["#auth raw"] = *m.Counter.Value - authRequests_old
				authRequests_old = *m.Counter.Value
			}

			if err := authRequestsSuccessful.Write(&m); err != nil {
				panic(err)
			} else {
				f["#auth good"] = *m.Counter.Value - authRequestsSuccessful_old
				authRequestsSuccessful_old = *m.Counter.Value
			}

			if err := activeQueries.Write(&m); err != nil {
				panic(err)
			} else {
				f["#active"] = *m.Gauge.Value
			}

			//var rtm runtime.MemStats
			//runtime.ReadMemStats(&rtm)
			f["#goroutines"] = runtime.NumGoroutine()
			//f["total_alloc"] = rtm.TotalAlloc / 1e6
			//f["alloc"] = rtm.Alloc / 1e6

			log.WithFields(f).Info(">")
		}
	}()

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
	cfg, err := ReadConfig("mortarconfig.yml")
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

	frontend_stage_cfg := &ApiFrontendBasicStageConfig{
		StageContext: maincontext,
		ListenAddr:   cfg.ListenAddr,
		AuthConfig:   cfg.Cognito,
		TLSCrtFile:   cfg.TLSCrtFile,
		TLSKeyFile:   cfg.TLSKeyFile,
	}
	frontend_stage, err := NewApiFrontendBasicStage(frontend_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	md_stage_cfg := &BrickQueryStageConfig{
		Upstream:          frontend_stage,
		StageContext:      maincontext,
		HodConfigLocation: cfg.HodConfig,
	}

	md_stage, err := NewBrickQueryStage(md_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}
	brickready = true

	ts_stage_cfg := &TimeseriesStageConfig{
		Upstream:     md_stage,
		StageContext: maincontext,
		BTrDBAddress: cfg.BTrDBAddr,
	}
	ts_stage, err := NewTimeseriesQueryStage(ts_stage_cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = ts_stage

	var end Stage = ts_stage
	for end != nil {
		log.Println(end)
		end = end.GetUpstream()
	}

	if doCPUprofile {
		defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	}
	if doBlockprofile {
		defer profile.Start(profile.BlockProfile, profile.ProfilePath(".")).Stop()
	}
	if doMemprofile {
		defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	}

	//TODO: i think the problem is inside here
	go func() {
		log.Println("get output")
		processedContexts := ts_stage.GetQueue()
		for queryCtx := range processedContexts {
			if queryCtx.response == nil {
				if queryCtx.done != nil {
					close(queryCtx.done)
				}
				if queryCtx.qualify_done != nil {
					close(queryCtx.qualify_done)
				}
			} else {
				select {
				case queryCtx.done <- queryCtx.response:
				case <-queryCtx.ctx.Done():
				}
			}
		}
	}()

	select {}
	cancel()
}
