package bytesconv

import "testing"

func TestWriteHexInt(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		n int
	}{
		{"0", 0},
		{"1", 1},
		{"123", 0x123},
		{"7fffffffffffffff", 0x7fffffffffffffff},
	} {
		testWriteHexInt(t, v.n, v.s)
	}
}

func TestReadHexInt(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		n int
	}{
		//errTooLargeHexNum "too large hex number"
		//{"0123456789abcdef", -1},
		{"0", 0},
		{"fF", 0xff},
		{"00abc", 0xabc},
		{"7fffffff", 0x7fffffff},
		{"000", 0},
		{"1234ZZZ", 0x1234},
		{"7ffffffffffffff", 0x7ffffffffffffff},
	} {
		testReadHexInt(t, v.s, v.n)
	}
}
