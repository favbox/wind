package sse

import (
	"bytes"
	"testing"

	"github.com/favbox/wind/common/json"
	"github.com/stretchr/testify/assert"
)

type myStruct struct {
	A int
	B string `json:"value"`
}

func unsafeMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestEncode(t *testing.T) {
	tests := []struct {
		Name    string
		Event   *Event
		WantErr byte
		Want    string
	}{
		{
			Name:  "只有数据",
			Event: &Event{Data: []byte("junk\n\njk\nid:fake")},
			Want: `data:junk
data:
data:jk
data:id:fake

`,
		},
		{
			Name: "有事件",
			Want: `event:t\n:<>\r	est
data:junk
data:
data:jk
data:id:fake

`,
			Event: &Event{
				Event: "t\n:<>\r\test",
				Data:  []byte("junk\n\njk\nid:fake"),
			},
		},
		{
			Name: "有ID",
			Event: &Event{
				ID:   "t\n:<>\r\test",
				Data: []byte("junk\n\njk\nid:fa\rke"),
			},
			Want: `id:t\n:<>\r	est
data:junk
data:
data:jk
data:id:fa\rke

`,
		},
		{
			Name: "有重试",
			Event: &Event{
				Retry: 11,
				Data:  []byte("junk\n\njk\nid:fake\n"),
			},
			Want: `retry:11
data:junk
data:
data:jk
data:id:fake
data:

`,
		},
		{
			Name: "啥都有",
			Event: &Event{
				Event: "abc",
				ID:    "12345",
				Retry: 10,
				Data:  []byte("some data"),
			},
			Want: "id:12345\nevent:abc\nretry:10\ndata:some data\n\n",
		},
		{
			Name: "数据含结构体",
			Event: &Event{
				Event: "a struct",
				Data:  unsafeMarshal(myStruct{1, "number"}),
			},
			Want: "event:a struct\ndata:{\"A\":1,\"value\":\"number\"}\n\n",
		},
		{
			Name: "数据含结构体指针",
			Event: &Event{
				Event: "a struct",
				Data:  unsafeMarshal(&myStruct{1, "number"}),
			},
			Want: "event:a struct\ndata:{\"A\":1,\"value\":\"number\"}\n\n",
		},
		{
			Name: "整数编码",
			Event: &Event{
				Event: "an integer",
				Data:  []byte("1"),
			},
			Want: "event:an integer\ndata:1\n\n",
		},
		{
			Name: "浮点数编码",
			Event: &Event{
				Event: "Float",
				Data:  []byte("1.5"),
			},
			Want: "event:Float\ndata:1.5\n\n",
		},
		{
			Name: "字符串编码",
			Event: &Event{
				Event: "String",
				Data:  []byte("hertz"),
			},
			Want: "event:String\ndata:hertz\n\n",
		},
	}

	for _, test := range tests {
		var b bytes.Buffer
		err := Encode(&b, test.Event)
		got := b.String()
		assert.Nil(t, err)
		assert.Equal(t, got, test.Want)
	}
}

func TestEncodeStream(t *testing.T) {
	w := new(bytes.Buffer)
	event := &Event{
		Event: "float",
		Data:  []byte("1.5"),
	}
	err := Encode(w, event)
	assert.Nil(t, err)

	event = &Event{
		ID:   "123",
		Data: unsafeMarshal(map[string]interface{}{"foo": "bar", "bar": "foo"}),
	}
	err = Encode(w, event)
	assert.Nil(t, err)

	event = &Event{
		ID:    "124",
		Event: "chat",
		Data:  []byte("hi! dude"),
	}
	err = Encode(w, event)
	assert.Nil(t, err)
	assert.Equal(t, "event:float\ndata:1.5\n\nid:123\ndata:{\"bar\":\"foo\",\"foo\":\"bar\"}\n\nid:124\nevent:chat\ndata:hi! dude\n\n", w.String())
}

func BenchmarkFullSSE(b *testing.B) {
	buf := new(bytes.Buffer)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			ID:    "13435",
			Retry: 10,
			Data:  []byte("hi! how are you? I am fine. this is a long stupid message!!!"),
		})
		buf.Reset()
	}
}

func BenchmarkNoRetrySSE(b *testing.B) {
	buf := new(bytes.Buffer)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			ID:    "13435",
			Data:  []byte("hi! how are you? I am fine. this is a long stupid message!!!"),
		})
		buf.Reset()
	}
}

func BenchmarkSimpleSSE(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	buf := new(bytes.Buffer)

	for i := 0; i < b.N; i++ {
		_ = Encode(buf, &Event{
			Event: "new_message",
			Data:  []byte("hi! how are you? I am fine. this is a long stupid message!!!"),
		})
		buf.Reset()
	}
}
