package main

import "testing"

func TestUTF16zKeepsNULs(t *testing.T) {
	f := utf16z("Save files (*.sav)\x00*.sav\x00All files\x00*.*\x00")
	nul := 0
	for _, c := range f {
		if c == 0 {
			nul++
		}
	}
	if nul != 5 || len(f) != 40 {
		t.Fatalf("filter encoded wrong: %d NULs, %d chars", nul, len(f))
	}
}
