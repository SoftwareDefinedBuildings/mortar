package main

import (
	"context"
	"fmt"
	"git.sr.ht/~gabe/hod/hod"
	logpb "git.sr.ht/~gabe/hod/proto"
	"github.com/pkg/errors"
	logrus "github.com/sirupsen/logrus"
	"os"
	"strings"
	"sync"
	"time"
)

var log = logrus.New()

func init() {
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, ForceColors: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.DebugLevel)
}

type BrickQueryStage struct {
	upstream Stage
	ctx      context.Context
	output   chan Context

	db            *hod.Log
	highwatermark int64

	sync.Mutex
}

type BrickQueryStageConfig struct {
	Upstream     Stage
	StageContext context.Context
}

func NewBrickQueryStage(cfg *BrickQueryStageConfig) (*BrickQueryStage, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("Need to specify Upstream in Metadata config")
	}
	stage := &BrickQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan Context),
		ctx:      cfg.StageContext,
	}

	log.Info("Start loading Brick config")
	start := time.Now()
	hodcfg, err := hod.ReadConfig("hodconfig.yml")
	if err != nil {
		return nil, err
	}
	stage.db, err = hod.NewLog(hodcfg)
	if err != nil {
		return nil, err
	}

	stage.highwatermark = time.Now().UnixNano()
	q := "SELECT ?vav FROM ciee WHERE { ?vav rdf:type brick:Zone_Temperature_Sensor };"
	query, err := stage.db.ParseQuery(q, stage.highwatermark)
	if err != nil {
		return nil, err
	}
	// TODO: rewrite query to get points and units
	if _, err = stage.db.Select(stage.ctx, query); err != nil {
		return nil, err
	}
	log.Infof("Done loading Brick. Took %s", time.Since(start))

	num_workers := 10
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
					// TODO: process query context
				case <-stage.ctx.Done():
					// case that breaks the stage and releases resources
					fmt.Println("Ending Brick Queue")
					return
				}
			}
		}()
	}

	return stage, nil
}

// get the stage we pull from
func (stage *BrickQueryStage) GetUpstream() Stage {
	stage.Lock()
	defer stage.Unlock()
	return stage.upstream
}

// set the stage we pull from
func (stage *BrickQueryStage) SetUpstream(upstream Stage) {
	stage.Lock()
	defer stage.Unlock()
	if stage != nil {
		stage.upstream = upstream
	}
	fmt.Println("Updated stage to ", upstream)
}

// blocks on internal channel until next "Context" is ready
func (stage *BrickQueryStage) GetQueue() chan Context {
	return stage.output
}
func (stage *BrickQueryStage) String() string {
	return "<| brick stage |>"
}

func (stage *BrickQueryStage) processQuery(ctx Context) error {
	qctx, cancel := context.WithTimeout(ctx.ctx, MAX_TIMEOUT)
	defer cancel()

	for idx, reqstream := range ctx.request.Streams {
		query, err := stage.db.ParseQuery(reqstream.Definition, stage.highwatermark)
		if err != nil {
			ctx.addError(err)
			return err
		}
		// this rewrites the incoming query so that it extracts the UUIDs (bf:uuid property) for each of the
		// variables in the SELECT clause of the query. This removes the need for the user to know that the bf:uuid
		// property is how to relate the points to the timeseries database. However, it also introduces the complexity
		// of dealing with whether or not the variables *do* have associated timeseries or not.
		startIdx := rewriteQuery(query)
		res, err := stage.db.Select(qctx, query)
		if err != nil {
			ctx.addError(err)
			return err
		}

		// extract UUIDs from query results and push into context
		stream := ctx.request.Streams[idx]
		for _, row := range res.Rows {
			for uuidx := startIdx; uuidx < len(query.Vars); uuidx++ {
				stream.Uuids = append(stream.Uuids, row.Values[uuidx].Value)
			}
		}

		stage.output <- ctx
	}
	// brick query
	return nil
}

func rewriteQuery(query *logpb.SelectQuery) int {
	var newtriples []*logpb.Triple
	var newselect []string
	uuidPred := logpb.URI{Namespace: "https://brickschema.org/schema/1.0.3/BrickFrame", Value: "uuid"}

	for _, varname := range query.Vars {
		basevarname := strings.TrimPrefix(varname, "?")
		basevarname_uuid := "?" + basevarname + "_uuid"
		newtriples = append(newtriples, &logpb.Triple{Subject: &logpb.URI{Value: varname}, Predicate: []*logpb.URI{&uuidPred}, Object: &logpb.URI{Value: basevarname_uuid}})
		newselect = append(newselect, basevarname_uuid)
	}

	oldidx := len(query.Vars)
	query.Where = append(query.Where, newtriples...)
	query.Vars = append(query.Vars, newselect...)
	return oldidx
}
