//go:build !darwin

package core

func copyMetadata(source, destination string) error { return nil }
