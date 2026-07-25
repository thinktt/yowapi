package games

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLockGameSerializesSameGame(t *testing.T) {
	var active int32
	var overlap int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			unlock := lockGame("game-1")
			if atomic.AddInt32(&active, 1) != 1 {
				atomic.StoreInt32(&overlap, 1)
			}
			atomic.AddInt32(&active, -1)
			unlock()
		}()
	}

	close(start)
	wg.Wait()

	if atomic.LoadInt32(&overlap) != 0 {
		t.Fatal("same-game operations overlapped")
	}
}

func TestLockGameCleansUpUnusedLocks(t *testing.T) {
	unlock := lockGame("game-cleanup")
	unlock()

	gameLocks.Lock()
	_, found := gameLocks.locks["game-cleanup"]
	gameLocks.Unlock()

	if found {
		t.Fatal("unused game lock was not removed")
	}
}
