package bytesconv

import (
	"testing"

	"golang.org/x/net/http/httpguts"
)

func BenchmarkValidHeaderFiledValueTable(b *testing.B) {
	// Test all characters
	allBytes := make([]string, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, string([]byte{byte(i)}))
	}

	for i := 0; i < b.N; i++ {
		for _, s := range allBytes {
			_ = httpguts.ValidHeaderFieldValue(s)
		}
	}
}

func BenchmarkValidHeaderFiledValueTableHertz(b *testing.B) {
	// Test all characters
	allBytes := make([]byte, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, byte(i))
	}

	for i := 0; i < b.N; i++ {
		for _, s := range allBytes {
			_ = func() bool {
				return ValidHeaderFieldValueTable[s] != 0
			}()
		}
	}
}
