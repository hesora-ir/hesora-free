//go:build !windows

package datadir

func hidePath(string) error { return nil }
