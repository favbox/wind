package utils

import (
	"testing"

	"github.com/favbox/wind/common/mock"
	"github.com/stretchr/testify/assert"
)

func TestChunkParseChunkSizeGetCorrect(t *testing.T) {
	// 迭代 hexMap，并判断 dec 和 ParseChunkSize 之间的差异
	hexMap := map[int]string{0: "0", 10: "a", 100: "64", 1000: "3e8"}
	for dec, hex := range hexMap {
		chunkSizeBody := hex + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.Equal(t, nil, err)
		assert.Equal(t, chunkSize, dec)
	}
}

func TestChunkParseChunkSizeCorrectWhiteSpace(t *testing.T) {
	// 测试空格
	whiteSpace := ""
	for i := 0; i < 10; i++ {
		whiteSpace += " "
		chunkSizeBody := "0" + whiteSpace + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.Equal(t, nil, err)
		assert.Equal(t, 0, chunkSize)
	}
}

func TestChunkParseChunkSizeNonCRLF(t *testing.T) {
	// 测试非 "\r\n" 结尾
	chunkSizeBody := "0" + "\n\r"
	zr := mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err := ParseChunkSize(zr)
	assert.Equal(t, true, err != nil)
	assert.Equal(t, -1, chunkSize)
}

func TestChunkReadTrueCRLF(t *testing.T) {
	CRLF := "\r\n"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.Equal(t, nil, err)
}

func TestChunkReadFalseCRLF(t *testing.T) {
	CRLF := "\n\r"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.Equal(t, errBrokenChunk, err)
}
