package app

import (
	"fmt"
	"os"
	"syscall"
)

// Keep the lock inode across restarts. The kernel releases the lock when the process exits.
func lockJournal(statePath string) (*os.File, error) {
	f, err := os.OpenFile(statePath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock journal state (another Tailwatch may be running): %w", err)
	}
	return f, nil
}
