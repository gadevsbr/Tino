package queue

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestSameKeyIsFIFOAndSequential(t *testing.T) {
	q := New()
	var mu sync.Mutex
	var order []int
	var active, max int
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		i := i
		q.Enqueue("guest", func() {
			mu.Lock()
			active++
			if active > max {
				max = active
			}
			order = append(order, i)
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			if i == 9 {
				close(done)
			}
		})
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue did not drain")
	}
	if max != 1 || !reflect.DeepEqual(order, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("max=%d order=%v", max, order)
	}
}

func TestDifferentKeysRunConcurrently(t *testing.T) {
	q := New()
	started := make(chan string, 2)
	release := make(chan struct{})
	for _, key := range []string{"guest-a", "guest-b"} {
		key := key
		q.Enqueue(key, func() { started <- key; <-release })
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case key := <-started:
			seen[key] = true
		case <-time.After(time.Second):
			t.Fatal("different keys were blocked")
		}
	}
	close(release)
	if !seen["guest-a"] || !seen["guest-b"] {
		t.Fatal(seen)
	}
}

func TestPanicDoesNotBlockNextTask(t *testing.T) {
	q := New()
	done := make(chan struct{})
	q.Enqueue("guest", func() { panic("test") })
	q.Enqueue("guest", func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue remained blocked after panic")
	}
}
