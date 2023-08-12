package client

import (
	"context"
	"testing"

	"github.com/favbox/wind/protocol"
)

var (
	biz       = "Biz"
	beforeMW0 = "BeforeMiddleware0"
	afterMW0  = "AfterMiddleware0"
	beforeMW1 = "BeforeMiddleware1"
	afterMW1  = "AfterMiddleware1"
)

func invoke(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
	req.BodyBuffer().WriteString(biz)
	return nil
}

func mockMW0(next Endpoint) Endpoint {
	return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
		req.BodyBuffer().WriteString(beforeMW0)
		err = next(ctx, req, resp)
		if err != nil {
			return err
		}
		req.BodyBuffer().WriteString(afterMW0)
		return nil
	}
}

func mockMW1(next Endpoint) Endpoint {
	return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
		req.BodyBuffer().WriteString(beforeMW1)
		err = next(ctx, req, resp)
		if err != nil {
			return err
		}
		req.BodyBuffer().WriteString(afterMW1)
		return nil
	}
}

func TestChain(t *testing.T) {
	mws := chain(mockMW0, mockMW1)
	req := protocol.AcquireRequest()
	mws(invoke)(context.Background(), req, nil)
	final := beforeMW0 + beforeMW1 + biz + afterMW1 + afterMW0
	if req.BodyBuffer().String() != final {
		t.Errorf("unexpected %#v, expected %#v", req.BodyBuffer().String(), final)
	}
}
