//go:build darwin || linux

package career

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
)

var errLifecycleGuardBusy = errors.New("lifecycle execution guard is held")

func syscallFlock(file *os.File, lock bool) error {
	how := syscall.LOCK_UN
	if lock {
		how = syscall.LOCK_EX | syscall.LOCK_NB
	}
	err := syscall.Flock(int(file.Fd()), how)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return errLifecycleGuardBusy
	}
	return err
}

type lifecycleGuardSet struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func newLifecycleGuardSet() *lifecycleGuardSet {
	return &lifecycleGuardSet{locks: make(map[string]chan struct{})}
}
func (s *lifecycleGuardSet) acquire(ctx context.Context, key string) (func(), error) {
	s.mu.Lock()
	ch := s.locks[key]
	if ch == nil {
		ch = make(chan struct{}, 1)
		ch <- struct{}{}
		s.locks[key] = ch
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
		return func() { ch <- struct{}{} }, nil
	}
}
