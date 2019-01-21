package main

import (
	"context"
	"fmt"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"gopkg.in/btrdb.v4"
	"math"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type TimeseriesQueryStage struct {
	upstream Stage
	ctx      context.Context
	output   chan Context

	// timeseries database stuff
	conn        *btrdb.BTrDB
	streamCache sync.Map

	sync.Mutex
}

type TimeseriesStageConfig struct {
	Upstream     Stage
	StageContext context.Context
	BTrDBAddress string
}

func NewTimeseriesQueryStage(cfg *TimeseriesStageConfig) (*TimeseriesQueryStage, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("Need to specify Upstream in Timeseries config")
	}
	stage := &TimeseriesQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan Context),
		ctx:      cfg.StageContext,
	}

	conn, err := btrdb.Connect(stage.ctx, cfg.BTrDBAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not connect to BTrDB at address %s", cfg.BTrDBAddress)
	}
	stage.conn = conn

	// TODO: configure concurrent connections
	num_workers := 20
	// consume function
	for i := 0; i < num_workers; i++ {
		go func() {
			input := stage.upstream.GetQueue()
			for {
				select {
				case ctx := <-input:
					if err := stage.processQuery(ctx); err != nil {
						log.Println(err)
					}
					ctx.response = nil
					stage.output <- ctx
					//ctx.done <- nil
					// TODO: process query context
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

// TODO: write the helper function that retrieves data fro mthe timeseries database and adds it to the output channel.
// TODO: the context needs t ocarry any errors forward

func (stage *TimeseriesQueryStage) GetUpstream() Stage {
	stage.Lock()
	defer stage.Unlock()
	return stage.upstream
}

func (stage *TimeseriesQueryStage) SetUpstream(upstream Stage) {
	stage.Lock()
	defer stage.Unlock()
	if stage != nil {
		stage.upstream = upstream
	}
	fmt.Println("Updated stage to ", upstream)
}

func (stage *TimeseriesQueryStage) GetQueue() chan Context {
	return stage.output
}

func (stage *TimeseriesQueryStage) String() string {
	return "<|ts stage|>"
}

func (stage *TimeseriesQueryStage) getStream(ctx context.Context, streamuuid uuid.UUID) (stream *btrdb.Stream, err error) {
	_stream, found := stage.streamCache.Load(streamuuid.Array())
	if found {
		//var ok bool
		stream = _stream.(*btrdb.Stream)
		//_units, _ := b.unitCache.Load(streamuuid.Array())
		//units, ok = _units.(Unit)
		//if !ok {
		//	units = NO_UNITS
		//}
		return
	}
	stream = stage.conn.StreamFromUUID(streamuuid)
	if exists, existsErr := stream.Exists(ctx); existsErr != nil {
		if existsErr != nil {
			e := btrdb.ToCodedError(existsErr)
			if e.Code != 501 {
				err = errors.Wrap(existsErr, "Could not fetch stream")
				log.Fatal("c")
				//defer cancel()
				return
			}
		}
	} else if exists {

		//// get the units
		//annotations, _, annotationErr := stream.CachedAnnotations(context.Background())
		//if annotationErr != nil {
		//	err = errors.Wrap(annotationErr, "Could not fetch stream annotations")
		//	return
		//}
		//if _units, found := annotations["unit"]; found {
		//	units = ParseUnit(_units)
		//	b.unitCache.Store(streamuuid.Array(), units)
		//} else {
		//	b.unitCache.Store(streamuuid.Array(), NO_UNITS)
		//	units = NO_UNITS
		//}

		stage.streamCache.Store(streamuuid.Array(), stream)
		return
	}

	// else where we return a nil stream and the errStreamNotExist
	if stream == nil {
		err = errStreamNotExist
		//defer cancel()
	}
	return
}

func (stage *TimeseriesQueryStage) processQuery(ctx Context) error {
	//	defer ctx.finish()
	// parse timestamps for the query
	start_time, err := time.Parse(time.RFC3339, ctx.request.Time.Start)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse Start time (%s)", ctx.request.Time.Start)
		ctx.addError(err)
		return err
	}
	end_time, err := time.Parse(time.RFC3339, ctx.request.Time.End)
	if err != nil {
		err = errors.Wrapf(err, "Could not parse End time (%s)", ctx.request.Time.End)
		ctx.addError(err)
		return err
	}

	//ctx.request.TimeParams.window
	//qctx, cancel := context.WithTimeout(ctx.ctx, MAX_TIMEOUT)

	//TODO: when we try to download multiple streams, a "nil" gets sent too
	// early and causes the "done" channel to be closed

	// loop over all streams, and then over all UUIDs
	for _, reqstream := range ctx.request.Streams {
		for _, uuStr := range reqstream.Uuids {
			uu := uuid.Parse(uuStr)
			stream, err := stage.getStream(ctx.ctx, uu)
			if err != nil {
				ctx.addError(err)
				return err
			}

			// handle RAW streams
			if reqstream.Aggregation == mortarpb.AggFunc_AGG_FUNC_RAW {
				// if raw data...
				rawpoints, generations, errchan := stream.RawValues(ctx.ctx, start_time.UnixNano(), end_time.UnixNano(), 0)
				resp := &mortarpb.FetchResponse{}
				var pcount = 0
				for p := range rawpoints {
					pcount += 1
					resp.Times = append(resp.Times, p.Time)
					resp.Values = append(resp.Values, p.Value)
					if pcount == TS_BATCH_SIZE {
						resp.Variable = reqstream.Name
						resp.Identifier = uuStr
						ctx.response = resp
						stage.output <- ctx
						resp = &mortarpb.FetchResponse{}
						pcount = 0
					}
				}
				if len(resp.Times) > 0 {
					resp.Variable = reqstream.Name
					resp.Identifier = uuStr
					ctx.response = resp
					stage.output <- ctx
				}

				<-generations
				if err := <-errchan; err != nil {
					ctx.addError(err)
					return err
				}
			} else {
				windowSize, err := ParseDuration(ctx.request.Time.Window)
				if err != nil {
					ctx.addError(err)
					return err
				}
				windowDepth := math.Log2(float64(windowSize))
				suggestedAccuracy := uint8(math.Max(windowDepth-5, 30))

				statpoints, generations, errchan := stream.Windows(ctx.ctx, start_time.UnixNano(), end_time.UnixNano(), uint64(windowSize.Nanoseconds()), suggestedAccuracy, 0)

				resp := &mortarpb.FetchResponse{}
				var pcount = 0
				for p := range statpoints {
					pcount += 1
					resp.Times = append(resp.Times, p.Time)

					resp.Values = append(resp.Values, valueFromAggFunc(p, reqstream.Aggregation))

					if pcount == TS_BATCH_SIZE {
						resp.Variable = reqstream.Name
						resp.Identifier = uuStr
						ctx.response = resp
						stage.output <- ctx
						resp = &mortarpb.FetchResponse{}
						pcount = 0
					}
				}
				if len(resp.Times) > 0 {
					resp.Variable = reqstream.Name
					resp.Identifier = uuStr
					ctx.response = resp
					stage.output <- ctx
				}

				<-generations
				if err := <-errchan; err != nil {
					ctx.addError(err)
					return err
				}

			}

		}
	}

	return nil
}

var dur_re = regexp.MustCompile(`(\d+)(\w+)`)

func ParseDuration(expr string) (time.Duration, error) {
	var d time.Duration
	results := dur_re.FindAllStringSubmatch(expr, -1)
	if len(results) == 0 {
		return d, errors.New("Invalid. Must be Number followed by h,m,s,us,ms,ns,d")
	}
	num := results[0][1]
	units := results[0][2]
	i, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return d, err
	}
	d = time.Duration(i)
	switch units {
	case "h", "hr", "hour", "hours":
		d *= time.Hour
	case "m", "min", "minute", "minutes":
		d *= time.Minute
	case "s", "sec", "second", "seconds":
		d *= time.Second
	case "us", "usec", "microsecond", "microseconds":
		d *= time.Microsecond
	case "ms", "msec", "millisecond", "milliseconds":
		d *= time.Millisecond
	case "ns", "nsec", "nanosecond", "nanoseconds":
		d *= time.Nanosecond
	case "d", "day", "days":
		d *= 24 * time.Hour
	default:
		err = fmt.Errorf("Invalid unit %v. Must be h,m,s,us,ms,ns,d", units)
	}
	return d, err
}

func valueFromAggFunc(point btrdb.StatPoint, aggfunc mortarpb.AggFunc) float64 {
	switch aggfunc {
	case mortarpb.AggFunc_AGG_FUNC_MEAN:
		return point.Mean
	case mortarpb.AggFunc_AGG_FUNC_MIN:
		return point.Min
	case mortarpb.AggFunc_AGG_FUNC_MAX:
		return point.Max
	case mortarpb.AggFunc_AGG_FUNC_COUNT:
		return float64(point.Count)
	case mortarpb.AggFunc_AGG_FUNC_SUM:
		return float64(point.Count) * point.Mean
	}
	return point.Mean
}
