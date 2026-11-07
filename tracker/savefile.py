"""Read progress from a save file (melonDS .sav, or the PC port's spyro.sav).

Backup layout (SaveMem_LayOut, SaveGame2.c): an options block at 0, then three
slots of equal size, each a 0x20-byte header followed by the save stream. The
slot size depends on the backup chip, so we find the slots by their checksums:
a valid header's sixteen u16 words sum to 0, and header+0x1c is the u16 sum of
the stream that follows.

Stream (SaveGame_Save): visited-levels (u8 n, n bytes) | u16 len, 0x200 flag
bits + 0x30 bytes (g_ScriptVars) | u16 len, inventory | 0x14 ability levels |
level data.
"""
import struct

from .progress import Snapshot


def _sum16(b):
    return sum(struct.unpack_from("<%dH" % (len(b) // 2), b)) & 0xFFFF


def _valid_header(b):
    return len(b) == 0x20 and _sum16(b) == 0 and b != bytes(0x20)


def find_slots(data):
    """[(slot index, header offset, stream bytes)] for every valid slot."""
    heads = [o for o in range(0x10, len(data) - 0x20 + 1, 0x10) if _valid_header(data[o:o + 0x20])]
    if not heads:
        return []
    # slot stride: try what the headers imply, then common chunk layouts
    strides = set()
    for a in heads:
        for b in heads:
            if b > a:
                strides.add(b - a)
    first_candidates = sorted(set(heads))
    out = []
    for first in first_candidates:
        for stride in sorted(strides) + [(len(data) - first) // 3]:
            if stride <= 0x40:
                continue
            found = []
            for s in range(3):
                h = first + s * stride
                hdr = data[h:h + 0x20]
                if h + stride > len(data) + 0x10 or not _valid_header(hdr):
                    continue
                body = data[h + 0x20:h + stride]
                body = body[:len(body) & ~1]
                if _sum16(body) == struct.unpack_from("<H", hdr, 0x1C)[0]:
                    found.append((s, h, body))
            if found and len(found) >= len(out):
                out = found
        if out:
            break
    if not out:  # data checksum never matched: fall back to the headers alone
        out = [(i, h, data[h + 0x20:]) for i, h in enumerate(heads[:3])]
    return out


def parse_stream(stream):
    p = 0
    n = stream[p]
    p += 1 + n
    ln = struct.unpack_from("<H", stream, p)[0]
    p += 2
    flags = stream[p:p + 0x40]
    vars_ = bytearray(0x240)
    for i in range(0x200):
        vars_[i] = (flags[i >> 3] >> (i & 7)) & 1
    vars_[0x200:0x230] = stream[p + 0x40:p + 0x70]
    p += ln
    ln = struct.unpack_from("<H", stream, p)[0]
    p += 2
    s = stream[p:p + ln]
    p += ln
    inv = bytearray(0x130)
    q = 0
    inv[0x129] = s[q]; q += 1
    for i in range(5):
        struct.pack_into("<i", inv, 4 + i * 8, s[q]); inv[8 + i * 8] = s[q + 1]; inv[9 + i * 8] = s[q + 2]
        q += 3
    inv[0x12A] = s[q]; q += 1
    inv[0x120:0x129] = s[q:q + 9]; q += 9
    inv[0x11C:0x11E] = s[q:q + 2]; q += 2
    inv[0x11E:0x120] = s[q:q + 2]; q += 2
    inv[0x12B:0x130] = s[q:q + 5]
    abilities = stream[p:p + 0x14]
    return bytes(vars_), bytes(inv), bytes(abilities)


def read_save(path, slot=None):
    data = open(path, "rb").read()
    slots = find_slots(data)
    if not slots:
        raise ValueError("no valid save slot found in " + path)
    if slot is not None:
        slots = [s for s in slots if s[0] == slot] or slots
    out = []
    for idx, off, body in slots:
        hdr = data[off:off + 0x20]
        v, inv, ab = parse_stream(body)
        out.append((idx, Snapshot(hdr, v, inv, ab, source=f"{path} (slot {idx + 1})")))
    return out


def build_save(slots, slot_size=0x1000, total=0x8000):
    """Make a save image the way the game lays it out (used by the tests)."""
    data = bytearray(b"\xff" * total)
    for s, (hdr, stream) in slots.items():
        h = 0x100 + s * slot_size  # options chunk first
        body = bytearray(slot_size - 0x20)
        body[:len(stream)] = stream
        hdr = bytearray(hdr)
        struct.pack_into("<H", hdr, 0x1C, _sum16(body))
        struct.pack_into("<H", hdr, 0x1E, 0)
        struct.pack_into("<H", hdr, 0x1E, (-_sum16(hdr)) & 0xFFFF)
        data[h:h + 0x20] = hdr
        data[h + 0x20:h + slot_size] = body
    return bytes(data)


def build_stream(vars_, inv, abilities):
    out = bytearray([6]) + bytes(6)
    flags = bytearray(0x40)
    for i in range(0x200):
        if vars_[i]:
            flags[i >> 3] |= 1 << (i & 7)
    st = flags + vars_[0x200:0x230]
    out += struct.pack("<H", len(st)) + st
    s = bytearray([inv[0x129]])
    for i in range(5):
        s += bytes([struct.unpack_from("<i", inv, 4 + i * 8)[0] & 0xFF, inv[8 + i * 8], inv[9 + i * 8]])
    s += bytes([inv[0x12A]]) + inv[0x120:0x129] + inv[0x11C:0x120] + inv[0x12B:0x130]
    out += struct.pack("<H", len(s)) + s
    out += abilities
    return bytes(out)
