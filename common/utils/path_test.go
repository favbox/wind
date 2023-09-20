package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 函数 AddMissingPort 只添加丢失的端口，不考虑其他错误情况。
func TestPathAddMissingPort(t *testing.T) {
	ipList := []string{"127.0.0.1", "111.111.1.1", "[0:0:0:0:0:ffff:192.1.56.10]", "[0:0:0:0:0:ffff:c0a8:101]", "www.foobar.com"}
	for _, ip := range ipList {
		assert.Equal(t, ip+":443", AddMissingPort(ip, true))
		assert.Equal(t, ip+":80", AddMissingPort(ip, false))
		customizedPort := ":8080"
		assert.Equal(t, ip+customizedPort, AddMissingPort(ip+customizedPort, true))
		assert.Equal(t, ip+customizedPort, AddMissingPort(ip+customizedPort, false))
	}
}

func TestCleanPath(t *testing.T) {
	normalPath := "/Foo/Bar/go/src/github.com/wind/common/utils/path_test.go"
	expectedNormalPath := "/Foo/Bar/go/src/github.com/wind/common/utils/path_test.go"
	cleanNormalPath := CleanPath(normalPath)
	assert.Equal(t, expectedNormalPath, cleanNormalPath)

	singleDotPath := "/Foo/Bar/./././go/src"
	expectedSingleDotPath := "/Foo/Bar/go/src"
	cleanSingleDotPath := CleanPath(singleDotPath)
	assert.Equal(t, expectedSingleDotPath, cleanSingleDotPath)

	doubleDotPath := "../../.."
	expectedDoubleDotPath := "/"
	cleanDoublePotPath := CleanPath(doubleDotPath)
	assert.Equal(t, expectedDoubleDotPath, cleanDoublePotPath)

	// 多点可作文件名
	multiDotPath := "/../...."
	expectedMultiDotPath := "/...."
	cleanMultiDotPath := CleanPath(multiDotPath)
	assert.Equal(t, expectedMultiDotPath, cleanMultiDotPath)

	nullPath := ""
	expectedNullPath := "/"
	cleanNullPath := CleanPath(nullPath)
	assert.Equal(t, expectedNullPath, cleanNullPath)

	relativePath := "/Foo/Bar/../go/src/../../github.com/wind"
	expectedRelativePath := "/Foo/github.com/wind"
	cleanRelativePath := CleanPath(relativePath)
	assert.Equal(t, expectedRelativePath, cleanRelativePath)

	multiSlashPath := "///////Foo//Bar////go//src/github.com/wind//.."
	expectedMultiSlashPath := "/Foo/Bar/go/src/github.com"
	cleanMultiSlashPath := CleanPath(multiSlashPath)
	assert.Equal(t, expectedMultiSlashPath, cleanMultiSlashPath)

	inputPath := "/Foo/Bar/go/src/github.com/favbox/wind/common/utils/path_test.go/."
	expectedPath := "/Foo/Bar/go/src/github.com/favbox/wind/common/utils/path_test.go/"
	cleanedPath := CleanPath(inputPath)
	assert.Equal(t, expectedPath, cleanedPath)
}
