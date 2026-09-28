package queue

import (
	"sync"
	"testing"
	"time"
)

func TestSameKeyIsSequential(t *testing.T) {
	q := New()
	var mu sync.Mutex
	var active, max int
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Do("user", func() {
				mu.Lock()
				active++
				if active > max {
					max = active
				}
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				active--
				mu.Unlock()
			})
		}()
	}
	wg.Wait()
	if max != 1 {
		t.Fatalf("max active=%d", max)
	}
	if len(q.locks) != 0 {
		t.Fatal("completed guest queues retained")
	}
}

func TestQueueReleasedAfterPanic(t *testing.T) {
	q := New()
	func() {
		defer func() { _ = recover() }()
		q.Do("guest", func() { panic("test") })
	}()
	q.Do("guest", func() {})
	if len(q.locks) != 0 {
		t.Fatal("queue leaked after panic")
	}
}
