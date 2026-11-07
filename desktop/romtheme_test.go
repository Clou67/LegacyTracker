package main

import (
	"os"
	"testing"
)

// Structural checks only (sizes, counts); nothing from the ROM is written out.
func TestThemeFromROM(t *testing.T) {
	rom := os.Getenv("SSL_ROM")
	if rom == "" {
		t.Skip("set SSL_ROM to your .nds to test the game theme")
	}
	th, err := loadTheme(rom)
	if err != nil {
		t.Fatal(err)
	}
	if th.bg.w != 256 || th.bg.h < 192 {
		t.Fatalf("background is %dx%d, expected 256 wide and at least 192 tall", th.bg.w, th.bg.h)
	}
	colours := map[uint32]bool{}
	for _, p := range th.bg.pix {
		colours[p] = true
	}
	if len(colours) < 16 {
		t.Fatalf("background decoded to only %d colours", len(colours))
	}
	for _, f := range []*pixelFont{th.body, th.bold, th.small} {
		if w := f.textWidth("Rescue 6 dragons"); w < 40 || w > 200 {
			t.Fatalf("font %d px: odd text width %d", f.height, w)
		}
	}
	t.Logf("bg %dx%d, %d colours; fonts %d/%d/%d px", th.bg.w, th.bg.h, len(colours), th.body.height, th.bold.height, th.small.height)
}

func TestNotAROM(t *testing.T) {
	f := t.TempDir() + "/x.nds"
	os.WriteFile(f, make([]byte, 0x400), 0644)
	if _, err := loadTheme(f); err == nil {
		t.Fatal("accepted a file that isn't the game")
	}
}
