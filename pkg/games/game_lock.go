package games

import "sync"

type gameLock struct {
	mu   sync.Mutex
	refs int
}

var gameLocks = struct {
	sync.Mutex
	locks map[string]*gameLock
}{
	locks: make(map[string]*gameLock),
}

func lockGame(gameID string) func() {
	gameLocks.Lock()
	lock := gameLocks.locks[gameID]
	if lock == nil {
		lock = &gameLock{}
		gameLocks.locks[gameID] = lock
	}
	lock.refs++
	gameLocks.Unlock()

	lock.mu.Lock()

	return func() {
		lock.mu.Unlock()

		gameLocks.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(gameLocks.locks, gameID)
		}
		gameLocks.Unlock()
	}
}
