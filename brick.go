package main

import (
	"context"
	"fmt"
	"git.sr.ht/~gabe/hod/hod"
	"github.com/pkg/errors"
	"log"
	"sync"
	"time"
)

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

	hodcfg, err := hod.ReadConfig("hodconfig.yml")
	if err != nil {
		return nil, err
	}
	stage.db, err = hod.NewLog(hodcfg)
	if err != nil {
		return nil, err
	}
	_, err = stage.db.LoadFile("soda", "BrickFrame.ttl", "brickframe")
	if err != nil {
		log.Fatal(errors.Wrap(err, "load brickframe"))
	}
	_, err = stage.db.LoadFile("soda", "Brick.ttl", "brick")
	if err != nil {
		log.Fatal(errors.Wrap(err, "load brick"))
	}
	_, err = stage.db.LoadFile("soda", "berkeley.ttl", "berkeley")
	if err != nil {
		log.Fatal(errors.Wrap(err, "load berkeley"))
	}
	stage.highwatermark = time.Now().UnixNano()
	//q := "SELECT ?x ?y FROM soda WHERE { ?r rdf:type brick:Room . ?x ?y ?r };"
	// TODO: https://todo.sr.ht/%7Egabe/hod/1

	// preseed
	q := "SELECT ?vav FROM soda WHERE { ?vav rdf:type brick:VAV };"
	query, err := stage.db.ParseQuery(q, stage.highwatermark)
	if err != nil {
		return nil, err
	}
	// TODO: rewrite query to get points and units
	_, err = stage.db.Select(stage.ctx, query)
	if err != nil {
		return nil, err
	}

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

	for _, reqstream := range ctx.request.Streams {
		query, err := stage.db.ParseQuery(reqstream.Definition, stage.highwatermark)
		if err != nil {
			ctx.addError(err)
			return err
		}
		// TODO: rewrite query to get points and units
		res, err := stage.db.Select(qctx, query)
		if err != nil {
			ctx.addError(err)
			return err
		}
		_ = res
		// TODO: extract UUIDs from query results and push into context
		stage.output <- ctx
		//fmt.Printf("Err: %v, # vars %d, count %d, num rows %d\n", res.Error, len(res.Variables), int(res.Count), len(res.Rows))
	}
	// brick query
	return nil
}
