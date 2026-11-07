"""Eggs per level.

Every overworld egg is a level object: a loose egg (item 25) or a chest that
drops one. The game remembers each object per level in the save stream: one
"still there" bit per saved object, group by group (Appearances_WriteGroup).
0 means the object is gone, which for an egg means it was picked up and for a
chest means it was opened. The game writes the current level into the
stream every time you leave a map (Map_Exit -> SaveGame_Save), so the stream
in RAM and the save file both cover every level up to the last map change.
Eggs picked up in the level you are standing in show as "pending" until then.
"""
import struct

from . import gamedata as G
from .levels_data import LEVELS

N_LEVELS = 0x29


class BitReader:
    def __init__(self, data, pos):
        self.d, self.p, self.bit = data, pos, 0

    def read_bit(self):
        v = (self.d[self.p] >> self.bit) & 1
        self.bit += 1
        if self.bit == 8:
            self.bit, self.p = 0, self.p + 1
        return v

    def read_byte(self):
        if self.bit:
            self.bit, self.p = 0, self.p + 1
        v = self.d[self.p]
        self.p += 1
        return v

    def end(self):
        return self.p + (1 if self.bit else 0)


def levels_offset(stream):
    """Where the level position table starts (after visited, state, inventory, abilities)."""
    p = 1 + stream[0]
    p += 2 + struct.unpack_from("<H", stream, p)[0]
    p += 2 + struct.unpack_from("<H", stream, p)[0]
    return p + 0x14


def parse_level(stream, pos, layout):
    """{(group, index): present} for the saved objects of one level, or None if
    the data doesn't match the layout."""
    if pos <= 0 or pos + 2 > len(stream):
        return None
    n = struct.unpack_from("<H", stream, pos)[0]
    if n != len(layout):
        return None
    offs = struct.unpack_from("<%dH" % n, stream, pos + 2)
    out = {}
    try:
        for g, codes in enumerate(layout):
            r = BitReader(stream, offs[g])
            for k, c in enumerate(codes):
                if c == "-":
                    continue
                present = r.read_bit()
                out[(g, k)] = bool(present)
                if present and c == "b":
                    r.read_bit()
                elif present and c == "B":
                    r.read_byte()
            if g + 1 < n and r.end() > offs[g + 1]:
                return None
    except IndexError:
        return None
    return out


def from_stream(stream):
    """{level index: {(group, index): present}} for every level found in a save stream."""
    base = levels_offset(stream)
    positions = struct.unpack_from("<%dH" % N_LEVELS, stream, base)
    res = {}
    for idx, _file, _name, layout, eggs in LEVELS:
        if not eggs:
            continue
        st = parse_level(stream, positions[idx], layout)
        if st is not None:
            res[idx] = st
    return res


def hole_status(st, taken, current_hole=None):
    """The Hole: only the first hole entered keeps its egg (see gamedata.HOLE_EGG_GROUPS).
    current_hole: the hole the hero is in now (live), else None."""
    eggs = [e for e in LEVELS[G.HOLE_LEVEL][4]]
    egg_holes = {h: w for h, w in G.HOLE_EGG_GROUPS.values()}
    if taken:
        return "got", 1, "Hole egg collected."
    if st is None:
        return "unknown", 0, "No save data for The Hole yet."
    here = [(g, k) for g, k, *_ in eggs if st.get((g, k), True)]
    if current_hole is not None and len(here) == len(eggs):
        if current_hole in egg_holes:
            return "inside", 0, f"You're in hole {current_hole}, an egg hole: grab the egg before you leave!"
        return "danger", 0, (f"You're in hole {current_hole}, which has NO egg. Leaving now wipes every Hole egg "
                             "for this file.")
    if len(here) == len(eggs):
        return "open", 0, ("Not entered yet. The first hole you go down is the only one that keeps its egg: "
                           "enter an egg hole first (Hermit Cave is the early one).")
    if len(here) == 1:
        hole, where = G.HOLE_EGG_GROUPS[here[0][0]]
        return "waiting", 0, f"The egg is still in hole {hole} ({where}): go back down that hole for it."
    if not here:
        return "gone", 0, ("The Hole has been entered and no Hole egg is left. If you didn't pick one up, "
                           "it is lost for this file. (Watched live, the tracker marks the pickup itself.)")
    return "open", 0, "Several Hole eggs are still there."


def summarize(level_states, live_level=None, vars_=None, hole_taken=False):
    current_hole = vars_[0x184] if (vars_ is not None and live_level == G.HOLE_LEVEL) else None
    """Per-level egg list. level_states: from_stream(); live_level: the
    level the hero is in now; vars_: the story flags (for NPC eggs);
    hole_taken: the tracker saw an egg picked up inside The Hole."""
    levels = []
    total = got = 0
    for idx, file, name, layout, eggs in LEVELS:
        if not eggs:
            continue
        st = level_states.get(idx)
        rows = []
        for g, k, kind, x, y in eggs:
            eid = f"{file}:{g}:{k}"
            state = "unknown" if st is None else ("left" if st.get((g, k), True) else "got")
            row = dict(id=eid, kind=kind, x=x, y=y, state=state)
            if idx == G.HOLE_LEVEL:
                row["hole"], row["where"] = G.HOLE_EGG_GROUPS[g]
                row["state"] = "left" if state == "left" else ("unknown" if state == "unknown" else "gone")
            rows.append(row)
        lv = dict(level=idx, file=file, name=name, eggs=rows, here=idx == live_level, known=st is not None)
        if idx == G.HOLE_LEVEL:
            status, n_got, msg = hole_status(st, hole_taken, current_hole)
            lv.update(got=n_got, total=1, hole=status, note=msg)
        else:
            lv.update(got=sum(r["state"] == "got" for r in rows), total=len(rows))
        levels.append(lv)
        total += lv["total"]
        got += lv["got"]
    # eggs NPCs hand over
    rows = []
    for who, flag in G.NPC_EGGS:
        state = "unknown" if vars_ is None else ("got" if vars_[flag] else "left")
        rows.append(dict(id="npc:" + who, kind="npc", name=who, state=state))
    n = sum(r["state"] == "got" for r in rows)
    levels.append(dict(level=None, file="npc", name="Given by NPCs", eggs=rows, got=n, total=len(rows),
                       here=False, known=vars_ is not None))
    return dict(levels=levels, got=got + n, total=total + len(rows))


def egg_ids_for_level(idx):
    for i, file, _n, _l, eggs in LEVELS:
        if i == idx:
            return {(g, k): f"{file}:{g}:{k}" for g, k, *_ in eggs}
    return {}


class BitWriter:
    def __init__(self):
        self.d, self.bit = bytearray(), 0

    def write_bit(self, v):
        if self.bit == 0:
            self.d.append(0)
        if v:
            self.d[-1] |= 1 << self.bit
        self.bit = (self.bit + 1) & 7

    def write_byte(self, v):
        self.bit = 0
        self.d.append(v & 0xFF)

    def align(self):
        self.bit = 0


def build_levels(start, gone=(), visited=None):
    """The levels part of a save stream as the game writes it (for tests):
    every saved object present except the (level, group, index) in `gone`."""
    gone = set(gone)
    out = bytearray(N_LEVELS * 2)
    pos = [0] * N_LEVELS
    for idx, _f, _n, layout, _e in LEVELS:
        if visited is not None and idx not in visited:
            continue
        here = start + len(out)
        pos[idx] = here
        blob = bytearray(struct.pack("<H", len(layout))) + bytes(2 * len(layout))
        offs = []
        for g, codes in enumerate(layout):
            offs.append(here + len(blob))
            w = BitWriter()
            for k, c in enumerate(codes):
                if c == "-":
                    continue
                p = (idx, g, k) not in gone
                w.write_bit(p)
                if p and c == "b":
                    w.write_bit(0)
                elif p and c == "B":
                    w.write_byte(3)
            blob += w.d
        struct.pack_into("<%dH" % len(offs), blob, 2, *offs)
        out += blob
    struct.pack_into("<%dH" % N_LEVELS, out, 0, *pos)
    return bytes(out)
