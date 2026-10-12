package bot

import (
	"sync"
	"time"
)

type Limiter struct {
	mu           sync.Mutex
	maxPerMinute int
	maxPerDay    int
	userHistory  map[int][]time.Time
	dailyStart   time.Time
	dailyCount   int
}

func NewLimiter(ratePerMinute, dailyCap int) *Limiter {
	if ratePerMinute <= 0 {
		ratePerMinute = 5
	}
	if dailyCap <= 0 {
		dailyCap = 100
	}
	return &Limiter{
		maxPerMinute: ratePerMinute,
		maxPerDay:    dailyCap,
		userHistory:  make(map[int][]time.Time),
		dailyStart:   time.Now(),
	}
}

func (l *Limiter) Allow(userID int) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.dailyStart) >= 24*time.Hour {
		l.dailyStart = now
		l.dailyCount = 0
	}

	window := l.userHistory[userID]
	trimmed := window[:0]
	for _, ts := range window {
		if now.Sub(ts) <= time.Minute {
			trimmed = append(trimmed, ts)
		}
	}
	l.userHistory[userID] = trimmed

	if len(trimmed) >= l.maxPerMinute || l.dailyCount >= l.maxPerDay {
		return false
	}

	trimmed = append(trimmed, now)
	l.userHistory[userID] = trimmed
	l.dailyCount++
	return true
}
