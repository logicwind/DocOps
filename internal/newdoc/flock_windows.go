//go:build windows

package newdoc

import "os"

// flock is a no-op on Windows — parallel safety on Windows would require
// LockFileEx which adds significant complexity for a platform rarely used
// with this CLI.
func flock(_ *os.File) error  { return nil }
func funlock(_ *os.File) error { return nil }
