package ut

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/route"
	"github.com/stretchr/testify/assert"
)

func newTestEngine() *route.Engine {
	opt := config.NewOptions(nil)
	return route.NewEngine(opt)
}

func createChunkedBody(body []byte) []byte {
	var b []byte
	chunkSize := 3
	for len(body) > 0 {
		if chunkSize > len(body) {
			chunkSize = len(body)
		}
		b = append(b, []byte(fmt.Sprintf("%x\r\n", chunkSize))...)
		b = append(b, body[:chunkSize]...)
		b = append(b, []byte("\r\n")...)
		body = body[chunkSize:]
		chunkSize++
	}
	return append(b, []byte("0\r\n\r\n")...)
}

func TestPerformRequest(t *testing.T) {
	router := newTestEngine()
	router.PUT("/hey/:user", func(ctx context.Context, c *app.RequestContext) {
		user := c.Param("user")
		if string(c.Request.Body()) == "1" {
			assert.True(t, c.Request.ConnectionClose())
			c.Response.SetConnectionClose()
			c.JSON(consts.StatusCreated, map[string]string{"hi": user})
		} else if string(c.Request.Body()) == "" {
			c.AbortWithMsg("未授权", consts.StatusUnauthorized)
		} else {
			assert.Equal(t, "PUT /hey/dy HTTP/1.1\r\nContent-Type: application/x-www-form-urlencoded\r\nTransfer-Encoding: chunked\r\n\r\n", string(c.Request.Header.Header()))
			c.String(consts.StatusAccepted, "body:%v", string(c.Request.Body()))
		}
	})
	router.GET("/her/header", func(ctx context.Context, c *app.RequestContext) {
		assert.Equal(t, "application/json", string(c.GetHeader("Content-Type")))
		assert.Equal(t, 1, c.Request.Header.ContentLength())
		assert.Equal(t, "a", c.Request.Header.Get("dummy"))
	})

	// 验证用户
	w := PerformRequest(router, "PUT", "/hey/dy", &Body{bytes.NewBufferString("1"), 1},
		Header{"Connection", "close"})
	resp := w.Result()
	assert.Equal(t, consts.StatusCreated, resp.StatusCode())
	assert.Equal(t, `{"hi":"dy"}`, string(resp.Body()))
	assert.Equal(t, "application/json; charset=utf-8", string(resp.Header.ContentType()))
	assert.True(t, resp.Header.ConnectionClose())

	// 未授权用户
	w = PerformRequest(router, "PUT", "/hey/dy", nil)
	_ = w.Result()
	resp = w.Result()
	assert.Equal(t, consts.StatusUnauthorized, resp.StatusCode())
	assert.Equal(t, "未授权", string(resp.Body()))
	assert.Equal(t, "text/plain; charset=utf-8", string(resp.Header.ContentType()))
	assert.Equal(t, 3*3, resp.Header.ContentLength())

	// 特殊标头
	PerformRequest(router, "GET", "/her/header", nil,
		Header{"content-type", "application/json"},
		Header{"content-length", "1"},
		Header{"dummy", "a"},
		Header{"dummy", "b"},
	)

	// 未找到
	w = PerformRequest(router, "GET", "/hey", nil)
	resp = w.Result()
	assert.Equal(t, consts.StatusNotFound, resp.StatusCode())

	// 假冒的正文
	w = PerformRequest(router, "GET", "/hey", nil)
	_, err := w.WriteString("，仿冒者")
	resp = w.Result()
	assert.Nil(t, err)
	assert.Equal(t, consts.StatusNotFound, resp.StatusCode())
	assert.Equal(t, "404 页面未找到，仿冒者", string(resp.Body()))

	// 分块响应的正文
	body := bytes.NewReader(createChunkedBody([]byte("hello world!")))
	w = PerformRequest(router, "PUT", "/hey/dy", &Body{body, -1})
	resp = w.Result()
	assert.Equal(t, consts.StatusAccepted, resp.StatusCode())
	assert.Equal(t, "body:3\r\nhel\r\n4\r\nlo w\r\n5\r\norld!\r\n0\r\n\r\n", string(resp.Body()))

}
