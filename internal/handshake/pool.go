package handshake

import (
	"fmt"
	"sync"
)

const (
	PortStart = 7947
	PortEnd   = 8946
)

type PortPool struct {
	mu    sync.Mutex
	inUse map[int]bool
}

func NewPortPool() *PortPool {
	return &PortPool{
		inUse: make(map[int]bool),
	}
}

func (p *PortPool) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port := PortStart; port <= PortEnd; port++ {
		if !p.inUse[port] {
			p.inUse[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in the pool")
}

func (p *PortPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inUse, port)
}
