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


def summarize(level_states, live_level=None):
    """Per-level egg list. level_states: from_stream(); live_level: the
    level the hero is in now."""
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
            rows.append(dict(id=eid, kind=kind, x=x, y=y, state=state))
        n_got = sum(r["state"] == "got" for r in rows)
        levels.append(dict(level=idx, file=file, name=name, eggs=rows, got=n_got, total=len(rows),
                           here=idx == live_level, known=st is not None))
        total += len(rows)
        got += n_got
    return dict(levels=levels, got=got, total=total)


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
