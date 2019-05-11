package stages

import (
	"github.com/pkg/errors"
	"time"
)

var MAX_TIMEOUT = time.Second * 300
var TS_BATCH_SIZE = 500
var errStreamNotExist = errors.New("Stream does not exist")

type Stage interface {
	// get the stage we pull from
	GetUpstream() Stage
	// set the stage we pull from
	SetUpstream(upstream Stage)
	// blocks on internal channel until next Request is ready
	GetQueue() chan *Request
	String() string
}
