package stages

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
	"sync"
)

type Request struct {
	sync.Mutex

	ctx    context.Context
	cancel func()
	err    error

	qualify_request *mortarpb.QualifyRequest
	fetch_request   *mortarpb.FetchRequest

	fetch_responses   chan *mortarpb.FetchResponse
	qualify_responses chan *mortarpb.QualifyResponse
}

func NewQualifyRequest(ctx context.Context, qualify *mortarpb.QualifyRequest) *Request {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	//defer cancel()

	req := &Request{
		ctx:               ctx,
		cancel:            cancel,
		qualify_request:   qualify,
		qualify_responses: make(chan *mortarpb.QualifyResponse),
	}

	return req
}

func NewFetchRequest(ctx context.Context, fetch *mortarpb.FetchRequest) *Request {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	//defer cancel()

	req := &Request{
		ctx:             ctx,
		cancel:          cancel,
		fetch_request:   fetch,
		fetch_responses: make(chan *mortarpb.FetchResponse),
	}

	return req
}

func (request *Request) addError(err error) {
	request.Lock()
	defer request.Unlock()
	if request.err != nil {
		return
	}
	request.err = err
	if request.fetch_responses != nil {
		request.fetch_responses <- &mortarpb.FetchResponse{
			Error: err.Error(),
		}
	} else if request.qualify_responses != nil {
		request.qualify_responses <- &mortarpb.QualifyResponse{
			Error: err.Error(),
		}
	}
}

func (request *Request) finish() {
	request.Lock()
	defer request.Unlock()
	if request.fetch_responses != nil {
		request.fetch_responses <- nil
	} else if request.qualify_responses != nil {
		request.qualify_responses <- nil
	}
}

func (request *Request) Done() <-chan struct{} {
	request.Lock()
	defer request.Unlock()
	return request.ctx.Done()
}

//func (request *Request) handle() {
//	go func() {
//		if request.qualify_responses != nil {
//			for outmsg := range request.qualify_responses {
//				log.Println(outmsg)
//			}
//		}
//		//for out := range queue
//	}()
//}
