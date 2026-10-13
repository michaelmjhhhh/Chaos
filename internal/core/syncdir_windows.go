//go:build windows

package core

// Windows does not support FlushFileBuffers on ordinary directory handles.
// File contents are flushed before publication, but directory-entry durability
// depends on the filesystem/OS. Returning an invalid-handle error here would
// falsely report every successful settings write and prevent source cleanup.
func syncDirectory(string) error { return nil }
