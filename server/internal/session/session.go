package session

import (
	"sync"

	"github.com/ricardo/cli-game/internal/game"
)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*game.Engine
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*game.Engine),
	}
}

func (m *Manager) Get(playerID string) *game.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[playerID]
}

func (m *Manager) Set(playerID string, eng *game.Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[playerID] = eng
}

func (m *Manager) Remove(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, playerID)
}
