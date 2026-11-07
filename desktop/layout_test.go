package main

import (
	"fmt"
	"html"
	"os"
	"strings"
	"testing"
)

// svgCanvas renders draw() to an SVG so the layout can be checked without Windows.
type svgCanvas struct{ b strings.Builder }

func col(c uintptr) string { return fmt.Sprintf("#%02x%02x%02x", c&0xff, c>>8&0xff, c>>16&0xff) }
func (s *svgCanvas) fill(x, y, w, h int, c uintptr) {
	fmt.Fprintf(&s.b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, w, h, col(c))
}
func (s *svgCanvas) frame(x, y, w, h int, c uintptr) {
	fmt.Fprintf(&s.b, `<rect x="%d.5" y="%d.5" width="%d" height="%d" fill="none" stroke="%s"/>`, x, y, w-1, h-1, col(c))
}
func (s *svgCanvas) text(x, y, w int, t string, c uintptr, size int, bold bool, flags uintptr) int {
	wt := 400
	if bold {
		wt = 600
	}
	per := int(float64(size) * 0.55)
	lines := []string{t}
	if flags&dtWordBrk != 0 {
		lines = nil
		cur := ""
		for _, wd := range strings.Fields(t) {
			if len(cur)+len(wd)+1 > w/per && cur != "" {
				lines = append(lines, cur)
				cur = wd
			} else if cur == "" {
				cur = wd
			} else {
				cur += " " + wd
			}
		}
		lines = append(lines, cur)
	}
	anchor, ax := "start", x
	if flags&dtRight != 0 {
		anchor, ax = "end", x+w
	}
	for i, l := range lines {
		fmt.Fprintf(&s.b, `<text x="%d" y="%d" font-family="Segoe UI,sans-serif" font-size="%d" font-weight="%d" fill="%s" text-anchor="%s">%s</text>`,
			ax, y+size+i*(size+5), size, wt, col(c), anchor, html.EscapeString(l))
	}
	if flags&dtWordBrk != 0 {
		return len(lines) * (size + 5)
	}
	return size + 6
}

func TestLayoutSVG(t *testing.T) {
	path := os.Getenv("SSL_SAVE")
	if path == "" {
		t.Skip("set SSL_SAVE to a save file")
	}
	snap, _, err := ReadSave(path, -1)
	if err != nil {
		t.Fatal(err)
	}
	snap.Level, snap.Vars[0x184] = HoleLevel, 3 // pretend we're live in a hole with no egg
	r := Evaluate(snap, nil, "t")
	for _, wcase := range []struct {
		w       int
		compact bool
		name    string
	}{{1180, false, "wide"}, {420, true, "compact"}} {
		s := &svgCanvas{}
		h := draw(s, r, "Live: melonDS.exe (slot 1)", false, wcase.compact, wcase.w)
		out := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><rect width="100%%" height="100%%" fill="%s"/>%s</svg>`, wcase.w, h, col(cBg), s.b.String())
		os.WriteFile(os.Getenv("SSL_OUT")+"layout_"+wcase.name+".svg", []byte(out), 0644)
	}
}
