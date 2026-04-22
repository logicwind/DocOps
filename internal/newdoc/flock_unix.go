//go:build !windows

package newdoc

import (
	"os"
	"syscall"
)

// flock acquires an exclusive lock on f. Blocks until the lock is available.
func flock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// funlock releases the lock on f.
func funlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
