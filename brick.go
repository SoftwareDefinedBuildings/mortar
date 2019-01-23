package main

import (
	"context"
	"fmt"
	"git.sr.ht/~gabe/hod/hod"
	logpb "git.sr.ht/~gabe/hod/proto"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
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
	q := "SELECT ?c FROM * WHERE { ?c rdf:type brick:Class };"
	query, err := stage.db.ParseQuery(q, stage.highwatermark)
	if err != nil {
		return nil, err
	}
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
					if len(ctx.request.Sites) > 0 {
						if err := stage.processQuery(ctx); err != nil {
							log.Println(err)
							ctx.response = nil
							stage.output <- ctx
						}
					} else if len(ctx.qualify_request.Required) > 0 {
						if err := stage.processQualify(ctx); err != nil {
							log.Println(err)
							ctx.response = nil
							stage.output <- ctx
						}
					}
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

func (stage *BrickQueryStage) processQualify(ctx Context) error {
	brickresp := &mortarpb.QualifyResponse{}

	sites := make(map[string]struct{})

	// TODO: need to implement the query inside hod
	version_query := &logpb.VersionQuery{
		Graphs:    []string{"*"},
		Filter:    logpb.TimeFilter_At,
		Timestamp: time.Now().UnixNano(),
	}
	version_response, err := stage.db.Versions(ctx.ctx, version_query)
	if err != nil {
		ctx.addError(err)
		return err
	}
	if version_response.Error != "" {
		ctx.addError(errors.New(version_response.Error))
		return err
	}

	for _, row := range version_response.Rows {
		sites[row.Values[0].Value] = struct{}{}
	}

	for _, querystring := range ctx.qualify_request.Required {
		query, err := stage.db.ParseQuery(querystring, stage.highwatermark)
		if err != nil {
			ctx.addError(err)
			return err
		}

		for site := range sites {
			query.Graphs = []string{site}
			res, err := stage.db.Select(ctx.ctx, query)
			if err != nil {
				ctx.addError(err)
				//return err
			} else if len(res.Rows) == 0 {
				delete(sites, site)
			}
		}
	}
	for site := range sites {
		brickresp.Sites = append(brickresp.Sites, site)
	}
	ctx.qualify_done <- brickresp

	return nil
}

func (stage *BrickQueryStage) processQuery(ctx Context) error {
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
		startIdx := rewriteQuery(reqstream.DataVars, query)
		for _, sitename := range ctx.request.Sites {
			query.Graphs = []string{sitename}
			res, err := stage.db.Select(ctx.ctx, query)
			if err != nil {
				ctx.addError(err)
				//return err
			}

			// TODO: if we have no results from anywhere, need to notify the user and terminate early

			// collate the UUIDs from query results and push into context.
			// Because the rewritten query puts all of the new variables corresponding to the possible UUIDs at the end,
			// the rewriteQuery method has to return the index that we start with when iterating through the variables in
			// each row to make sure we get the actual queries.
			stream := ctx.request.Streams[idx]

			brickresp := &mortarpb.FetchResponse{}

			brickresp.Variable = reqstream.Name
			brickresp.Variables = res.Variables
			brickresp.Site = sitename
			for _, row := range res.Rows {
				for uuidx := startIdx; uuidx < len(query.Vars); uuidx++ {
					stream.Uuids = append(stream.Uuids, row.Values[uuidx].Value)
				}
				// we also add the query results to the output
				brickresp.Rows = append(brickresp.Rows, transformRow(row))
			}
			// send the query results to the client
			// TODO: make this streaming?
			ctx.done <- brickresp
		}

	}
	// signal that we are done processing this stage (1x)
	stage.output <- ctx
	return nil
}

func rewriteQuery(datavars []string, query *logpb.SelectQuery) int {
	var newtriples []*logpb.Triple
	var newselect []string
	uuidPred := logpb.URI{Namespace: "https://brickschema.org/schema/1.0.3/BrickFrame", Value: "uuid"}

	for _, varname := range datavars {
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

func transformRow(r *logpb.Row) *mortarpb.Row {
	newr := &mortarpb.Row{}
	for _, rr := range r.Values {
		newr.Values = append(newr.Values, &mortarpb.URI{Namespace: rr.Namespace, Value: rr.Value})
	}
	return newr
}
