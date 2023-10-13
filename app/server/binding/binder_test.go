package binding

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/favbox/wind/app/server/binding/testdata"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	req2 "github.com/favbox/wind/protocol/http1/req"
	"github.com/favbox/wind/route/param"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

type mockRequest struct {
	Req *protocol.Request
}

func newMockRequest() *mockRequest {
	return &mockRequest{
		Req: &protocol.Request{},
	}
}

func (m *mockRequest) SetRequestURI(uri string) *mockRequest {
	m.Req.SetRequestURI(uri)
	return m
}

func (m *mockRequest) SetFile(param, fileName string) *mockRequest {
	m.Req.SetFile(param, fileName)
	return m
}

func (m *mockRequest) SetHeader(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetHeaders(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetPostArg(key, value string) *mockRequest {
	m.Req.PostArgs().Add(key, value)
	return m
}

func (m *mockRequest) SetUrlEncodedContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	return m
}

func (m *mockRequest) SetJSONContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	return m
}

func (m *mockRequest) SetProtobufContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEPROTOBUF))
	return m
}

func (m *mockRequest) SetBody(data []byte) *mockRequest {
	m.Req.SetBody(data)
	m.Req.Header.SetContentLength(len(data))
	return m
}

func TestBind_BaseType(t *testing.T) {
	type Req struct {
		Version int    `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"H"`
		Form    string `form:"f"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodedContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	var result Req
	err := DefaultBinder().Bind(req.Req, &result, params)
	assert.Nil(t, err)
	assert.Equal(t, 1, result.Version)
	assert.Equal(t, 12, result.ID)
	assert.Equal(t, "header", result.Header)
	assert.Equal(t, "form", result.Form)
}

func TestBind_SliceType(t *testing.T) {
	type Req struct {
		ID   *[]int    `query:"id"`
		Str  [3]string `query:"str"`
		Byte []byte    `query:"b"`
	}

	IDs := []int{11, 12, 13}
	Strs := [3]string{"qwe", "asd", "zxc"}
	Bytes := []byte("123")

	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?id=%d&id=%d&id=%d&str=%s&str=%s&str=%s&b=%d&b=%d&b=%d", IDs[0], IDs[1], IDs[2], Strs[0], Strs[1], Strs[2], Bytes[0], Bytes[1], Bytes[2]))

	var result Req
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(*result.ID))
	for idx, val := range IDs {
		assert.Equal(t, val, (*result.ID)[idx])
	}
	for idx, val := range Strs {
		assert.Equal(t, val, result.Str[idx])
	}
	assert.Equal(t, 3, len(result.Byte))
	for idx, val := range Bytes {
		assert.Equal(t, val, result.Byte[idx])
	}
}

func TestBind_StructType(t *testing.T) {
	type FFF struct {
		F1 string `query:"F1"`
	}

	type TTT struct {
		T1 string `query:"F1"`
		T2 FFF
	}

	type Foo struct {
		F1 string `query:"F1"`
		F2 string `header:"f2"`
		F3 TTT
	}

	type Bar struct {
		B1 string `query:"B1"`
		B2 Foo    `query:"B2"`
	}

	var result Bar
	req := newMockRequest().SetRequestURI("http://foobar.com?F1=f1&B1=b1").SetHeaders("f2", "f2")
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)

	assert.Equal(t, "b1", result.B1)
	assert.Equal(t, "f1", result.B2.F1)
	assert.Equal(t, "f2", result.B2.F2)
	assert.Equal(t, "f1", result.B2.F3.T1)
	assert.Equal(t, "f1", result.B2.F3.T2.F1)
}

func TestBind_PointerType(t *testing.T) {
	type TT struct {
		T1 string `query:"F1"`
	}

	type Foo struct {
		F1 *TT                       `query:"F1"`
		F2 *******************string `query:"F1"`
	}

	type Bar struct {
		B1 ***string `query:"B1"`
		B2 ****Foo   `query:"B2"`
		B3 []*string `query:"B3"`
		B4 [2]*int   `query:"B4"`
	}

	result := Bar{}

	F1 := "f1"
	B1 := "b1"
	B2 := "b2"
	B3s := []string{"b31", "b32"}
	B4s := [2]int{0, 1}

	req := newMockRequest().SetRequestURI(fmt.Sprintf("http://foobar.com?F1=%s&B1=%s&B2=%s&B3=%s&B3=%s&B4=%d&B4=%d", F1, B1, B2, B3s[0], B3s[1], B4s[0], B4s[1])).
		SetHeader("f2", "f2")

	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, B1, ***result.B1)
	assert.Equal(t, F1, (*(****result.B2).F1).T1)
	assert.Equal(t, F1, *******************(****result.B2).F2)
	assert.Equal(t, len(B3s), len(result.B3))
	for idx, val := range B3s {
		assert.Equal(t, val, *result.B3[idx])
	}
	assert.Equal(t, len(B4s), len(result.B4))
	for idx, val := range B4s {
		assert.Equal(t, val, *result.B4[idx])
	}
}

func TestBind_NestedStruct(t *testing.T) {
	type Foo struct {
		F1 string `query:"F1"`
	}

	type Bar struct {
		Foo
		Nested struct {
			N1 string `query:"F1"`
		}
	}

	result := Bar{}

	req := newMockRequest().SetRequestURI("http://foobar.com?F1=qwe")
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "qwe", result.Foo.F1)
	assert.Equal(t, "qwe", result.Nested.N1)
}

func TestBind_SliceStruct(t *testing.T) {
	type Foo struct {
		F1 string `json:"f1"`
	}

	type Bar struct {
		B1 []Foo `query:"F1"`
	}

	result := Bar{}
	B1s := []string{"1", "2", "3"}

	req := newMockRequest().SetRequestURI(fmt.Sprintf("http://foobar.com?F1={\"f1\":\"%s\"}&F1={\"f1\":\"%s\"}&F1={\"f1\":\"%s\"}", B1s[0], B1s[1], B1s[2]))
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, len(result.B1), len(B1s))
	for idx, val := range B1s {
		assert.Equal(t, B1s[idx], val)
	}
}

func TestBind_MapType(t *testing.T) {
	var result map[string]string
	req := newMockRequest().
		SetJSONContentType().
		SetBody([]byte(`{"j1":"j1", "j2":"j2"}`))
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "j1", result["j1"])
	assert.Equal(t, "j2", result["j2"])
}

func TestBind_MapFieldType(t *testing.T) {
	type Foo struct {
		F1 ***map[string]string `query:"f1" json:"f1"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?f1={\"f1\":\"f1\"}").
		SetJSONContentType().
		SetBody([]byte(`{"j1":"j1", "j2":"j2"}`))
	result := Foo{}
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(***result.F1))
	assert.Equal(t, "f1", (***result.F1)["f1"])

	type Foo2 struct {
		F1 map[string]string `query:"f1" json:"f1"`
	}
	result2 := Foo2{}
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result2.F1))
	assert.Equal(t, "f1", result2.F1["f1"])

	req = newMockRequest().
		SetRequestURI("http://foobar.com?f1={\"f1\":\"f1\"")
	result2 = Foo2{}
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	assert.NotNil(t, err)
}

func TestBind_UnexpectedField(t *testing.T) {
	var s struct {
		A int `query:"a"`
		b int `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, s.A)
	assert.Equal(t, 0, s.b)
}

func TestBind_NoTagField(t *testing.T) {
	var s struct {
		A string
		B string
		C string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?B=b1&C=c1").
		SetHeader("A", "a2")

	var params param.Params
	params = append(params, param.Param{
		Key:   "B",
		Value: "b2",
	})

	err := DefaultBinder().Bind(req.Req, &s, params)
	assert.Nil(t, err)
	assert.Equal(t, "a2", s.A)
	assert.Equal(t, "b2", s.B)
	assert.Equal(t, "c1", s.C)
}

func TestBind_ZeroValueBind(t *testing.T) {
	var s struct {
		A int     `query:"a"`
		B float64 `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=&b")

	bindConfig := &BindConfig{}
	bindConfig.LooseZeroMode = true
	binder := NewBinder(bindConfig)
	err := binder.Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, s.A)
	assert.Equal(t, float64(0), s.B)
}

func TestBind_DefaultValueBind(t *testing.T) {
	var s struct {
		A int      `default:"15"`
		B float64  `query:"b" default:"17"`
		C []int    `default:"15"`
		D []string `default:"qwe"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com")

	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, 15, s.A)
	assert.Equal(t, float64(17), s.B)
	assert.Equal(t, 15, s.C[0])
	assert.Equal(t, "qwe", s.D[0])

	var d struct {
		D [2]string `default:"qwe"`
	}

	err = DefaultBinder().Bind(req.Req, &d, nil)
	assert.NotNil(t, err)
}

func TestBind_RequiredBind(t *testing.T) {
	var s struct {
		A int `query:"a,required"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeader("A", "1")

	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.NotNil(t, err)

	var d struct {
		A int `query:"a,required" header:"A"`
	}
	err = DefaultBinder().Bind(req.Req, &d, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, d.A)
}

func TestBind_TypedefType(t *testing.T) {
	type Foo string
	type Bar *int
	type T struct {
		T1 string `query:"a"`
	}
	type TT T

	var s struct {
		A  Foo `query:"a"`
		B  Bar `query:"b"`
		T1 TT
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, Foo("1"), s.A)
	assert.Equal(t, 2, *s.B)
	assert.Equal(t, "1", s.T1.T1)
}

type EnumType int64

const (
	EnumType_TWEET   EnumType = 0
	EnumType_RETWEET EnumType = 2
)

func (p EnumType) String() string {
	switch p {
	case EnumType_TWEET:
		return "TWEET"
	case EnumType_RETWEET:
		return "RETWEET"
	}
	return "<UNSET>"
}

func TestBind_EnumBind(t *testing.T) {
	var s struct {
		A EnumType `query:"a"`
		B EnumType `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=0&b=2")
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, EnumType_TWEET, s.A)
	assert.Equal(t, EnumType_RETWEET, s.B)
}

type CustomizedDecode struct {
	A string
}

func TestBind_CustomizedTypeDecode(t *testing.T) {
	type Foo struct {
		F ***CustomizedDecode `query:"a"`
	}

	bindConfig := &BindConfig{}
	err := bindConfig.RegTypeUnmarshal(reflect.TypeOf(CustomizedDecode{}), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		q1 := req.URI().QueryArgs().Peek("a")
		if len(q1) == 0 {
			return reflect.Value{}, fmt.Errorf("can be nil")
		}
		val := CustomizedDecode{
			A: string(q1),
		}
		return reflect.ValueOf(val), nil
	})
	assert.Nil(t, err)

	binder := NewBinder(bindConfig)

	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	result := Foo{}
	err = binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "1", (***result.F).A)

	type Bar struct {
		B *Foo
	}

	result2 := Bar{}
	err = binder.Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
	assert.Equal(t, "1", (***(*result2.B).F).A)
}

func TestBind_CustomizedTypeDecodeForPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("期待一个恐慌，但未触发")
		}
	}()

	bindConfig := &BindConfig{}
	bindConfig.MustRegTypeUnmarshal(reflect.TypeOf(string("")), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		return reflect.Value{}, nil
	})
}

func TestBind_BindJSON(t *testing.T) {
	type Req struct {
		J1 string    `json:"j1"`
		J2 int       `json:"j2" query:"j2"` // 1. json 解码 2. query 绑定后覆盖
		J3 []byte    `json:"j3"`
		J4 [2]string `json:"j4"`
	}
	J3s := []byte("12")
	J4s := [2]string{"qwe", "asd"}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody([]byte(fmt.Sprintf(`{"j1":"j1", "j2":12, "j3":[%d, %d], "j4":["%s", "%s"]}`, J3s[0], J3s[1], J4s[0], J4s[1])))
	var result Req
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "j1", result.J1)
	assert.Equal(t, 13, result.J2)
	for idx, val := range J3s {
		assert.Equal(t, val, result.J3[idx])
	}
	for idx, val := range J4s {
		assert.Equal(t, val, result.J4[idx])
	}
}

func TestBind_ResetJSONUnmarshal(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.UseStdJSONUnmarshaler()
	binder := NewBinder(bindConfig)
	type Req struct {
		J1 string    `json:"j1"`
		J2 int       `json:"j2"`
		J3 []byte    `json:"j3"`
		J4 [2]string `json:"j4"`
	}
	J3s := []byte("12")
	J4s := [2]string{"qwe", "asd"}

	req := newMockRequest().
		SetJSONContentType().
		SetBody([]byte(fmt.Sprintf(`{"j1":"j1", "j2":12, "j3":[%d, %d], "j4":["%s", "%s"]}`, J3s[0], J3s[1], J4s[0], J4s[1])))
	var result Req
	err := binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "j1", result.J1)
	assert.Equal(t, 12, result.J2)
	for idx, val := range J3s {
		assert.Equal(t, val, result.J3[idx])
	}
	for idx, val := range J4s {
		assert.Equal(t, val, result.J4[idx])
	}
}

func TestBind_FileBind(t *testing.T) {
	type Nest struct {
		N multipart.FileHeader `file_name:"d"`
	}

	var s struct {
		A *multipart.FileHeader `file_name:"a"`
		B *multipart.FileHeader `form:"b"`
		C multipart.FileHeader
		D **Nest `file_name:"d"`
	}
	fileName := "binder_test.go"
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetFile("a", fileName).
		SetFile("b", fileName).
		SetFile("C", fileName).
		SetFile("d", fileName)
	// 用于解析多部分文件
	req2 := req2.GetHTTP1Request(req.Req)
	_ = req2.String()
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, fileName, s.A.Filename)
	assert.Equal(t, fileName, s.B.Filename)
	assert.Equal(t, fileName, s.C.Filename)
	assert.Equal(t, fileName, (**s.D).N.Filename)
}

func TestBind_FileSliceBind(t *testing.T) {
	type Nest struct {
		N *[]*multipart.FileHeader `form:"b"`
	}
	var s struct {
		A []multipart.FileHeader  `form:"a"`
		B [3]multipart.FileHeader `form:"b"`
		C []*multipart.FileHeader `form:"b"`
		D Nest
	}
	fileName := "binder_test.go"
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetFile("a", fileName).
		SetFile("a", fileName).
		SetFile("a", fileName).
		SetFile("b", fileName).
		SetFile("b", fileName).
		SetFile("b", fileName)
	// 用于解析多部分文件
	req2 := req2.GetHTTP1Request(req.Req)
	_ = req2.String()
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(s.A))
	for _, file := range s.A {
		assert.Equal(t, fileName, file.Filename)
	}
	assert.Equal(t, 3, len(s.B))
	for _, file := range s.B {
		assert.Equal(t, fileName, file.Filename)
	}
	assert.Equal(t, 3, len(s.C))
	for _, file := range s.C {
		assert.Equal(t, fileName, file.Filename)
	}
	assert.Equal(t, 3, len(*s.D.N))
	for _, file := range *s.D.N {
		assert.Equal(t, fileName, file.Filename)
	}
}

func TestBind_AnonymousField(t *testing.T) {
	type nest struct {
		n1     string       `query:"n1"` // bind default value
		N2     ***string    `query:"n2"` // bind n2 value
		string `query:"n3"` // bind default value
	}

	var s struct {
		s1  int          `query:"s1"` // bind default value
		int `query:"s2"` // bind default value
		nest
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?s1=1&s2=2&n1=1&n2=2&n3=3")
	err := DefaultBinder().Bind(req.Req, &s, nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, s.s1)
	assert.Equal(t, 0, s.int)
	assert.Equal(t, "", s.nest.n1)
	assert.Equal(t, "2", ***s.nest.N2)
	assert.Equal(t, "", s.nest.string)
}

func TestBind_IgnoreField(t *testing.T) {
	type Req struct {
		Version int    `path:"-"`
		ID      int    `query:"-"`
		Header  string `header:"-"`
		Form    string `form:"-"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?ID=12").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodedContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "Version",
		Value: "1",
	})

	var result Req

	err := DefaultBinder().Bind(req.Req, &result, params)
	assert.Nil(t, err)
	assert.Equal(t, 0, result.Version)
	assert.Equal(t, 0, result.ID)
	assert.Equal(t, "", result.Header)
	assert.Equal(t, "", result.Form)
}

func TestBind_DefaultTag(t *testing.T) {
	type Req struct {
		Version int
		ID      int
		Header  string
		Form    string
	}
	type Req2 struct {
		Version int
		ID      int
		Header  string
		Form    string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?ID=12").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodedContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "Version",
		Value: "1",
	})
	var result Req
	err := DefaultBinder().Bind(req.Req, &result, params)
	assert.Nil(t, err)
	assert.Equal(t, 1, result.Version)
	assert.Equal(t, 12, result.ID)
	assert.Equal(t, "header", result.Header)
	assert.Equal(t, "form", result.Form)

	bindConfig := &BindConfig{}
	bindConfig.DisableDefaultTag = true
	binder := NewBinder(bindConfig)
	result2 := Req2{}
	err = binder.Bind(req.Req, &result2, params)
	assert.Nil(t, err)
	assert.Equal(t, 0, result2.Version)
	assert.Equal(t, 0, result2.ID)
	assert.Equal(t, "", result2.Header)
	assert.Equal(t, "", result2.Form)
}

func TestBind_StructFieldResolve(t *testing.T) {
	type Nested struct {
		A int `query:"a" json:"a"`
		B int `query:"b" json:"b"`
	}
	type Req struct {
		N Nested `query:"n"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?n={\"a\":1,\"b\":2}").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodedContentType()
	var result Req
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewBinder(bindConfig)
	err := binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, result.N.A)
	assert.Equal(t, 2, result.N.B)

	req = newMockRequest().
		SetRequestURI("http://foobar.com?n={\"a\":1,\"b\":2}&a=11&b=22").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodedContentType()
	err = DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 11, result.N.A)
	assert.Equal(t, 22, result.N.B)
}

func TestBind_JSONRequiredField(t *testing.T) {
	type Nested2 struct {
		C int `json:"c,required"`
		D int `json:"dd,required"`
	}
	type Nested struct {
		A  int     `json:"a,required"`
		B  int     `json:"b,required"`
		N2 Nested2 `json:"n2"`
	}
	type Req struct {
		N Nested `json:"n,required"`
	}
	bodyBytes := []byte(`{
    "n": {
        "a": 1,
        "b": 2,
        "n2": {
             "dd": 4
        }
    }
}`)
	req := newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody(bodyBytes)
	var result Req
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.NotNil(t, err)
	assert.Equal(t, 1, result.N.A)
	assert.Equal(t, 2, result.N.B)
	assert.Equal(t, 0, result.N.N2.C)
	assert.Equal(t, 4, result.N.N2.D)

	bodyBytes = []byte(`{
    "n": {
        "a": 1,
        "b": 2
    }
}`)
	req = newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody(bodyBytes)
	var result2 Req
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	assert.Equal(t, 1, result2.N.A)
	assert.Equal(t, 2, result2.N.B)
	assert.Equal(t, 0, result2.N.N2.C)
	assert.Equal(t, 0, result2.N.N2.D)
}

func TestBind_MultipleValidate(t *testing.T) {
	type Test1 struct {
		A int `query:"a" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=9")
	var result Test1
	err := DefaultBinder().BindAndValidate(req.Req, &result, nil)
	assert.NotNil(t, err)
}

func TestBind_Query(t *testing.T) {
	type Req struct {
		Q1 int `query:"q1"`
		Q2 int
		Q3 string
		Q4 string
		Q5 []int
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?q1=1&Q2=2&Q3=3&Q4=4&Q5=51&Q5=52")

	var result Req

	err := DefaultBinder().BindQuery(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, 1, result.Q1)
	assert.Equal(t, 2, result.Q2)
	assert.Equal(t, "3", result.Q3)
	assert.Equal(t, "4", result.Q4)
	assert.Equal(t, 51, result.Q5[0])
	assert.Equal(t, 52, result.Q5[1])
}

func TestBind_LooseZeroMode(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.LooseZeroMode = false
	binder := NewBinder(bindConfig)
	type Req struct {
		ID int `query:"id"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=")

	var result Req

	err := binder.Bind(req.Req, &result, nil)
	assert.NotNil(t, err)
	assert.Equal(t, 0, result.ID)

	bindConfig.LooseZeroMode = true
	binder = NewBinder(bindConfig)
	var result2 Req

	err = binder.Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, result2.ID)
}

func TestBind_NonStruct(t *testing.T) {
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=1&id=2")
	var id any
	err := DefaultBinder().Bind(req.Req, &id, nil)
	assert.Nil(t, err)

	err = DefaultBinder().BindAndValidate(req.Req, &id, nil)
	assert.Nil(t, err)
}

func TestBind_BindTag(t *testing.T) {
	type Req struct {
		Query  string
		Header string
		Path   string
		Form   string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?Query=query").
		SetHeader("Header", "header").
		SetPostArg("Form", "form")
	var params param.Params
	params = append(params, param.Param{
		Key:   "Path",
		Value: "path",
	})
	result := Req{}

	// 测试 query 标签
	err := DefaultBinder().BindQuery(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, "query", result.Query)

	// 测试 header 标签
	err = DefaultBinder().BindHeader(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, "header", result.Header)

	// 测试 path 标签
	err = DefaultBinder().BindPath(req.Req, &result, params)
	assert.Nil(t, err)
	assert.Equal(t, "path", result.Path)

	// 测试 form 标签
	err = DefaultBinder().BindForm(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, "form", result.Form)

	// 测试 json 标签
	req = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte("{\n    \"Query\": \"query\",\n    \"Path\": \"path\",\n    \"Header\": \"header\",\n    \"Form\": \"form\"\n}"))
	result = Req{}
	err = DefaultBinder().BindJSON(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, "path", result.Path)
	assert.Equal(t, "query", result.Query)
	assert.Equal(t, "header", result.Header)
	assert.Equal(t, "form", result.Form)
}

func TestDefaultBinder_BindAndValidate(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// 测试绑定并验证
	var result Req
	err := BindAndValidate(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 12, result.ID)

	// 测试绑定
	result = Req{}
	err = Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 12, result.ID)

	// 测试验证
	req = newMockRequest().
		SetRequestURI("http://foobar.com?id=9")
	result = Req{}
	err = Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	err = Validate(result)
	assert.NotNil(t, err)
	assert.Equal(t, 9, result.ID)
}

func TestBind_FastPath(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// 测试绑定并验证
	var result Req
	err := BindAndValidate(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 12, result.ID)
	// 执行多次以测试缓存
	for i := 0; i < 10; i++ {
		result = Req{}
		err = BindAndValidate(req.Req, &result, nil)
		assert.Nil(t, err)
		assert.Equal(t, 12, result.ID)
	}
}

func TestBind_NonPointer(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// 测试绑定并验证
	var result Req
	err := BindAndValidate(req.Req, result, nil)
	assert.NotNil(t, err)

	err = Bind(req.Req, result, nil)
	assert.NotNil(t, err)
}

func TestBind_PreBind(t *testing.T) {
	type Req struct {
		Query  string
		Header string
		Path   string
		Form   string
	}
	// test json tag
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte("\n    \"Query\": \"query\",\n    \"Path\": \"path\",\n    \"Header\": \"header\",\n    \"Form\": \"form\"\n}"))
	result := Req{}
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.NotNil(t, err)
	err = DefaultBinder().BindAndValidate(req.Req, &result, nil)
	assert.NotNil(t, err)
}

func TestDefaultBinder_BindProtobuf(t *testing.T) {
	data := testdata.WindReq{Name: "wind"}
	body, err := proto.Marshal(&data)
	assert.Nil(t, err)
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetProtobufContentType().
		SetBody(body)

	result := testdata.WindReq{}
	err = DefaultBinder().BindAndValidate(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "wind", result.Name)

	result = testdata.WindReq{}
	err = DefaultBinder().BindProtobuf(req.Req, &result)
	assert.Nil(t, err)
	assert.Equal(t, "wind", result.Name)
}

func TestBind_PointerStruct(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}
	query := make(url.Values)
	query.Add("B1", "{\n    \"F1\": \"111\"\n}")

	var result Bar
	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s", query.Encode()))

	err := binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "111", (**result.B1).F1)

	result = Bar{}
	req = newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s&F1=222", query.Encode()))
	err = binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "222", (**result.B1).F1)
}

func TestBind_StructRequired(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}

	var result Bar
	req := newMockRequest().
		SetRequestURI("http://foobar.com")

	err := binder.Bind(req.Req, &result, nil)
	assert.NotNil(t, err)

	type Bar2 struct {
		B1 **Foo `query:"B1"`
	}
	var result2 Bar2
	req = newMockRequest().
		SetRequestURI("http://foobar.com")

	err = binder.Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
}

func TestBind_StructErrorToWarn(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}

	var result Bar
	req := newMockRequest().
		SetRequestURI("http://foobar.com?B1=111&F1=222")

	err := binder.Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "222", (**result.B1).F1)

	type Bar2 struct {
		B1 Foo `query:"B1,required"`
	}
	var result2 Bar2
	err = binder.Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
	assert.Equal(t, "222", result2.B1.F1)
}

func TestBind_DisallowUnknownFieldsConfig(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.EnableDecoderDisallowUnknownFields = true
	binder := NewBinder(bindConfig)
	type FooStructUseNumber struct {
		Foo any `json:"foo"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte(`{"foo": 123,"bar": "456"}`))
	var result FooStructUseNumber

	err := binder.BindJSON(req.Req, &result)
	assert.NotNil(t, err)
}

func TestBind_UseNumberConfig(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.EnableDecoderUseNumber = true
	binder := NewBinder(bindConfig)
	type FooStructUseNumber struct {
		Foo any `json:"foo"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte(`{"foo": 123}`))
	var result FooStructUseNumber

	err := binder.BindJSON(req.Req, &result)
	assert.Nil(t, err)
	v, err := result.Foo.(json.Number).Int64()
	assert.Nil(t, err)
	assert.Equal(t, int64(123), v)
}

func TestBind_InterfaceType(t *testing.T) {
	type Bar struct {
		B1 any `query:"B1"`
	}
	var result Bar
	query := make(url.Values)
	query.Add("B1", `{"B1":"111"}`)
	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s", query.Encode()))
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)

	type Bar2 struct {
		B2 *any `query:"B1"`
	}

	var result2 Bar2
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	assert.Nil(t, err)
}

func TestBind_HeaderNormalize(t *testing.T) {
	type Req struct {
		Header string `header:"h"`
	}
	var result Req

	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("h", "header")
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "header", result.Header)

	req = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("H", "header")
	err = DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, "header", result.Header)

	type Req2 struct {
		Header string `header:"H"`
	}
	var result2 Req2

	req2 := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("h", "header")

	err2 := DefaultBinder().Bind(req2.Req, &result2, nil)
	assert.Nil(t, err2)

	req2 = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("H", "header")

	err2 = DefaultBinder().Bind(req2.Req, &result2, nil)
	assert.Nil(t, err2)

	type Req3 struct {
		Header string `header:"h"`
	}

	// 不经规范化，则标头键名和标签键名需要一致
	req3 := newMockRequest().
		SetRequestURI("http://foobar.com")
	req3.Req.Header.DisableNormalizing()
	req3.SetHeaders("h", "header")
	var result3 Req3
	err3 := DefaultBinder().Bind(req3.Req, &result3, nil)
	assert.Nil(t, err3)
	assert.Equal(t, "header", result3.Header)

	req3 = newMockRequest().
		SetRequestURI("http://foobar.com")
	req3.Req.Header.DisableNormalizing()
	req3.SetHeaders("H", "header")
	result3 = Req3{}
	err3 = DefaultBinder().Bind(req3.Req, &result3, nil)
	assert.Nil(t, err3)
	assert.Equal(t, "", result3.Header)
}

type ValidateError struct {
	ErrType, FailField, Msg string
}

// Error 实现错误接口。
func (e *ValidateError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": 表达式路径=" + e.FailField + ", 原因=" + e.Msg
	}
	return e.ErrType + ": 表达式路径=" + e.FailField + ", 原因=参数无效"
}

func TestValidateConfig_SetValidatorErrorFactory(t *testing.T) {
	type TestBind struct {
		A string `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?b=20")

	var req TestBind
	err := Bind(r, &req, nil)
	assert.NotNil(t, err)

	type TestValidate struct {
		B int `query:"b" vd:"$>100"`
	}

	var reqValidate TestValidate
	err = Bind(r, &reqValidate, nil)
	assert.Nil(t, err)

	CustomValidateErrFunc := func(failField, msg string) error {
		err := ValidateError{
			ErrType:   "验证错误",
			FailField: "[错误字段]: " + failField,
			Msg:       "[错误消息]: " + msg,
		}

		return &err
	}
	validateConfig := NewValidateConfig()
	validateConfig.SetValidatorErrorFactory(CustomValidateErrFunc)
	vd := NewValidator(validateConfig)
	err = vd.ValidateStruct(&reqValidate)
	assert.NotNil(t, err)
	assert.Equal(t, "验证错误: 表达式路径=[错误字段]: B, 原因=[错误消息]: ", err.Error())
}

// Test_Issue964 used to the cover issue for time.Time
func Test_Issue964(t *testing.T) {
	type CreateReq struct {
		StartAt *time.Time `json:"startAt"`
	}
	r := newMockRequest().SetBody([]byte("{\n  \"startAt\": \"2006-01-02T15:04:05+07:00\"\n}")).SetJSONContentType()
	var req CreateReq
	err := DefaultBinder().BindAndValidate(r.Req, &req, nil)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "2006-01-02 15:04:05 +0700 +0700", req.StartAt.String())
	r = newMockRequest()
	req = CreateReq{}
	err = DefaultBinder().BindAndValidate(r.Req, &req, nil)
	if err != nil {
		t.Error(err)
	}
	if req.StartAt != nil {
		t.Error("expected nil")
	}
}

func Benchmark_Binding(b *testing.B) {
	type Req struct {
		Version string `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"h"`
		Form    string `form:"f"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodedContentType()

	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result Req
		err := DefaultBinder().Bind(req.Req, &result, params)
		if err != nil {
			b.Error(err)
		}
		if result.ID != 12 {
			b.Error("Id failed")
		}
		if result.Form != "form" {
			b.Error("form failed")
		}
		if result.Header != "header" {
			b.Error("header failed")
		}
		if result.Version != "1" {
			b.Error("path failed")
		}
	}
}
