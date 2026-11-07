package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// settings are kept in tracker_settings.json next to the program.
type settings struct {
	ROM     string `json:"rom"`     // the user's own ROM, for the game theme
	Theme   bool   `json:"theme"`   // use the game theme
	Hide    bool   `json:"hide"`    // hide finished
	Compact bool   `json:"compact"` // checklist and eggs only
	Top     bool   `json:"on_top"`  // always on top
}

func settingsPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "tracker_settings.json"
	}
	return filepath.Join(filepath.Dir(exe), "tracker_settings.json")
}

func loadSettings() settings {
	var s settings
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

func (s settings) save() {
	b, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(settingsPath(), b, 0644)
}
