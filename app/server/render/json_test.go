package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResetStdJSONMarshal(t *testing.T) {
	table := map[string]string{
		"testA": "hello",
		"B":     "world",
	}

	ResetStdJSONMarshal()
	jsonBytes, err := jsonMarshalFunc(table)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, string(jsonBytes), `"testA":"hello"`)
	assert.Contains(t, string(jsonBytes), `"B":"world"`)
}

func TestDefaultJSONMarshal(t *testing.T) {
	table := map[string]string{
		"testA": "hello",
		"B":     "world",
	}

	jsonBytes, err := jsonMarshalFunc(table)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, string(jsonBytes), `"testA":"hello"`)
	assert.Contains(t, string(jsonBytes), `"B":"world"`)
}
