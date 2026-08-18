package handshake

import (
	"sync"
	"time"
)


type RateLimitEntry struct {
	count int
	windowstart time.Time
}

type RateLimitMux struct {
	mu sync.Mutex
	rateLimit map[string]RateLimitEntry
}

func NewRateLimitMux() *RateLimitMux {
	return &RateLimitMux{
		rateLimit: make(map[string]RateLimitEntry),
	}
}

func RegisterAndCheckRateLimit(s *RateLimitMux , ip string)bool{
	s.mu.Lock()
	defer s.mu.Unlock()
	CleanupRateLimit(s)
	entry, exists := s.rateLimit[ip]
	if !exists{
		s.rateLimit[ip] = RateLimitEntry{
			count : 1,
			windowstart: time.Now(),
		}
		return true
	}
	if entry.count<5 {
		s.rateLimit[ip] = RateLimitEntry{
			count : entry.count+1,
			windowstart: entry.windowstart,
		}
		return true
	}
	return false
}

func CleanupRateLimit(s *RateLimitMux){
	cutoff:= time.Now().Add(-1*time.Minute)
	for r:= range s.rateLimit{
		if s.rateLimit[r].windowstart.Before(cutoff) {
			delete(s.rateLimit,r)
		}
	}
}
