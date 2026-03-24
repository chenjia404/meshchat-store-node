package ratelimit

import (
	"sync"
	"time"
)

type senderWindow struct {
	WindowStart time.Time
	Count       int
}

type Limiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]senderWindow
	nowFunc func() time.Time
}

func New(limit int) *Limiter {
	return &Limiter{
		limit:   limit,
		windows: make(map[string]senderWindow),
		nowFunc: time.Now,
	}
}

func (l *Limiter) SetNowFunc(nowFunc func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if nowFunc != nil {
		l.nowFunc = nowFunc
	}
}

func (l *Limiter) Allow(senderID string) bool {
	if l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc().UTC()
	window := now.Truncate(time.Minute)
	state := l.windows[senderID]
	if state.WindowStart != window {
		state = senderWindow{WindowStart: window}
	}
	if state.Count >= l.limit {
		l.windows[senderID] = state
		return false
	}
	state.Count++
	l.windows[senderID] = state
	return true
}
