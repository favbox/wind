package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURIPathNormalize(t *testing.T) {
	t.Parallel()

	var u URI

	testURIPathNormalize(t, &u, `a`, `/a`)
	testURIPathNormalize(t, &u, "/../../../../../foo", "/foo")
	testURIPathNormalize(t, &u, "/..\\..\\..\\..\\..\\", "/")
	testURIPathNormalize(t, &u, "/..%5c..%5cfoo", "/foo")
}

func TestGetScheme(t *testing.T) {
	scheme, path := getScheme([]byte("E:\\file.go"))
	assert.DeepEqual(t, "", string(scheme))
	assert.DeepEqual(t, "E:\\file.go", string(path))

	scheme, path = getScheme([]byte("E:\\"))
	assert.DeepEqual(t, "", string(scheme))
	assert.DeepEqual(t, "E:\\", string(path))

	scheme, path = getScheme([]byte("https://foo.com"))
	assert.DeepEqual(t, "https", string(scheme))
	assert.DeepEqual(t, "//foo.com", string(path))

	scheme, path = getScheme([]byte("://"))
	assert.DeepEqual(t, "", string(scheme))
	assert.DeepEqual(t, "", string(path))

	scheme, path = getScheme([]byte("ws://127.0.0.1"))
	assert.DeepEqual(t, "ws", string(scheme))
	assert.DeepEqual(t, "//127.0.0.1", string(path))
}
