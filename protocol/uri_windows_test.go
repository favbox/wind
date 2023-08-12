package protocol

func TestURIPathNormalize(t *testing.T) {
	t.Parallel()

	var u URI

	testURIPathNormalize(t, &u, `a`, `/a`)
	testURIPathNormalize(t, &u, "/../../../../../foo", "/foo")
	testURIPathNormalize(t, &u, "/..\\..\\..\\..\\..\\", "/")
	testURIPathNormalize(t, &u, "/..%5c..%5cfoo", "/foo")
}
