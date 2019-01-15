package main

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"github.com/pkg/errors"
	"sync"
	"time"
)

var MAX_TIMEOUT = time.Second * 300
var TS_BATCH_SIZE = 10
var errStreamNotExist = errors.New("Stream does not exist")

type Context struct {
	ctx      context.Context
	request  mortarpb.FetchRequest
	response *mortarpb.FetchResponse
	done     chan *mortarpb.FetchResponse
	errors   []error
	sync.Mutex
}

func (ctx *Context) addError(err error) {
	ctx.Lock()
	defer ctx.Unlock()
	ctx.errors = append(ctx.errors, err)
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
