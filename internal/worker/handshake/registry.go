package workerhandshake

import (
	"sync"
)

type ConnectionState string

const (
	StatusConnected ConnectionState = "CONNECTED"
	StatusDisconnected ConnectionState = "DISCONNECTED"
)

type MasterRecord struct {
	DedicatedPort int
	State ConnectionState
}

type Registry struct {
	mu sync.RWMutex
	masters map[string]*MasterRecord
	pool *PortPool
}

func NewRegistry(pool *PortPool) *Registry {
    return &Registry{
        masters: make(map[string]*MasterRecord),
        pool:    pool,
    }
}