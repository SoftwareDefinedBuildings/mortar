package stages

import (
	"context"
	mortarpb "git.sr.ht/~gabe/mortar/proto"
)

type Request struct {
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
	if request.err == nil {
		request.err = err
	}
}

func (request *Request) Done() <-chan struct{} {
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
