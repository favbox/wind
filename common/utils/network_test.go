package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTLSRecordHeaderLooksLikeHTTP(t *testing.T) {
	HeaderValueAndExpectedResult := [][]interface{}{
		{[5]byte{'G', 'E', 'T', ' ', '/'}, true},
		{[5]byte{'H', 'E', 'A', 'D', ' '}, true},
		{[5]byte{'P', 'O', 'S', 'T', ' '}, true},
		{[5]byte{'P', 'U', 'T', ' ', '/'}, true},
		{[5]byte{'O', 'P', 'T', 'I', 'O'}, true},
		{[5]byte{'G', 'E', 'T', '/', ' '}, false},
		{[5]byte{' ', 'H', 'E', 'A', 'D'}, false},
		{[5]byte{' ', 'P', 'O', 'S', 'T'}, false},
		{[5]byte{'P', 'U', 'T', '/', ' '}, false},
		{[5]byte{'H', 'E', 'R', 'T', 'Z'}, false},
	}

	for _, testCase := range HeaderValueAndExpectedResult {
		value, expectedResult := testCase[0].([5]byte), testCase[1].(bool)
		assert.Equal(t, expectedResult, TLSRecordHeaderLooksLikeHTTP(value))
	}
}

func TestLocalIP(t *testing.T) {
	// 为了测试目的，模拟本地IP变量。
	localIP = "192.168.0.1"

	// 确保LocalIP()返回预期的本地IP。
	expectedIP := "192.168.0.1"
	if got := LocalIP(); got != expectedIP {
		assert.Equal(t, got, expectedIP)
	}
}
