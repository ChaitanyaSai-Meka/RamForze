package handshake

import (
	"fmt"
	"sync"
)

type ConnectionState string

const (
	StateConnected    ConnectionState = "CONNECTED"
	StateDisconnected ConnectionState = "DISCONNECTED"
)

type MasterRecord struct {
	DedicatedPort int
	State         ConnectionState
}

type Registry struct {
	mu      sync.RWMutex
	masters map[string]*MasterRecord
	pool    *PortPool
}

func NewRegistry(pool *PortPool) *Registry {
	return &Registry{
		masters: make(map[string]*MasterRecord),
		pool:    pool,
	}
}

func (r *Registry) Register(masterID string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.masters[masterID] = &MasterRecord{
		DedicatedPort: port,
		State:         StateConnected,
	}
}

func (r *Registry) Disconnect(masterID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.masters[masterID]
	if !exists {
		return fmt.Errorf("master %s not found in registry", masterID)
	}

	if record.State == StateConnected {
		r.pool.Release(record.DedicatedPort)
		record.State = StateDisconnected
		record.DedicatedPort = 0
	}

	return nil
}

func (r *Registry) Get(masterID string) (*MasterRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.masters[masterID]
	if !exists {
		return nil, fmt.Errorf("master %s not found in registry", masterID)
	}

	return record, nil
}
