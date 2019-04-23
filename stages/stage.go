package stages

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/errors"
	"sync"
	"time"
)

var MAX_TIMEOUT = time.Second * 300
var TS_BATCH_SIZE = 500
var errStreamNotExist = errors.New("Stream does not exist")

type Context struct {
	ctx             context.Context
	qualify_request mortarpb.QualifyRequest
	request         mortarpb.FetchRequest
	response        *mortarpb.FetchResponse
	done            chan *mortarpb.FetchResponse
	qualify_done    chan *mortarpb.QualifyResponse
	errors          []error
	finished        bool
	sync.Mutex
}

func (ctx *Context) isDone() bool {
	select {
	case <-ctx.ctx.Done():
		return true
	default:
		return false
	}
}

func (ctx *Context) addError(err error) {
	ctx.Lock()
	defer ctx.Unlock()
	log.Error("Context Error", err)
	ctx.errors = append(ctx.errors, err)
}

func (ctx *Context) finish() {
	ctx.Lock()
	defer ctx.Unlock()
	ctx.finished = true
	ctx.done <- nil
}

func (ctx *Context) is_finished() bool {
	ctx.Lock()
	defer ctx.Unlock()
	return ctx.finished
}

type Stage interface {
	// get the stage we pull from
	GetUpstream() Stage
	// set the stage we pull from
	SetUpstream(upstream Stage)
	// blocks on internal channel until next "Context" is ready
	GetQueue() chan Context
	String() string
}

func Showtime(queue chan Context) {
	go func() {
		log.Println("get output")
		for out := range queue {
			if out.isDone() {
				continue
			}
			if out.response == nil {
				if out.done != nil {
					close(out.done)
				}
				if out.qualify_done != nil {
					close(out.qualify_done)
				}
			} else {
				out.done <- out.response
			}
		}
	}()
}
