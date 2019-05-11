package stages

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
	output   chan *Request

	db            *hod.HodDB
	highwatermark int64

	sync.Mutex
}

type BrickQueryStageConfig struct {
	Upstream          Stage
	StageContext      context.Context
	HodConfigLocation string
}

func NewBrickQueryStage(cfg *BrickQueryStageConfig) (*BrickQueryStage, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("Need to specify Upstream in Metadata config")
	}
	stage := &BrickQueryStage{
		upstream: cfg.Upstream,
		output:   make(chan *Request),
		ctx:      cfg.StageContext,
	}

	log.Info("Start loading Brick config")
	start := time.Now()
	hodcfg, err := hod.ReadConfig(cfg.HodConfigLocation)
	if err != nil {
		return nil, err
	}
	stage.db, err = hod.MakeHodDB(hodcfg)
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
				case req := <-input:
					if req.fetch_request != nil {
						// handle metadata stage of fetch request
						if len(req.fetch_request.Sites) > 0 && len(req.fetch_request.Views) > 0 {
							if err := stage.processQuery(req); err != nil {
								log.Println(err)
								req.addError(err)
							}
						}
						stage.output <- req
					} else if req.qualify_request != nil {
						// handle qualify request
						if len(req.qualify_request.Required) > 0 {
							if err := stage.processQualify(req); err != nil {
								req.addError(err)
								req.qualify_responses <- &mortarpb.QualifyResponse{
									Error: err.Error(),
								}
							}
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
func (stage *BrickQueryStage) GetQueue() chan *Request {
	return stage.output
}
func (stage *BrickQueryStage) String() string {
	return "<| brick stage |>"
}

func (stage *BrickQueryStage) processQualify(req *Request) error {
	brickresp := &mortarpb.QualifyResponse{}

	sites := make(map[string]struct{})

	version_query := &logpb.VersionQuery{
		Graphs:    []string{"*"},
		Filter:    logpb.TimeFilter_At,
		Timestamp: time.Now().UnixNano(),
	}
	version_response, err := stage.db.Versions(req.ctx, version_query)
	if err != nil {
		req.addError(err)
		log.Error(err)
		return err
	}
	if version_response.Error != "" {
		err = errors.New(version_response.Error)
		log.Error(err)
		req.addError(err)
		return err
	}

	for _, row := range version_response.Rows {
		sites[row.Values[0].Value] = struct{}{}
	}

	for _, querystring := range req.qualify_request.Required {
		query, err := stage.db.ParseQuery(querystring, 0)
		if err != nil {
			req.addError(err)
			log.Error(err)
			return err
		}

		for site := range sites {
			query.Graphs = []string{site}
			res, err := stage.db.Select(req.ctx, query)
			if err != nil {
				log.Error(err)
				req.addError(err)
				//return err
			} else if len(res.Rows) == 0 {
				delete(sites, site)
			}
		}
	}
	for site := range sites {
		brickresp.Sites = append(brickresp.Sites, site)
	}
	req.qualify_responses <- brickresp

	return nil
}

// We need to rethink how the Brick stage handles the view + dataFrame processing

func (stage *BrickQueryStage) processQuery(req *Request) error {
	// store view name -> list of dataVars
	var viewDataVars = make(map[string][]string)
	// store view name -> list of indexes to dependent dataFrames
	var viewDataFrames = make(map[string][]int)
	for idx, dataFrame := range req.fetch_request.DataFrames {
		idx := idx
	tsLoop:
		for _, timeseries := range dataFrame.Timeseries {
			viewDataVars[timeseries.View] = append(viewDataVars[timeseries.View], timeseries.DataVars...)

			// add the index of the dataFrame to the list associated with the view if it doesn't already exist in the list
			for _, selIdx := range viewDataFrames[timeseries.View] {
				if selIdx == idx {
					continue tsLoop
				}
			}
			viewDataFrames[timeseries.View] = append(viewDataFrames[timeseries.View], idx)

		}
	}

	log.Info("DataVars: ", viewDataVars)
	log.Info("DataFrames: ", viewDataFrames)

	for _, view := range req.fetch_request.Views {
		query, err := stage.db.ParseQuery(view.Definition, stage.highwatermark)
		if err != nil {
			req.addError(err)
			return err
		}

		// this rewrites the incoming query so that it extracts the UUIDs (bf:uuid property) for each of the
		// variables in the SELECT clause of the query. This removes the need for the user to know that the bf:uuid
		// property is how to relate the points to the timeseries database. However, it also introduces the complexity
		// of dealing with whether or not the variables *do* have associated timeseries or not.
		mapping, _ := rewriteQuery(viewDataVars[view.Name], query)
		for _, sitename := range req.fetch_request.Sites {
			query.Graphs = []string{sitename}
			res, err := stage.db.Select(req.ctx, query)
			if err != nil {
				log.Error(err)
				req.addError(err)
				continue
				//return err
			}

			// collate the UUIDs from query results and push into context.
			// Because the rewritten query puts all of the new variables corresponding to the possible UUIDs at the end,
			// the rewriteQuery method has to return the index that we start with when iterating through the variables in
			// each row to make sure we get the actual queries.
			//stream := ctx.request.Streams[idx]

			// need to associate the results of the query with this view:
			// - site name
			// - view name
			// - variables involved in the query
			brickresp := &mortarpb.FetchResponse{}
			brickresp.Site = sitename
			brickresp.View = view.Name
			brickresp.Variables = res.Variables

			for _, row := range res.Rows {
				//	// for each dependent dataFrame
				for _, selIdx := range viewDataFrames[view.Name] {
					// for each timeseries
					dataFrame := req.fetch_request.DataFrames[selIdx]
					// add uuids to the list on the DataFrame for each datavar from the view we're currently working with
					for _, ts := range dataFrame.Timeseries {
						if ts.View == view.Name {
							for _, dataVar := range ts.DataVars {
								uuidx := mapping[dataVar]
								dataFrame.Uuids = append(dataFrame.Uuids, stripQuotes(row.Values[uuidx].Value))
							}
						}
					}
					// TODO: do we need to update the dataFrame?
					req.fetch_request.DataFrames[selIdx] = dataFrame
				}
				//}
				// we also add the query results to the output
				brickresp.Rows = append(brickresp.Rows, transformRow(row))
			}

			// send the query results to the client
			req.fetch_responses <- brickresp
		}

	}
	return nil
}

// startIdx gives the index into the row where the UUIDs start
// mapping stores variable name -> index where the UUID is in the rewritten query
func rewriteQuery(datavars []string, query *logpb.SelectQuery) (mapping map[string]int, startIdx int) {
	var newtriples []*logpb.Triple
	var newselect []string
	mapping = make(map[string]int)
	uuidPred := logpb.URI{Namespace: "https://brickschema.org/schema/1.0.3/BrickFrame", Value: "uuid"}

	for _, varname := range datavars {
		basevarname := strings.TrimPrefix(varname, "?")
		basevarname_uuid := "?" + basevarname + "_uuid"
		newtriples = append(newtriples, &logpb.Triple{Subject: &logpb.URI{Value: varname}, Predicate: []*logpb.URI{&uuidPred}, Object: &logpb.URI{Value: basevarname_uuid}})
		mapping[varname] = len(query.Vars) + len(newselect)
		newselect = append(newselect, basevarname_uuid)
	}

	oldidx := len(query.Vars)
	query.Where = append(query.Where, newtriples...)
	query.Vars = append(query.Vars, newselect...)
	return mapping, oldidx
}

func transformRow(r *logpb.Row) *mortarpb.Row {
	newr := &mortarpb.Row{}
	for _, rr := range r.Values {
		newr.Values = append(newr.Values, &mortarpb.URI{Namespace: rr.Namespace, Value: rr.Value})
	}
	return newr
}

func stripQuotes(s string) string {
	if s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
