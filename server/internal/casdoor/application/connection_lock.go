package application

import (
	"context"
	"fmt"
	"sync"
)

type connectionLock struct {
	token chan struct{}
	refs  int
}

// ConnectionLocks 协调同一连接上的同步与管理命令。
type ConnectionLocks struct {
	mu    sync.Mutex
	locks map[uint64]*connectionLock
}

func NewConnectionLocks() *ConnectionLocks {
	return &ConnectionLocks{locks: make(map[uint64]*connectionLock)}
}

func (l *ConnectionLocks) Acquire(ctx context.Context, id uint64) (func(), error) {
	if l == nil || ctx == nil || id == 0 {
		return nil, fmt.Errorf("invalid connection lock request")
	}
	l.mu.Lock()
	lock := l.locks[id]
	if lock == nil {
		lock = &connectionLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}
		l.locks[id] = lock
	}
	lock.refs++
	l.mu.Unlock()
	drop := func() {
		l.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(l.locks, id)
		}
		l.mu.Unlock()
	}
	select {
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	case <-lock.token:
		var once sync.Once
		return func() { once.Do(func() { lock.token <- struct{}{}; drop() }) }, nil
	}
}
