//go:build !windows

package main

import (
	"fmt"
	"os"
)

func attachConsole() {}

func runGUI() {
	fmt.Fprintln(os.Stderr, "The window is Windows-only; use -text -save FILE here.")
	os.Exit(2)
}
