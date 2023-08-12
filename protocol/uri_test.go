package protocol

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/favbox/wind/common/utils"
	"github.com/stretchr/testify/assert"
)

func TestUtilsNormalizeHeaderKey(t *testing.T) {
	contentTypeStr := []byte("Content-Type")
	lowerContentTypeStr := []byte("content-type")
	mixedContentTypeStr := []byte("conTENT-tYpE")
	mixedContentTypeStrWithoutNormalizing := []byte("Content-type")
	utils.NormalizeHeaderKey(contentTypeStr, false)
	utils.NormalizeHeaderKey(lowerContentTypeStr, false)
	utils.NormalizeHeaderKey(mixedContentTypeStr, false)
	utils.NormalizeHeaderKey(mixedContentTypeStrWithoutNormalizing, false)

	assert.Equal(t, "Content-Type", string(contentTypeStr))
	assert.Equal(t, "Content-Type", string(lowerContentTypeStr))
	assert.Equal(t, "Content-Type", string(mixedContentTypeStr))
	assert.Equal(t, "Content-Type", string(mixedContentTypeStrWithoutNormalizing))
}

func TestURIPathNormalize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	t.Parallel()

	var u URI

	// 双斜线
	testURIPathNormalize(t, &u, "/aa//bb", "/aa/bb")

	// 三斜线
	testURIPathNormalize(t, &u, "/x///y/", "/x/y/")

	// 很多斜线
	testURIPathNormalize(t, &u, "/abc//de///fg////", "/abc/de/fg/")

	// 编码后的斜线
	testURIPathNormalize(t, &u, "/xxxx%2fyyy%2f%2F%2F", "/xxxx/yyy/")

	// ..
	testURIPathNormalize(t, &u, "/aaa/..", "/")

	// ../ 结尾
	testURIPathNormalize(t, &u, "/xxx/yyy/../", "/xxx/")

	// 连续多个 ..
	testURIPathNormalize(t, &u, "/aaa/bbb/ccc/../../ddd", "/aaa/ddd")

	// 断续多个 ..
	testURIPathNormalize(t, &u, "/a/b/../c/d/../e/..", "/a/c/")

	// 很多级 ..
	testURIPathNormalize(t, &u, "/aaa/../../../../xxx", "/xxx")
	testURIPathNormalize(t, &u, "/../../../../../..", "/")
	testURIPathNormalize(t, &u, "/../../../../../../", "/")

	// 编码后的 ..
	testURIPathNormalize(t, &u, "/aaa%2Fbbb%2F%2E.%2Fxxx", "/aaa/xxx")

	// 带 .. 和 //
	testURIPathNormalize(t, &u, "/aaa////..//b", "/b")

	// 不作为路径的假 ..
	testURIPathNormalize(t, &u, "/aaa/..bbb/ccc/..", "/aaa/..bbb/")

	// 单个 .
	testURIPathNormalize(t, &u, "/a/./b/././c/./d.html", "/a/b/c/d.html")
	testURIPathNormalize(t, &u, "./foo/", "/foo/")
	testURIPathNormalize(t, &u, "./../.././../../aaa/bbb/../../../././../", "/")
	testURIPathNormalize(t, &u, "./a/./.././../b/./foo.html", "/b/foo.html")
}

func TestParsePathWindows(t *testing.T) {
	t.Parallel()

	testParsePathWindows(t, "/../../../../../foo", "/foo")
	testParsePathWindows(t, "/..\\..\\..\\..\\..\\foo", "/foo")
	testParsePathWindows(t, "/..%5c..%5cfoo", "/foo")
}

func testURIPathNormalize(t *testing.T, u *URI, requestURI, expectedPath string) {
	u.Parse(nil, []byte(requestURI))
	if string(u.Path()) != expectedPath {
		t.Fatalf("不期待的路径 %q。期待 %q。requestURI=%q", u.Path(), expectedPath, requestURI)
	}
}

func testParsePathWindows(t *testing.T, requestURI, expectedPath string) {
	var u URI
	u.Parse(nil, []byte(requestURI))
	if filepath.Separator == '\\' && string(u.Path()) != expectedPath {
		t.Fatalf("不期待的路径 %q。期待 %q。requestURI=%q", u.Path(), expectedPath, requestURI)
	}
}

func TestDelArgs(t *testing.T) {
	var args Args
	args.Set("foo", "bar")
	assert.Equal(t, string(args.Peek("foo")), "bar")
	args.Del("foo")
	assert.Equal(t, string(args.Peek("foo")), "")

	args.Set("foo2", "bar2")
	assert.Equal(t, string(args.Peek("foo2")), "bar2")
	args.DelBytes([]byte("foo2"))
	assert.Equal(t, string(args.Peek("foo2")), "")
}

func TestURIFullURI(t *testing.T) {
	t.Parallel()

	var args Args

	// 空白协议、路径和哈希
	testURIFullURI(t, "", "foobar.com", "", "", &args, "http://foobar.com/")

	// 空白协议和哈希
	testURIFullURI(t, "", "aaa.com", "/foo/bar", "", &args, "http://aaa.com/foo/bar")
	// 空白哈希
	testURIFullURI(t, "fTP", "XXx.com", "/foo", "", &args, "ftp://xxx.com/foo")

	// 空白参数
	testURIFullURI(t, "https", "xx.com", "/", "aaa", &args, "https://xx.com/#aaa")

	// 费控参数和非ASCII路径
	args.Set("foo", "bar")
	args.Set("xxx", "йух")
	testURIFullURI(t, "", "xxx.com", "/тест123", "2er", &args, "http://xxx.com/%D1%82%D0%B5%D1%81%D1%82123?foo=bar&xxx=%D0%B9%D1%83%D1%85#2er")

	// 测试空白参数和非空查询字符串
	var u URI
	u.Parse([]byte("google.com"), []byte("/foo?bar=baz&baraz#qqqq"))
	uri := u.FullURI()
	expectedURI := "http://google.com/foo?bar=baz&baraz#qqqq"
	assert.Equal(t, expectedURI, string(uri))
}

func testURIFullURI(t *testing.T, scheme, host, path, hash string, args *Args, expectedURI string) {
	var u URI

	u.SetScheme(scheme)
	u.SetHost(host)
	u.SetPath(path)
	u.SetHash(hash)
	args.CopyTo(u.QueryArgs())

	uri := u.FullURI()
	assert.Equal(t, expectedURI, string(uri))
}

func TestParseHostWithStr(t *testing.T) {
	expectUsername := "username"
	expectPassword := "password"

	testParseHostWithStr(t, "username", "", "")
	testParseHostWithStr(t, "username@", expectUsername, "")
	testParseHostWithStr(t, "username:password@", expectUsername, expectPassword)
	testParseHostWithStr(t, ":password@", "", expectPassword)
	testParseHostWithStr(t, ":password", "", "")
}

func testParseHostWithStr(t *testing.T, host, expectUsername, expectPassword string) {
	var u URI
	u.Parse([]byte(host), nil)
	assert.Equal(t, expectUsername, string(u.Username()))
	assert.Equal(t, expectPassword, string(u.Password()))
}

func TestURIPathEscape(t *testing.T) {
	t.Parallel()

	testURIPathEscape(t, "/foo/bar", "/foo/bar")
	testURIPathEscape(t, "/f_o-o=b:ar,b.c&q", "/f_o-o=b:ar,b.c&q")
	testURIPathEscape(t, "/aa?bb.тест~qq", "/aa%3Fbb.%D1%82%D0%B5%D1%81%D1%82~qq")
}

func testURIPathEscape(t *testing.T, path, expectedRequestURI string) {
	var u URI
	u.SetPath(path)
	requestURI := u.RequestURI()
	assert.Equal(t, expectedRequestURI, string(requestURI))
}

func TestURIUpdate(t *testing.T) {
	t.Parallel()

	// 完整替换
	testURIUpdate(t, "http://foo.bar/baz?aaa=22#aaa", "https://aaa.com/bb", "https://aaa.com/bb")
	// 不替换
	testURIUpdate(t, "http://aaa.com/aaa.html?234=234#add", "", "http://aaa.com/aaa.html?234=234#add")

	// 替换 RequestURI
	testURIUpdate(t, "ftp://aaa/xxx/yyy?aaa=bb#aa", "/boo/bar?xx", "ftp://aaa/boo/bar?xx")

	// 替换相对URI
	testURIUpdate(t, "http://foo.bar/baz/xxx.html?aaa=22#aaa", "bb.html?xx=12#pp", "http://foo.bar/baz/bb.html?xx=12#pp")
	testURIUpdate(t, "http://xx/a/b/c/d", "../qwe/p?zx=34", "http://xx/a/b/qwe/p?zx=34")
	testURIUpdate(t, "https://qqq/aaa.html?foo=bar", "?baz=434&aaa#xcv", "https://qqq/aaa.html?baz=434&aaa#xcv")
	testURIUpdate(t, "http://foo.bar/baz", "~a/%20b=c,тест?йцу=ке", "http://foo.bar/~a/%20b=c,%D1%82%D0%B5%D1%81%D1%82?йцу=ке")
	testURIUpdate(t, "http://foo.bar/baz", "/qwe#fragment", "http://foo.bar/qwe#fragment")
	testURIUpdate(t, "http://foobar/baz/xxx", "aaa.html#bb?cc=dd&ee=dfd", "http://foobar/baz/aaa.html#bb?cc=dd&ee=dfd")

	// 替换hash
	testURIUpdate(t, "http://foo.bar/baz#aaa", "#fragment", "http://foo.bar/baz#fragment")

	// 保留协议替换
	testURIUpdate(t, "https://foo.bar/baz", "//aaa.bbb/cc?dd", "https://aaa.bbb/cc?dd")
	testURIUpdate(t, "http://foo.bar/baz", "//aaa.bbb/cc?dd", "http://aaa.bbb/cc?dd")
}

func testURIUpdate(t *testing.T, base, update, result string) {
	var u URI
	u.Parse(nil, []byte(base))
	u.Update(update)
	s := u.String()
	assert.Equal(t, result, s)
}

func TestURILastPathSegment(t *testing.T) {
	t.Parallel()

	testURILastPathSegment(t, "", "")
	testURILastPathSegment(t, "/", "")
	testURILastPathSegment(t, "/foo/bar/", "")
	testURILastPathSegment(t, "/foobar.js", "foobar.js")
	testURILastPathSegment(t, "/foo/bar/baz.html", "baz.html")
}

func testURILastPathSegment(t *testing.T, path, expectedSegment string) {
	var u URI
	u.SetPath(path)
	segment := u.LastPathSegment()
	assert.Equal(t, expectedSegment, string(segment))
}

func TestURICopyTo(t *testing.T) {
	t.Parallel()

	var u URI
	var copyU URI
	u.CopyTo(&copyU)
	if !reflect.DeepEqual(&u, &copyU) { //nolint:govet
		t.Fatalf("URICopyTo fail, u: \n%+v\ncopyu: \n%+v\n", &u, &copyU) //nolint:govet
	}

	u.UpdateBytes([]byte("https://google.com/foo?bar=baz&baraz#qqqq"))
	u.CopyTo(&copyU)
	if !reflect.DeepEqual(&u, &copyU) { //nolint:govet
		t.Fatalf("URICopyTo fail, u: \n%+v\ncopyu: \n%+v\n", &u, &copyU) //nolint:govet
	}
}

func TestURICopyToQueryArgs(t *testing.T) {
	t.Parallel()

	var u URI
	a := u.QueryArgs()
	a.Set("foo", "bar")

	var u1 URI
	u.CopyTo(&u1)
	a1 := u1.QueryArgs()

	if string(a1.Peek("foo")) != "bar" {
		t.Fatalf("unexpected query args value %q. Expecting %q", a1.Peek("foo"), "bar")
	}
	assert.Equal(t, "bar", string(a1.Peek("foo")))
}

func TestArgsKV_Get(t *testing.T) {
	var kv argsKV
	expectKey := "key"
	expectValue := "value"
	kv.key = []byte(expectKey)
	kv.value = []byte(expectValue)
	key := string(kv.GetKey())
	value := string(kv.GetValue())
	assert.Equal(t, expectKey, key)
	assert.Equal(t, expectValue, value)
}

func TestURI_PathOriginal(t *testing.T) {
	var u URI
	expectPath := "/path"
	u.Parse(nil, []byte(expectPath))
	uri := string(u.PathOriginal())
	assert.Equal(t, expectPath, uri)
}

func TestURI_Host(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectHost1 := "host1"
	expectHost2 := "host2"

	u.SetHost(expectHost1)
	host1 := string(u.Host())
	assert.Equal(t, expectHost1, host1)
	u.SetHost(expectHost2)
	host2 := string(u.Host())
	assert.Equal(t, expectHost2, host2)

	u.SetHostBytes([]byte(host1))
	assert.Equal(t, expectHost1, host1)
	u.SetHostBytes([]byte(host2))
	assert.Equal(t, expectHost2, host2)
}

func TestURI_Scheme(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectScheme1 := "scheme1"
	expectScheme2 := "scheme2"

	u.SetScheme(expectScheme1)
	scheme1 := string(u.Scheme())
	assert.Equal(t, expectScheme1, scheme1)
	u.SetScheme(expectScheme2)
	scheme2 := string(u.Scheme())
	assert.Equal(t, expectScheme2, scheme2)

	u.SetSchemeBytes([]byte(scheme1))
	assert.Equal(t, expectScheme1, scheme1)
	u.SetSchemeBytes([]byte(scheme2))
	assert.Equal(t, expectScheme2, scheme2)
}

func TestURI_Path(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectPath1 := "/"
	expectPath2 := "/path1"
	expectPath3 := "/path3"

	// When Path is not set, Path defaults to "/"
	path1 := string(u.Path())
	assert.Equal(t, expectPath1, path1)

	u.SetPath(expectPath2)
	path2 := string(u.Path())
	assert.Equal(t, expectPath2, path2)
	u.SetPath(expectPath3)
	path3 := string(u.Path())
	assert.Equal(t, expectPath3, path3)

	u.SetPathBytes([]byte(path2))
	assert.Equal(t, expectPath2, path2)
	u.SetPathBytes([]byte(path3))
	assert.Equal(t, expectPath3, path3)
}

func TestURI_QueryString(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectQueryString1 := "key1=value1&key2=value2"
	expectQueryString2 := "key3=value3&key4=value4"

	u.SetQueryString(expectQueryString1)
	queryString1 := string(u.QueryString())
	assert.Equal(t, expectQueryString1, queryString1)
	u.SetQueryString(expectQueryString2)
	queryString2 := string(u.QueryString())
	assert.Equal(t, expectQueryString2, queryString2)
}

func TestURI_Hash(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectHash1 := "hash1"
	expectHash2 := "hash2"

	u.SetHash(expectHash1)
	hash1 := string(u.Hash())
	assert.Equal(t, expectHash1, hash1)
	u.SetHash(expectHash2)
	hash2 := string(u.Hash())
	assert.Equal(t, expectHash2, hash2)
}

//func TestURI_Username(t *testing.T) {
//	var req Request
//	req.SetRequestURI("http://user:pass@example.com/foo/bar")
//	u := req.URI()
//	user1 := string(u.Username())
//	req.Header.SetRequestURIBytes([]byte("/foo/bar"))
//	u = req.URI()
//	user2 := string(u.Username())
//	assert.Equal(t, user1, user2)
//
//	expectUser3 := "user3"
//	expectUser4 := "user4"
//
//	u.SetUsername(expectUser3)
//	user3 := string(u.Username())
//	assert.Equal(t, expectUser3, user3)
//	u.SetUsername(expectUser4)
//	user4 := string(u.Username())
//	assert.Equal(t, expectUser4, user4)
//
//	u.SetUsernameBytes([]byte(user3))
//	assert.Equal(t, expectUser3, user3)
//	u.SetUsernameBytes([]byte(user4))
//	assert.Equal(t, expectUser4, user4)
//}

func TestURI_Password(t *testing.T) {
	u := AcquireURI()
	defer ReleaseURI(u)

	expectPassword1 := "password1"
	expectPassword2 := "password2"

	u.SetPassword(expectPassword1)
	password1 := string(u.Password())
	assert.Equal(t, expectPassword1, password1)
	u.SetPassword(expectPassword2)
	password2 := string(u.Password())
	assert.Equal(t, expectPassword2, password2)

	u.SetPasswordBytes([]byte(password1))
	assert.Equal(t, expectPassword1, password1)
	u.SetPasswordBytes([]byte(password2))
	assert.Equal(t, expectPassword2, password2)
}
