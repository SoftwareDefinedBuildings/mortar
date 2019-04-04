package stages

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
	logrus "github.com/sirupsen/logrus"
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

func init() {
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

			log.WithFields(f).Info(">")
		}
	}()
}
