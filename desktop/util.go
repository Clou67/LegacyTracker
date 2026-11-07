package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
	"unicode/utf16"
)

// utf16z encodes s (which may contain NULs, e.g. a file dialog filter) as a
// NUL-terminated UTF-16 buffer. syscall.UTF16FromString refuses embedded NULs.
func utf16z(s string) []uint16 {
	return append(utf16.Encode([]rune(s)), 0)
}

// logCrash appends an error and its stack trace to tracker_crash.log next to
// the program, and returns the log's path.
func logCrash(what interface{}) string {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	p := filepath.Join(dir, "tracker_crash.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		fmt.Fprintf(f, "%s  %v\n%s\n", time.Now().Format(time.RFC3339), what, debug.Stack())
		f.Close()
	}
	return p
}
