package main

import (
	. "testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanText(t *T) {
	m := map[string]string{
		"this is a test":             "this is a test",
		"<some stuff>":               "<some stuff>",
		"<http://gobin.io/whatever>": "http://gobin.io/whatever",
		"A: <http://example.com>, B: <https://example2.com>": "A: http://example.com, B: https://example2.com",
	}

	for in, expected := range m {
		assert.Equal(t, expected, cleanText(in))
	}
}
