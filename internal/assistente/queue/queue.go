package queue

import "sync"

type PerKey struct {
	mu    sync.Mutex
	locks map[string]*entry
}

type entry struct {
	mu    sync.Mutex
	users int
}

func New() *PerKey { return &PerKey{locks: make(map[string]*entry)} }
func (q *PerKey) Do(key string, fn func()) {
	q.mu.Lock()
	lock := q.locks[key]
	if lock == nil {
		lock = &entry{}
		q.locks[key] = lock
	}
	lock.users++
	q.mu.Unlock()
	lock.mu.Lock()
	defer func() {
		lock.mu.Unlock()
		q.mu.Lock()
		lock.users--
		if lock.users == 0 {
			delete(q.locks, key)
		}
		q.mu.Unlock()
	}()
	fn()
}
