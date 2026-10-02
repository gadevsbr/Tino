package queue

import "sync"

// PerKey dispatches tasks in FIFO order for each key. Different keys run in
// parallel, and idle queues are removed automatically.
type PerKey struct {
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	tasks   []func()
	running bool
}

func New() *PerKey { return &PerKey{entries: make(map[string]*entry)} }

// Enqueue returns immediately. A panic in one task cannot permanently block
// later messages for the same conversation.
func (q *PerKey) Enqueue(key string, fn func()) {
	q.mu.Lock()
	e := q.entries[key]
	if e == nil {
		e = &entry{}
		q.entries[key] = e
	}
	e.tasks = append(e.tasks, fn)
	start := !e.running
	e.running = true
	q.mu.Unlock()
	if start {
		go q.run(key, e)
	}
}

func (q *PerKey) run(key string, e *entry) {
	for {
		q.mu.Lock()
		if len(e.tasks) == 0 {
			delete(q.entries, key)
			q.mu.Unlock()
			return
		}
		fn := e.tasks[0]
		e.tasks[0] = nil
		e.tasks = e.tasks[1:]
		q.mu.Unlock()
		func() {
			defer func() { _ = recover() }()
			fn()
		}()
	}
}
