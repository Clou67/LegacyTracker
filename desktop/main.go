// Spyro: Shadow Legacy 100% tracker, desktop version.
//
//	SSLTracker.exe                     window; watches melonDS / the PC port
//	SSLTracker.exe -save FILE [-slot N] window; reads a save file
//	SSLTracker.exe -text -save FILE    print the progress and exit
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	flagSave    = flag.String("save", "", "read this save file (melonDS .sav or spyro.sav) instead of the running game")
	flagSlot    = flag.Int("slot", 0, "save slot 1-3 (default: the first valid one)")
	flagText    = flag.Bool("text", false, "print the progress to the console and exit")
	flagRun     = flag.String("run", "run", "name for this run's quest history")
	flagHistory = flag.String("history", "", "history file (default: tracker_history.json next to the program)")
)

func historyPath() string {
	if *flagHistory != "" {
		return *flagHistory
	}
	exe, err := os.Executable()
	if err != nil {
		return "tracker_history.json"
	}
	return filepath.Join(filepath.Dir(exe), "tracker_history.json")
}

func main() {
	flag.Parse()
	if *flagText {
		attachConsole()
		if *flagSave == "" {
			fmt.Fprintln(os.Stderr, "-text needs -save FILE")
			os.Exit(2)
		}
		snap, idx, err := ReadSave(*flagSave, *flagSlot-1)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		h := LoadHistory(historyPath())
		run := fmt.Sprintf("%s:slot%d", filepath.Base(*flagSave), idx+1)
		fmt.Println(TextReport(Evaluate(snap, h, run)))
		return
	}
	runGUI()
}
