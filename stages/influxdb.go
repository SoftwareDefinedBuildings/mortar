package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mortarpb "github.com/SoftwareDefinedBuildings/mortar/proto"
	//influx "github.com/influxdata/influxdb/client/v2"
	influx "github.com/hamilton-lima/influxdb1-client/client"
	//"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

type InfluxDBTimeseriesQueryStage struct {
	upstream Stage
	ctx      context.Context
	output   chan *Request

	conn influx.Client
	// timeseries database stuff
	//conn        *btrdb.BTrDB
	//streamCache sync.Map

	sync.Mutex
}

type InfluxDBTimeseriesStageConfig struct {
	Upstream     Stage
	StageContext context.Context
	Username     string
	Password     string
	Address      string
}

func NewInfluxDBTimeseriesQueryStage(cfg *InfluxDBTimeseriesStageConfig) (*InfluxDBTimeseriesQueryStage, error) {

	conn, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr:     cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not connect influx")
	}
	log.Info("Connected to InfluxDB!")

	stage := &InfluxDBTimeseriesQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan *Request),
		ctx:      cfg.StageContext,
		conn:     conn,
	}

	// TODO: configure concurrent connections
	num_workers := 20
	// consume function
	for i := 0; i < num_workers; i++ {
		go func() {
			input := stage.upstream.GetQueue()
			for {
				select {
				case req := <-input:
					if len(req.fetch_request.Sites) > 0 && len(req.fetch_request.DataFrames) > 0 {
						if err := stage.processQuery(req); err != nil {
							log.Println(err)
						}
					}
					//stage.output <- req
				case <-stage.ctx.Done():
					// case that breaks the stage and releases resources
					fmt.Println("Ending Timeseries Queue")
					return
				}
			}
		}()
	}

	return stage, nil

}

func (stage *InfluxDBTimeseriesQueryStage) GetUpstream() Stage {
	stage.Lock()
	defer stage.Unlock()
	return stage.upstream
}

func (stage *InfluxDBTimeseriesQueryStage) SetUpstream(upstream Stage) {
	stage.Lock()
	defer stage.Unlock()
	if stage != nil {
		stage.upstream = upstream
	}
	fmt.Println("Updated stage to ", upstream)
}

func (stage *InfluxDBTimeseriesQueryStage) GetQueue() chan *Request {
	return stage.output
}

func (stage *InfluxDBTimeseriesQueryStage) String() string {
	return "<|influx ts stage|>"
}

func (stage *InfluxDBTimeseriesQueryStage) processQuery(req *Request) error {
	// parse timestamps for the query
	start_time, err := time.Parse(time.RFC3339, req.fetch_request.Time.Start)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse Start time (%s)", req.fetch_request.Time.Start)
		req.addError(err)
		return err
	}
	end_time, err := time.Parse(time.RFC3339, req.fetch_request.Time.End)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse End time (%s)", req.fetch_request.Time.End)
		req.addError(err)
		return err
	}

	// TODO: there is an awkward design artifact we have inherited. Essentially,
	// we put each collection from a plugin in its own "measurement". But, we want to query
	// by UUID, and we don't know the measurement. Either need to build some index of UUID
	// to measurement, or just insert into a single measurement that we know from the beginning
	for _, dataFrame := range req.fetch_request.DataFrames {
		for _, uuStr := range dataFrame.Uuids {

			// default for RAW
			selector := "value"
			var groupby string
			if dataFrame.Aggregation != mortarpb.AggFunc_AGG_FUNC_RAW {
				window, err := ParseDuration(dataFrame.Window)
				if err != nil {
					req.addError(err)
					return err
				}
				groupby = fmt.Sprintf("GROUP BY time(%s)", window)
			}

			// TODO: this is automatically interpolating?

			switch dataFrame.Aggregation {
			case mortarpb.AggFunc_AGG_FUNC_MEAN:
				selector = `mean("value")`
			case mortarpb.AggFunc_AGG_FUNC_MIN:
				selector = `min("value")`
			case mortarpb.AggFunc_AGG_FUNC_MAX:
				selector = `max("value")`
			case mortarpb.AggFunc_AGG_FUNC_SUM:
				selector = `sum("value")`
			case mortarpb.AggFunc_AGG_FUNC_COUNT:
				selector = `count("value")`
			}

			q_str := fmt.Sprintf(`SELECT time, %s
                             FROM "timeseries"
                             WHERE uuid='%s' 
                               AND time >= %d 
                               AND time < %d
                             %s
                             ;`, selector, uuStr, start_time.UnixNano(), end_time.UnixNano(), groupby)
			q := influx.Query{
				Command:   q_str,
				Database:  "xbos",
				Precision: "ns",
			}
			resp, err := stage.conn.Query(q)
			if err != nil {
				log.Error(err)
				continue
			}
			if len(resp.Results) < 1 {
				log.Error("nil results")
				continue
			}
			if len(resp.Results[0].Series) < 1 {
				log.Error("nil series")
				continue
			}

			tsresp := &mortarpb.FetchResponse{}
			var pcount = 0

			for _, ser := range resp.Results[0].Series {
				for _, row := range ser.Values {
					if row[1] == nil {
						continue
					}

					time, err := row[0].(json.Number).Int64()
					if err != nil {
						log.Error(err)
						continue
					}
					value, err := row[1].(json.Number).Float64()
					if err != nil {
						log.Error(err)
						continue
					}
					pcount += 1
					tsresp.Times = append(tsresp.Times, time)
					tsresp.Values = append(tsresp.Values, value)

					if pcount == TS_BATCH_SIZE {
						tsresp.DataFrame = dataFrame.Name
						tsresp.Identifier = uuStr
						select {
						case req.fetch_responses <- tsresp:
						case <-req.Done():
							continue
						}
						//stage.output <- ctx
						tsresp = &mortarpb.FetchResponse{}
						pcount = 0
					}
				}
			}
			// any left over
			if len(tsresp.Times) > 0 {
				tsresp.DataFrame = dataFrame.Name
				tsresp.Identifier = uuStr
				select {
				case req.fetch_responses <- tsresp:
				case <-req.Done():
					continue
				}
				//stage.output <- ctx
			}

		}
	}
	req.fetch_responses <- nil

	return nil
}
