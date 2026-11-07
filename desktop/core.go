// The tracker's logic: turns the game's saved data into 100% progress.
// A line-for-line port of tracker/progress.py, tracker/eggs.py and
// tracker/savefile.py; see research/RESEARCH.md for what everything means.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

type signature struct {
	addr  uint32
	bytes []byte
}
type tier struct {
	bit uint16
	cap int
}
type upgradeDef struct {
	name  string
	tiers []tier
	base  int
}
type abilityDef struct {
	name     string
	idx, max int
}
type flagCond struct{ v, min int } // min 0: flag set; else counter >= min
type questDef struct {
	id          int
	area, name  string
	start, done []flagCond
	reward      bool
}
type holeDef struct {
	hole  int
	where string
}
type npcEgg struct {
	who  string
	flag int
}
type eggDef struct {
	g, k int
	kind string
	x, y int
}
type levelDef struct {
	idx        int
	file, name string
	layout     []string
	eggs       []eggDef
}

func (c flagCond) ok(v []byte) bool {
	if c.min > 0 {
		return int(v[c.v]) >= c.min
	}
	return v[c.v] != 0
}

// ---------------------------------------------------------------- snapshot

type Snapshot struct {
	Header, Vars, Inv, Abilities []byte
	Stream                       []byte // the save stream (per-level objects), may be nil
	Level                        int    // level the hero is in (live), -1 if unknown
	Pending                      int    // eggs picked up in this level since entering it (live)
	Source                       string
}

func (s *Snapshot) Key() string {
	return fmt.Sprintf("%x|%x|%x|%x|%x|%d|%d", s.Header, s.Vars, s.Inv, s.Abilities, s.Stream, s.Level, s.Pending)
}

func u16(b []byte, o int) int    { return int(binary.LittleEndian.Uint16(b[o:])) }
func u32(b []byte, o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }

// ---------------------------------------------------------------- history

// History remembers quests seen active and a Hole egg picked up live, per run.
type History struct {
	path string
	data map[string]interface{}
}

func LoadHistory(path string) *History {
	h := &History{path: path, data: map[string]interface{}{}}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &h.data)
	}
	return h
}

func (h *History) seen(run string) map[int]bool {
	out := map[int]bool{}
	if l, ok := h.data[run].([]interface{}); ok {
		for _, x := range l {
			if f, ok := x.(float64); ok {
				out[int(f)] = true
			}
		}
	}
	return out
}

func (h *History) mark(run string, active map[int]bool) {
	cur := h.seen(run)
	changed := false
	for q := range active {
		if !cur[q] {
			cur[q], changed = true, true
		}
	}
	if changed {
		l := []int{}
		for q := range cur {
			l = append(l, q)
		}
		sort.Ints(l)
		h.data[run] = l
		h.write()
	}
}

func (h *History) flag(key string) bool { v, _ := h.data[key].(bool); return v }
func (h *History) setFlag(key string) {
	if !h.flag(key) {
		h.data[key] = true
		h.write()
	}
}

func (h *History) write() {
	if h.path == "" {
		return
	}
	b, _ := json.MarshalIndent(h.data, "", " ")
	tmp := h.path + ".tmp"
	if os.WriteFile(tmp, b, 0644) == nil {
		os.Rename(tmp, h.path)
	}
}

// ---------------------------------------------------------------- results

type Quest struct {
	ID         int
	Area, Name string
	State, How string // todo / active / started / done; flag / marker / seen / inferred
	Reward     bool
}
type EggRow struct {
	Kind, Where, Name string
	X, Y, Hole        int
	State             string // got / left / gone / unknown
}
type EggLevel struct {
	Level       int // -1 for NPC eggs
	Name        string
	Eggs        []EggRow
	Got, Total  int
	Here, Known bool
	Pending     int
	Hole, Note  string // The Hole only
}
type Upgrade struct {
	Name          string
	Tiers         []bool
	Capacity, Max int
	Maxed         bool
}
type Ability struct {
	Name       string
	Level, Max int
}
type Rule struct {
	Rule       string
	Have, Need int
}
type Result struct {
	Source                string
	Level, XP, NextXP     int
	Gems                  int
	Quests                []Quest
	QuestsDone, QuestsAll int
	EggLevels             []EggLevel
	EggsGot, EggsAll      int
	EggsHeld, EggTarget   int
	EggsKnown             bool
	Upgrades              []Upgrade
	Abilities             []Ability
	Checklist             []Rule
}

// ---------------------------------------------------------------- eggs

type bitReader struct {
	d      []byte
	p, bit int
}

func (r *bitReader) readBit() int {
	v := int(r.d[r.p]>>uint(r.bit)) & 1
	r.bit++
	if r.bit == 8 {
		r.bit, r.p = 0, r.p+1
	}
	return v
}
func (r *bitReader) readByte() int {
	if r.bit != 0 {
		r.bit, r.p = 0, r.p+1
	}
	v := int(r.d[r.p])
	r.p++
	return v
}
func (r *bitReader) end() int {
	if r.bit != 0 {
		return r.p + 1
	}
	return r.p
}

type objKey struct{ g, k int }

func parseLevel(stream []byte, pos int, layout []string) (st map[objKey]bool) {
	defer func() {
		if recover() != nil {
			st = nil
		}
	}()
	if pos <= 0 || pos+2 > len(stream) {
		return nil
	}
	n := u16(stream, pos)
	if n != len(layout) {
		return nil
	}
	offs := make([]int, n)
	for i := range offs {
		offs[i] = u16(stream, pos+2+2*i)
	}
	st = map[objKey]bool{}
	for g, codes := range layout {
		r := &bitReader{d: stream, p: offs[g]}
		for k, c := range codes {
			if c == '-' {
				continue
			}
			present := r.readBit()
			st[objKey{g, k}] = present == 1
			if present == 1 && c == 'b' {
				r.readBit()
			} else if present == 1 && c == 'B' {
				r.readByte()
			}
		}
		if g+1 < n && r.end() > offs[g+1] {
			return nil
		}
	}
	return st
}

func eggsFromStream(stream []byte) (res map[int]map[objKey]bool) {
	res = map[int]map[objKey]bool{}
	defer func() {
		if recover() != nil {
			res = map[int]map[objKey]bool{}
		}
	}()
	p := 1 + int(stream[0])
	p += 2 + u16(stream, p)
	p += 2 + u16(stream, p)
	base := p + 0x14
	for _, lv := range levels {
		if len(lv.eggs) == 0 {
			continue
		}
		if st := parseLevel(stream, u16(stream, base+2*lv.idx), lv.layout); st != nil {
			res[lv.idx] = st
		}
	}
	return res
}

func holeStatus(st map[objKey]bool, taken bool, currentHole int) (string, int, string) {
	eggs := levels[HoleLevel].eggs
	if taken {
		return "got", 1, "Hole egg collected."
	}
	if st == nil {
		return "unknown", 0, "No save data for The Hole yet."
	}
	here := []eggDef{}
	for _, e := range eggs {
		if v, ok := st[objKey{e.g, e.k}]; !ok || v {
			here = append(here, e)
		}
	}
	isEggHole := false
	for _, h := range holeEggGroups {
		if h.hole == currentHole {
			isEggHole = true
		}
	}
	if currentHole >= 0 && len(here) == len(eggs) {
		if isEggHole {
			return "inside", 0, fmt.Sprintf("You're in hole %d, an egg hole: grab the egg before you leave!", currentHole)
		}
		return "danger", 0, fmt.Sprintf("You're in hole %d, which has NO egg. Leaving now wipes every Hole egg for this file.", currentHole)
	}
	switch {
	case len(here) == len(eggs):
		return "open", 0, "Not entered yet. Only the first hole you go down keeps its egg: enter an egg hole first (Hermit Cave is the early one)."
	case len(here) == 1:
		h := holeEggGroups[here[0].g]
		return "waiting", 0, fmt.Sprintf("The egg is still in hole %d (%s): go back down that hole for it.", h.hole, h.where)
	case len(here) == 0:
		return "gone", 0, "The Hole has been entered and no Hole egg is left. If you didn't pick one up, it is lost for this file. (Watched live, the tracker marks the pickup itself.)"
	}
	return "open", 0, "Several Hole eggs are still there."
}

// ---------------------------------------------------------------- evaluate

func Evaluate(s *Snapshot, h *History, run string) *Result {
	v, inv, hdr := s.Vars, s.Inv, s.Header
	r := &Result{Source: s.Source}

	// quests
	active := map[int]bool{}
	for _, q := range quests {
		if v[QuestMarkerBase+q.id] != 0 {
			active[q.id] = true
		}
	}
	seen := map[int]bool{}
	if h != nil {
		h.mark(run, active)
		seen = h.seen(run)
	}
	for _, q := range quests {
		done, started := false, false
		for _, c := range q.done {
			done = done || c.ok(v)
		}
		for _, c := range q.start {
			started = started || c.ok(v)
		}
		st, how := "todo", ""
		switch {
		case done:
			st, how = "done", "flag"
		case active[q.id]:
			st, how = "active", "marker"
		case seen[q.id]:
			st, how = "done", "seen"
		case started && len(q.done) == 0:
			st, how = "done", "inferred"
		case started:
			st, how = "started", "flag"
		}
		r.Quests = append(r.Quests, Quest{q.id, q.area, q.name, st, how, q.reward})
		if st == "done" {
			r.QuestsDone++
		}
	}
	r.QuestsAll = len(quests)

	// eggs
	states := map[int]map[objKey]bool{}
	if len(s.Stream) > 0 {
		states = eggsFromStream(s.Stream)
	}
	r.EggsKnown = len(states) > 0
	holeKey := "hole-egg:" + run
	if h != nil && s.Level == HoleLevel && s.Pending > 0 {
		h.setFlag(holeKey)
	}
	holeTaken := h != nil && h.flag(holeKey)
	currentHole := -1
	if s.Level == HoleLevel {
		currentHole = int(v[0x184])
	}
	for _, lv := range levels {
		if len(lv.eggs) == 0 {
			continue
		}
		st, known := states[lv.idx]
		el := EggLevel{Level: lv.idx, Name: lv.name, Here: lv.idx == s.Level, Known: known}
		for _, e := range lv.eggs {
			state := "unknown"
			if known {
				state = "left"
				if p, ok := st[objKey{e.g, e.k}]; ok && !p {
					state = "got"
				}
			}
			row := EggRow{Kind: e.kind, X: e.x, Y: e.y, State: state}
			if lv.idx == HoleLevel {
				hd := holeEggGroups[e.g]
				row.Hole, row.Where = hd.hole, hd.where
				if state == "got" {
					row.State = "gone"
				}
			}
			el.Eggs = append(el.Eggs, row)
		}
		if lv.idx == HoleLevel {
			el.Name = "The Hole (1 obtainable)"
			el.Hole, el.Got, el.Note = holeStatus(st, holeTaken, currentHole)
			if !known && !holeTaken {
				el.Hole, el.Got, el.Note = holeStatus(nil, false, currentHole)
			}
			el.Total = 1
		} else {
			for _, e := range el.Eggs {
				if e.State == "got" {
					el.Got++
				}
			}
			el.Total = len(el.Eggs)
		}
		if el.Here {
			el.Pending = s.Pending
		}
		r.EggLevels = append(r.EggLevels, el)
		r.EggsGot += el.Got
		r.EggsAll += el.Total
	}
	npc := EggLevel{Level: -1, Name: "Given by NPCs", Known: true}
	for _, n := range npcEggs {
		st := "left"
		if v[n.flag] != 0 {
			st = "got"
			npc.Got++
		}
		npc.Eggs = append(npc.Eggs, EggRow{Kind: "npc", Name: n.who, State: st})
	}
	npc.Total = len(npcEggs)
	r.EggLevels = append(r.EggLevels, npc)
	r.EggsGot += npc.Got
	r.EggsAll += npc.Total
	r.EggsHeld = int(inv[InvEggs])
	r.EggTarget = OverworldEggTarget

	// The Hole from the egg count: every collected egg stays counted except
	// Kaitlin's, which you hand back.
	for i := range r.EggLevels {
		el := &r.EggLevels[i]
		if el.Hole == "gone" && len(states) > 0 {
			rest := r.EggsGot - el.Got
			expect := rest + s.Pending
			if v[KaitlinEggReturned] != 0 {
				expect--
			}
			if r.EggsHeld == expect+1 {
				el.Hole, el.Got, el.Note = "got", 1, "Hole egg collected (worked out from the egg count)."
				r.EggsGot++
			} else if r.EggsHeld == expect {
				el.Hole, el.Note = "lost", "Hole egg LOST: The Hole was entered and its eggs are gone, and the egg count shows none was picked up. Only the first hole entered keeps its egg."
			}
		}
	}
	if s.Level != HoleLevel {
		r.EggsGot += s.Pending
	}

	// upgrades
	bits := uint16(u16(inv, InvUpgrades))
	for _, u := range upgrades {
		up := Upgrade{Name: u.name, Capacity: u.base, Max: u.tiers[len(u.tiers)-1].cap}
		for _, t := range u.tiers {
			has := bits&t.bit != 0
			up.Tiers = append(up.Tiers, has)
			if has {
				up.Capacity = t.cap
			}
		}
		up.Maxed = up.Tiers[len(up.Tiers)-1]
		r.Upgrades = append(r.Upgrades, up)
	}
	for _, a := range abilities {
		r.Abilities = append(r.Abilities, Ability{a.name, int(s.Abilities[a.idx]), a.max})
	}

	// level
	r.XP = int(u32(hdr, HdrXp))
	r.Level = int(hdr[HdrLevelIndex]) + 1
	if r.Level > MaxLevel {
		r.Level = MaxLevel
	}
	if r.Level < MaxLevel {
		r.NextXP = xpLevelEnd[r.Level-1]
	}
	r.Gems = u16(inv, InvGems)

	// checklist
	tiersHave, tiersAll, maxed, abil := 0, 0, 0, 0
	for _, u := range r.Upgrades {
		for _, t := range u.Tiers {
			tiersAll++
			if t {
				tiersHave++
			}
		}
		if u.Maxed {
			maxed++
		}
	}
	for _, a := range r.Abilities {
		if a.Level >= a.Max {
			abil++
		}
	}
	r.Checklist = []Rule{
		{"All quests", r.QuestsDone, r.QuestsAll},
		{fmt.Sprintf("All eggs (%d obtainable; rules say %d)", r.EggsAll, r.EggTarget), r.EggsGot, r.EggsAll},
		{"All upgrade tiers bought", tiersHave, tiersAll},
		{"Max upgrades (top tier only)", maxed, len(r.Upgrades)},
		{"All abilities maxed", abil, len(r.Abilities)},
		{"Level 25", r.Level, MaxLevel},
	}
	return r
}

// ---------------------------------------------------------------- save files

func sum16(b []byte) uint16 {
	var s uint16
	for i := 0; i+1 < len(b); i += 2 {
		s += binary.LittleEndian.Uint16(b[i:])
	}
	return s
}

func validHeader(b []byte) bool {
	if len(b) != 0x20 || sum16(b) != 0 {
		return false
	}
	for _, x := range b {
		if x != 0 {
			return true
		}
	}
	return false
}

type saveSlot struct {
	idx, off int
	body     []byte
}

func findSlots(data []byte) []saveSlot {
	heads := []int{}
	for o := 0x10; o+0x20 <= len(data); o += 0x10 {
		if validHeader(data[o : o+0x20]) {
			heads = append(heads, o)
		}
	}
	if len(heads) == 0 {
		return nil
	}
	strides := map[int]bool{}
	for _, a := range heads {
		for _, b := range heads {
			if b > a {
				strides[b-a] = true
			}
		}
	}
	var out []saveSlot
	for _, first := range heads {
		cands := []int{}
		for s := range strides {
			cands = append(cands, s)
		}
		sort.Ints(cands)
		cands = append(cands, (len(data)-first)/3)
		for _, stride := range cands {
			if stride <= 0x40 {
				continue
			}
			var found []saveSlot
			for s := 0; s < 3; s++ {
				h := first + s*stride
				if h+0x20 > len(data) || h+stride > len(data)+0x10 || !validHeader(data[h:h+0x20]) {
					continue
				}
				end := h + stride
				if end > len(data) {
					end = len(data)
				}
				body := data[h+0x20 : end]
				body = body[:len(body)&^1]
				if sum16(body) == uint16(u16(data, h+0x1c)) {
					found = append(found, saveSlot{s, h, body})
				}
			}
			if len(found) > 0 && len(found) >= len(out) {
				out = found
			}
		}
		if len(out) > 0 {
			break
		}
	}
	if len(out) == 0 {
		for i, h := range heads {
			if i == 3 {
				break
			}
			out = append(out, saveSlot{i, h, data[h+0x20:]})
		}
	}
	return out
}

func parseStream(stream []byte) (vars, inv, ab []byte) {
	p := 0
	n := int(stream[p])
	p += 1 + n
	ln := u16(stream, p)
	p += 2
	flags := stream[p : p+0x40]
	vars = make([]byte, 0x240)
	for i := 0; i < 0x200; i++ {
		vars[i] = (flags[i>>3] >> uint(i&7)) & 1
	}
	copy(vars[0x200:0x230], stream[p+0x40:p+0x70])
	p += ln
	ln = u16(stream, p)
	p += 2
	s := stream[p : p+ln]
	p += ln
	inv = make([]byte, 0x130)
	q := 0
	inv[0x129] = s[q]
	q++
	for i := 0; i < 5; i++ {
		binary.LittleEndian.PutUint32(inv[4+i*8:], uint32(s[q]))
		inv[8+i*8], inv[9+i*8] = s[q+1], s[q+2]
		q += 3
	}
	inv[0x12a] = s[q]
	q++
	copy(inv[0x120:0x129], s[q:q+9])
	q += 9
	copy(inv[0x11c:0x120], s[q:q+4])
	q += 4
	copy(inv[0x12b:0x130], s[q:q+5])
	ab = append([]byte{}, stream[p:p+0x14]...)
	return
}

// ReadSave returns a snapshot for the chosen slot (slot < 0: the first valid one).
func ReadSave(path string, slot int) (snap *Snapshot, idx int, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("can't read %s: %v", path, e)
		}
	}()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	slots := findSlots(data)
	if len(slots) == 0 {
		return nil, 0, errors.New("no valid save slot found in " + path)
	}
	pick := slots[0]
	for _, s := range slots {
		if s.idx == slot {
			pick = s
		}
	}
	v, inv, ab := parseStream(pick.body)
	return &Snapshot{Header: data[pick.off : pick.off+0x20], Vars: v, Inv: inv, Abilities: ab,
		Stream: pick.body, Level: -1, Source: fmt.Sprintf("%s (slot %d)", path, pick.idx+1)}, pick.idx, nil
}

func statMod(path string) (time.Time, error) {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return st.ModTime(), nil
}
