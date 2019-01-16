package main

import (
	"fmt"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/errors"
	"time"
)

func validateFetchRequest(req *mortarpb.FetchRequest) error {
	// check the list of sites is non-empty
	if len(req.Sites) == 0 {
		return errors.New("Need to include non-empty request.Sites")
	}

	// check that there are non-zero number of streams
	if len(req.Streams) == 0 {
		return errors.New("Need to include non-empty request.Streams")
	}

	hasWindowAgg := false
	for idx, stream := range req.Streams {
		// streams must have a name
		if stream.Name == "" {
			return fmt.Errorf("Stream %d must have a .Name", idx)
		}

		// streams EITHER have a definition (requiring Definition and DataVars)
		// or they have a list of UUIDs
		if stream.Definition != "" && len(stream.DataVars) == 0 {
			return fmt.Errorf("If stream %d has a Definition, it also needs a list of DataVars", idx)
		} else if stream.Definition == "" && len(stream.Uuids) == 0 {
			return fmt.Errorf("Stream %d has no Definition, so it needs a list of UUIDS", idx)
		}

		if stream.Aggregation == mortarpb.AggFunc_AGG_FUNC_INVALID {
			return fmt.Errorf("Stream %d has no aggregation function (can be RAW)", idx)
		}
		if stream.Aggregation != mortarpb.AggFunc_AGG_FUNC_RAW {
			hasWindowAgg = true
		}

		// TODO: check units?
	}

	// check time params
	if req.Time == nil {
		return errors.New("Need to include non-empty request.Time")
	}

	// parse the times to check
	if _, err := time.Parse(time.RFC3339, req.Time.Start); err != nil {
		return errors.Wrapf(err, "request.Time.Start is not RFC3339-formatted timestamp (%s)", req.Time.Start)
	}
	if _, err := time.Parse(time.RFC3339, req.Time.End); err != nil {
		return errors.Wrapf(err, "request.Time.End is not RFC3339-formatted timestamp (%s)", req.Time.End)
	}

	if hasWindowAgg && req.Time.Window == "" {
		return errors.New("One of your stream uses a windowed aggregation function e.g. MEAN. Need to provide a valid request.Time.Window")
	}

	return nil
}
