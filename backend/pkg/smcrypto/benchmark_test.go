package smcrypto

import (
	"testing"
)

func BenchmarkSM3BodyHash(b *testing.B) {
	smallBody := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	mediumBody := []byte(`{"messages":[{"role":"user","content":"Hello, how are you today? I hope you're having a great day and staying safe. Let me know if there's anything I can help you with."}]}`)
	largeBody := make([]byte, 1024)
	for i := range largeBody {
		largeBody[i] = byte(i % 256)
	}

	b.Run("SmallBody_1KB", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(smallBody, 1024)
		}
	})

	b.Run("MediumBody_1KB", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(mediumBody, 1024)
		}
	})

	b.Run("LargeBody_1KB_Full", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(largeBody, 1024)
		}
	})

	b.Run("LargeBody_1KB_Truncated", func(b *testing.B) {
		truncated := make([]byte, 1024)
		copy(truncated, largeBody[:1024])
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(truncated, 1024)
		}
	})
}

func BenchmarkSM3MetaFingerprint(b *testing.B) {
	meta := []byte("req-001|POST|/v1/chat|192.168.1.1|agk-001|15")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM3Hex(meta)
	}
}

func BenchmarkSM3Compare(b *testing.B) {
	smallBody := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	largeBody := make([]byte, 1024)
	for i := range largeBody {
		largeBody[i] = byte(i % 256)
	}
	meta := []byte("req-001|POST|/v1/chat|192.168.1.1|agk-001|15")

	b.Run("Old_SmallBody_1KB", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(smallBody, 1024)
		}
	})

	b.Run("New_MetaFingerprint", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3Hex(meta)
		}
	})

	b.Run("Old_LargeBody_1KB", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			SM3HexTruncated(largeBody, 1024)
		}
	})

	b.Run("New_Vs_Old_LargeBody", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = SM3Hex(meta)
			_ = SM3HexTruncated(largeBody, 1024)
		}
	})

	b.Run("New_Only_Meta", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = SM3Hex(meta)
		}
	})
}
