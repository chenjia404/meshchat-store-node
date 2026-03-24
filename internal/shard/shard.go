package shard

import (
	"hash/fnv"
	"sync"
)

type Manager struct {
	locks []sync.Mutex
}

func New(size int) *Manager {
	if size <= 0 {
		size = 256
	}
	return &Manager{locks: make([]sync.Mutex, size)}
}

func (m *Manager) Lock(key string) func() {
	if len(m.locks) == 0 {
		return func() {}
	}
	idx := indexOf(key, len(m.locks))
	m.locks[idx].Lock()
	return func() {
		m.locks[idx].Unlock()
	}
}

func indexOf(key string, size int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(size))
}
