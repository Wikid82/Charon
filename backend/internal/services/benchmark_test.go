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
	url := "https://discord.com/api/webhooks/123/abc:8080"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractPort(url)
	}
}
