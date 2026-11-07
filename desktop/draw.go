// Window layout, shared by the Windows window and the layout test.
package main

import "fmt"

// canvas is what draw() paints on (Win32 GDI in the app, SVG in the test).
type canvas interface {
	fill(x, y, w, h int, c uintptr)
	frame(x, y, w, h int, c uintptr)
	text(x, y, w int, s string, c uintptr, size int, bold bool, flags uintptr) int
}

const (
	dtLeft     = 0x0
	dtRight    = 0x2
	dtSingle   = 0x20
	dtNoPrefix = 0x800
	dtEndEll   = 0x8000
	dtWordBrk  = 0x10
	dtCalcRect = 0x400
)

func rgb(r, g, b int) uintptr { return uintptr(r | g<<8 | b<<16) }

var (
	cBg     = rgb(0x14, 0x11, 0x1c)
	cPanel  = rgb(0x1e, 0x1a, 0x2a)
	cLine   = rgb(0x34, 0x2d, 0x47)
	cText   = rgb(0xec, 0xe8, 0xf5)
	cDim    = rgb(0x8f, 0x87, 0xa6)
	cDone   = rgb(0x7d, 0xdc, 0x8a)
	cActive = rgb(0xf2, 0xc1, 0x4e)
	cTodo   = rgb(0x4a, 0x43, 0x60)
	cAccent = rgb(0xb4, 0x8c, 0xff)
	cRed    = rgb(0xe0, 0x60, 0x5a)
	cRedBg  = rgb(0x3a, 0x1d, 0x22)
)

func pips(p canvas, xRight, y int, on []bool, unknown []bool) {
	n := len(on)
	for i := 0; i < n; i++ {
		x := xRight - (n-i)*11
		c := cTodo
		if on[i] {
			c = cDone
		}
		if unknown != nil && unknown[i] {
			p.frame(x, y+4, 8, 8, cDim)
			continue
		}
		if on[i] {
			p.fill(x, y+4, 8, 8, c)
		} else {
			p.frame(x, y+4, 8, 8, c)
		}
	}
}

func panel(p canvas, x, y, w, h int, title string) {
	p.fill(x, y, w, h, cPanel)
	p.frame(x, y, w, h, cLine)
	p.text(x+12, y+8, w-24, title, cDim, 10, true, dtLeft)
}

func pct(have, need int) float64 {
	if need == 0 {
		return 1
	}
	f := float64(have) / float64(need)
	if f > 1 {
		f = 1
	}
	return f
}

// draw lays out the whole window at logical width lw; returns the content height
func draw(p canvas, r *Result, status string, hide, compact bool, lw int) int {
	m := 12
	p.text(m, 8, 300, "SHADOW LEGACY 100%", cAccent, 15, true, dtLeft)
	p.text(m+200, 11, lw-200-2*m, status, cDim, 10, false, dtLeft)
	y0 := 36
	if r == nil {
		p.text(m, y0+20, lw-2*m, "Start melonDS or the PC port and load your game, or use File > Open save file.", cDim, 12, false, dtWordBrk)
		return y0 + 60
	}

	leftW := 330
	if compact || lw < 700 {
		leftW = lw - 2*m
	}
	x := m
	y := y0

	// checklist
	ph := 30 + len(r.Checklist)*30
	panel(p, x, y, leftW, ph, "100% CHECKLIST")
	yy := y + 30
	for _, c := range r.Checklist {
		col := cText
		if c.Have >= c.Need {
			col = cDone
		}
		p.text(x+12, yy, leftW-90, c.Rule, col, 11, false, dtLeft)
		p.text(x+leftW-80, yy, 68, fmt.Sprintf("%d/%d", c.Have, c.Need), map[bool]uintptr{true: cDone, false: cDim}[c.Have >= c.Need], 11, false, dtRight)
		p.fill(x+12, yy+19, leftW-24, 4, cTodo)
		bc := cAccent
		if c.Have >= c.Need {
			bc = cDone
		}
		p.fill(x+12, yy+19, int(float64(leftW-24)*pct(c.Have, c.Need)), 4, bc)
		yy += 30
	}
	y += ph + 10

	// level & eggs
	panel(p, x, y, leftW, 92, "LEVEL & EGGS")
	p.text(x+12, y+28, 150, fmt.Sprintf("Lv %d", r.Level), cText, 22, true, dtLeft)
	xp := fmt.Sprintf("%d XP (max)", r.XP)
	if r.NextXP > 0 {
		xp = fmt.Sprintf("%d / %d XP", r.XP, r.NextXP)
	}
	p.text(x+12, y+60, 170, xp, cDim, 10, false, dtLeft)
	p.text(x+leftW-162, y+28, 150, fmt.Sprintf("%d/%d", r.EggsGot, r.EggsAll), cText, 22, true, dtRight)
	p.text(x+leftW-212, y+60, 200, fmt.Sprintf("eggs (%d held, rules say %d)", r.EggsHeld, r.EggTarget), cDim, 10, false, dtRight)
	y += 102

	if !compact {
		// upgrades
		ph = 30 + len(r.Upgrades)*21 + 6
		panel(p, x, y, leftW, ph, "UPGRADES")
		yy = y + 30
		for _, u := range r.Upgrades {
			p.text(x+12, yy, 160, u.Name, cText, 11, false, dtLeft)
			p.text(x+leftW-70, yy, 58, fmt.Sprintf("%d/%d", u.Capacity, u.Max), cDim, 10, false, dtRight)
			pips(p, x+leftW-74, yy, u.Tiers, nil)
			yy += 21
		}
		y += ph + 10
		// abilities
		ph = 30 + len(r.Abilities)*21 + 6
		panel(p, x, y, leftW, ph, "ABILITIES")
		yy = y + 30
		for _, a := range r.Abilities {
			p.text(x+12, yy, 160, a.Name, cText, 11, false, dtLeft)
			on := make([]bool, a.Max)
			for i := range on {
				on[i] = i < a.Level
			}
			pips(p, x+leftW-12, yy, on, nil)
			yy += 21
		}
		y += ph + 10
	}
	leftBottom := y

	// right column (or below, when narrow)
	rx, ry, rw := x+leftW+10, y0, lw-leftW-10-2*m
	if compact || lw < 700 {
		rx, ry, rw = x, leftBottom, lw-2*m
	}
	cols := 2
	if rw < 520 {
		cols = 1
	}
	colW := (rw - 24 - (cols-1)*16) / cols

	if !compact {
		// quests, grouped by area, flowing into columns
		type item struct {
			head string
			q    *Quest
		}
		var items []item
		area := ""
		for i := range r.Quests {
			q := &r.Quests[i]
			if hide && q.State == "done" {
				continue
			}
			if q.Area != area {
				area = q.Area
				items = append(items, item{head: area})
			}
			items = append(items, item{q: q})
		}
		rows := (len(items) + cols - 1) / cols
		ph = 32 + rows*19 + 8
		panel(p, rx, ry, rw, ph, fmt.Sprintf("QUESTS %d/%d", r.QuestsDone, r.QuestsAll))
		for i, it := range items {
			c, row := i/rows, i%rows
			ix, iy := rx+12+c*(colW+16), ry+30+row*19
			if it.q == nil {
				p.text(ix, iy+3, colW, it.head, cDim, 9, true, dtLeft)
				continue
			}
			dot := map[string]uintptr{"done": cDone, "active": cActive, "started": cActive, "todo": cTodo}[it.q.State]
			p.fill(ix, iy+6, 7, 7, dot)
			col := cText
			if it.q.State == "done" {
				col = cDim
			}
			name := it.q.Name
			if it.q.How == "seen" || it.q.How == "inferred" {
				name += "  (" + it.q.How + ")"
			}
			p.text(ix+13, iy, colW-13, name, col, 11, false, dtLeft)
		}
		ry += ph + 10
	}

	// eggs by level
	var lvls []EggLevel
	var hole *EggLevel
	for i := range r.EggLevels {
		l := r.EggLevels[i]
		if l.Hole != "" {
			hole = &r.EggLevels[i]
		}
		if hide && l.Got == l.Total {
			continue
		}
		lvls = append(lvls, l)
	}
	rows := (len(lvls) + cols - 1) / cols
	noteH := 0
	note := ""
	if hole != nil {
		note = "The Hole: " + hole.Note
		noteH = 66
	}
	ph = 32 + rows*19 + 10 + noteH + 40
	panel(p, rx, ry, rw, ph, fmt.Sprintf("EGGS BY LEVEL %d/%d", r.EggsGot, r.EggsAll))
	for i, l := range lvls {
		c, row := i/rows, i%rows
		ix, iy := rx+12+c*(colW+16), ry+30+row*19
		col := cText
		if l.Got == l.Total {
			col = cDim
		}
		if l.Here {
			col = cActive
		}
		name := l.Name
		if l.Here {
			name = "> " + name
		}
		if l.Pending > 0 && l.Hole == "" {
			name += fmt.Sprintf("  +%d", l.Pending)
		}
		p.text(ix, iy, colW-110, name, col, 11, false, dtLeft)
		p.text(ix+colW-40, iy, 40, fmt.Sprintf("%d/%d", l.Got, l.Total), cDim, 10, false, dtRight)
		var on, unk []bool
		if l.Hole != "" {
			on, unk = []bool{l.Got > 0}, []bool{false}
		} else {
			for _, e := range l.Eggs {
				on = append(on, e.State == "got")
				unk = append(unk, e.State == "unknown")
			}
		}
		pips(p, ix+colW-46, iy, on, unk)
	}
	ny := ry + 30 + rows*19 + 6
	if hole != nil {
		bg, fg, bd := cPanel, cText, cLine
		switch hole.Hole {
		case "danger", "gone", "lost":
			bg, fg, bd = cRedBg, rgb(0xff, 0xb4, 0xae), cRed
		case "inside", "waiting":
			fg, bd = cActive, cActive
		case "got":
			fg = cDone
		}
		p.fill(rx+12, ny, rw-24, 60, bg)
		p.frame(rx+12, ny, rw-24, 60, bd)
		p.text(rx+20, ny+4, rw-40, note, fg, 10, false, dtWordBrk|dtLeft)
		ny += noteH
	}
	p.text(rx+12, ny, rw-24, "Filled: got   Empty: still there   Grey outline: level not in the save yet. A level updates when you leave it (the game auto-saves).", cDim, 9, false, dtWordBrk|dtLeft)
	end := ry + ph
	if leftBottom > end {
		end = leftBottom
	}
	return end + 12
}
