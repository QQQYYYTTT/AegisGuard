package auth

import (
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkNonceMutexOriginal(b *testing.B) {
	ResetNonces()

	nonces := make([]string, 100)
	for i := range nonces {
		buf := make([]byte, 16)
		rand.Read(buf)
		nonces[i] = fmt.Sprintf("%x", buf)
	}

	type oldNonceMap struct {
		data map[string]bool
		mu   sync.Mutex
	}
	oldMap := &oldNonceMap{data: make(map[string]bool)}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			nonce := nonces[i%len(nonces)]
			oldMap.mu.Lock()
			if !oldMap.data[nonce] {
				oldMap.data[nonce] = true
			}
			oldMap.mu.Unlock()
			i++
		}
	})
}

func BenchmarkNonceRWMutexOptimized(b *testing.B) {
	ResetNonces()

	nonces := make([]string, 100)
	for i := range nonces {
		buf := make([]byte, 16)
		rand.Read(buf)
		nonces[i] = fmt.Sprintf("%x", buf)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			nonce := nonces[i%len(nonces)]
			nonceMu.RLock()
			_, exists := usedNonces[nonce]
			nonceMu.RUnlock()

			if !exists {
				nonceMu.Lock()
				usedNonces[nonce] = time.Now().Add(24 * time.Hour).Unix()
				nonceMu.Unlock()
			}
			i++
		}
	})
}

func BenchmarkNonceReadHeavy(b *testing.B) {
	ResetNonces()

	nonces := make([]string, 100)
	for i := range nonces {
		buf := make([]byte, 16)
		rand.Read(buf)
		nonces[i] = fmt.Sprintf("%x", buf)
	}

	for _, n := range nonces {
		nonceMu.Lock()
		usedNonces[n] = time.Now().Add(24 * time.Hour).Unix()
		nonceMu.Unlock()
	}

	b.Run("Mutex_Read", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			nonce := nonces[i%len(nonces)]
			nonceMu.Lock()
			_ = usedNonces[nonce]
			nonceMu.Unlock()
		}
	})

	b.Run("RWMutex_Read", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			nonce := nonces[i%len(nonces)]
			nonceMu.RLock()
			_ = usedNonces[nonce]
			nonceMu.RUnlock()
		}
	})
}

func BenchmarkNonceMixedReadWrite(b *testing.B) {
	nonces := make([]string, 100)
	for i := range nonces {
		buf := make([]byte, 16)
		rand.Read(buf)
		nonces[i] = fmt.Sprintf("%x", buf)
	}

	readRatio := 0.95

	oldMap := &struct {
		data map[string]bool
		mu   sync.Mutex
	}{data: make(map[string]bool)}

	b.Run("Mutex_Mixed", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			nonce := nonces[i%len(nonces)]
			if i%100 < int(readRatio*100) {
				oldMap.mu.Lock()
				_ = oldMap.data[nonce]
				oldMap.mu.Unlock()
			} else {
				oldMap.mu.Lock()
				oldMap.data[nonce] = true
				oldMap.mu.Unlock()
			}
		}
	})

	b.Run("RWMutex_Mixed", func(b *testing.B) {
		ResetNonces()
		for _, n := range nonces {
			nonceMu.Lock()
			usedNonces[n] = time.Now().Add(24 * time.Hour).Unix()
			nonceMu.Unlock()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			nonce := nonces[i%len(nonces)]
			if i%100 < int(readRatio*100) {
				nonceMu.RLock()
				_, _ = usedNonces[nonce]
				nonceMu.RUnlock()
			} else {
				nonceMu.Lock()
				usedNonces[nonce] = time.Now().Add(24 * time.Hour).Unix()
				nonceMu.Unlock()
			}
		}
	})
}
