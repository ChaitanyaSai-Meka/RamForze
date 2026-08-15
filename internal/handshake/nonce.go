package handshake

import (
	"time"
	"sync"
)

type Noncestruct struct {
	master_id string
	timestamp string
}

type NonceMutex struct {
	mu sync.Mutex
	nonce map[Noncestruct] bool
}

func NewNonceMutex() *NonceMutex {
	return &NonceMutex{
		nonce: make(map[Noncestruct]bool),
	}
}

func RegisterNonce(s *NonceMutex, n *Noncestruct) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonce[*n] = true
}

func Checknonce(s *NonceMutex, n *Noncestruct) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	Cleanup(s)
	_, exists := s.nonce[*n]
	return exists
}

func Cleanup(s *NonceMutex){
	cutoff := time.Now().Add(-2*time.Minute)
	for n:= range s.nonce {
		t,_:= time.Parse(time.RFC3339, n.timestamp)
        if t.Before(cutoff) {
            delete(s.nonce, n)
        }
	}
}