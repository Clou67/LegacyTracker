package main

import (
	"fmt"
	"strings"
)

// TextReport prints a result the same way as `python -m tracker --text`.
func TextReport(r *Result) string {
	var o []string
	o = append(o, "== "+r.Source)
	for _, c := range r.Checklist {
		m := " "
		if c.Have >= c.Need {
			m = "x"
		}
		o = append(o, fmt.Sprintf(" [%s] %s: %d/%d", m, c.Rule, c.Have, c.Need))
	}
	lv := fmt.Sprintf("\nLevel %d  XP %d", r.Level, r.XP)
	if r.NextXP > 0 {
		lv += fmt.Sprintf("/%d", r.NextXP)
	}
	o = append(o, lv)
	o = append(o, fmt.Sprintf("Eggs: %d/%d obtainable (rules say %d; %d held now)", r.EggsGot, r.EggsAll, r.EggTarget, r.EggsHeld))
	for _, l := range r.EggLevels {
		m := " "
		if l.Got == l.Total {
			m = "x"
		}
		here, status := "", ""
		if l.Here {
			here = "  <- here"
			if l.Pending > 0 {
				here += fmt.Sprintf(" (+%d picked up, saved when you leave)", l.Pending)
			}
		}
		if !l.Known {
			status = "  (not in this save yet)"
		}
		o = append(o, fmt.Sprintf("  [%s] %-28s %d/%d%s%s", m, l.Name, l.Got, l.Total, status, here))
		if l.Hole != "" {
			o = append(o, "        "+l.Note)
		}
	}
	o = append(o, "Upgrades:")
	for _, u := range r.Upgrades {
		t := ""
		for _, x := range u.Tiers {
			if x {
				t += "#"
			} else {
				t += "-"
			}
		}
		o = append(o, fmt.Sprintf("  %-17s [%s] capacity %d/%d", u.Name, t, u.Capacity, u.Max))
	}
	o = append(o, "Abilities:")
	var ab []string
	for _, a := range r.Abilities {
		ab = append(ab, fmt.Sprintf("%s %d/%d", a.Name, a.Level, a.Max))
	}
	o = append(o, "  "+strings.Join(ab, ", "))
	o = append(o, fmt.Sprintf("Quests %d/%d:", r.QuestsDone, r.QuestsAll))
	sym := map[string]string{"done": "x", "active": ">", "started": "?", "todo": " "}
	area := ""
	for _, q := range r.Quests {
		if q.Area != area {
			area = q.Area
			o = append(o, "  "+area)
		}
		how := ""
		if q.How == "seen" || q.How == "inferred" {
			how = " (" + q.How + ")"
		}
		o = append(o, fmt.Sprintf("    [%s] %s%s", sym[q.State], q.Name, how))
	}
	return strings.Join(o, "\n")
}
