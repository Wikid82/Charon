package services

import (
	"testing"
	"time"
)

func BenchmarkFormatDuration(b *testing.B) {
	d := 3665 * time.Second
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatDuration(d)
	}
}

func BenchmarkExtractPort(b *testing.B) {
	url := "http://example.com:8080"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractPort(url)
	}
}
