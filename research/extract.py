"""Re-derive the tracker's quest data from your own ROM.

  python research/extract.py "Spyro - Shadow Legacy (USA).nds" path/to/projShadow out/

Writes to out/: romfs/ (the ROM's files), strings.txt (the game's string table,
quest names are 0xba-0xf4), quest_flags.txt (every write to a quest marker in
the level scripts, with the commands around it) and eggs.txt (every scripted
egg). Needs projShadow only for config/arm9/symbols.txt (opcode names).
"""
import os, re, struct, sys

QUEST_BASE, QUEST_N = 0x1C2, 59
HANDLERS, LENGTHS = 0x020BCD20, 0x020AAC08


def rom_files(rom):
    fnt, _, fat, _ = struct.unpack_from("<IIII", rom, 0x40)
    out = []

    def walk(did, path):
        off, first, _ = struct.unpack_from("<IHH", rom, fnt + (did & 0xFFF) * 8)
        p, fid = fnt + off, first
        while rom[p]:
            b = rom[p]; p += 1
            name = rom[p:p + (b & 0x7F)].decode("latin1"); p += b & 0x7F
            if b & 0x80:
                walk(struct.unpack_from("<H", rom, p)[0], path + name + "/"); p += 2
            else:
                s, e = struct.unpack_from("<II", rom, fat + fid * 8)
                out.append((path + name, rom[s:e])); fid += 1
    walk(0xF000, "")
    return out


def arm9(rom):
    off, _, load, size = struct.unpack_from("<IIII", rom, 0x20)
    return rom[off:off + size], load   # this game's ARM9 is stored uncompressed


def strings(blob):
    offs = struct.unpack_from("<I", blob, 0)[0]
    res = []
    for i in range((len(blob) - offs) // 4):
        p = struct.unpack_from("<I", blob, offs + 4 * i)[0]
        if p >= len(blob): break
        bits, bit, out, two, code = blob[p], 0, bytearray(), 0, 0
        p += 1
        while True:
            node = 0x100
            while node >= 0x100:
                node = struct.unpack_from("<H", blob, 4 + (node - 0x100) * 4 + (2 if (bits >> bit) & 1 else 0))[0]
                bit += 1
                if bit == 8:
                    bit, bits = 0, blob[p]; p += 1
            c = node & 0xFF; out.append(c)
            if c >= 0xF0 and not two: two, code = 1, c << 8
            elif two: code |= c; two = 0
            else: code = c
            if code == 0: break
        res.append(bytes(out[:-1]).decode("latin1"))
    return res


def opcodes(code, load, symbols):
    syms = {}
    for l in open(symbols):
        m = re.match(r"(\S+) kind:function.*addr:(0x[0-9a-f]+)", l)
        if m: syms[int(m.group(2), 16)] = m.group(1).replace("ScriptCmd_", "")
    ops = {}
    for i in range(1, 300):
        h = struct.unpack_from("<I", code, HANDLERS - load + 4 * i)[0] & ~1
        if h not in syms: break
        ops[i] = (syms[h], code[LENGTHS - load + i])
    return ops


def decode(b, i, ops):
    out = []
    while i + 4 <= len(b):
        op = struct.unpack_from("<I", b, i)[0]
        if op not in ops or ops[op][1] < 4 or i + ops[op][1] > len(b): break
        out.append((i, op, b[i:i + ops[op][1]])); i += ops[op][1]
    return out


def fmt(op, c, ops):
    n, a = ops[op][0], c[4:]
    if op in (25, 41, 42): return f"{n} v{struct.unpack_from('<H', a)[0]:#x} {a[2]}"
    if op in (75, 77, 78, 88): return f"{n} {struct.unpack_from('<h', a)[0]}"
    return n + " " + a.hex()


def around(b, i, ops, before=4, after=6):
    for s in range(max(0, i - 64), i + 1, 4):
        ch = decode(b, s, ops)
        k = [j for j, (o, _, _) in enumerate(ch) if o == i]
        if k: return ch[max(0, k[0] - before):k[0] + after]
    return []


def main(rom_path, proj, out):
    rom = open(rom_path, "rb").read()
    os.makedirs(os.path.join(out, "romfs"), exist_ok=True)
    files = rom_files(rom)
    for name, data in files:
        p = os.path.join(out, "romfs", name)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        open(p, "wb").write(data)
    fs = dict(files)
    st = strings(fs["American_bin.rom"])
    with open(os.path.join(out, "strings.txt"), "w") as f:
        for i, s in enumerate(st): f.write(f"{i:#x} {s}\n")
    code, load = arm9(rom)
    ops = opcodes(code, load, os.path.join(proj, "config/arm9/symbols.txt"))
    apps = sorted(n for n in fs if n.endswith(".app"))
    with open(os.path.join(out, "quest_flags.txt"), "w") as f:
        for q in range(QUEST_N):
            f.write(f"\n## quest {q} marker v{QUEST_BASE + q:#x}: {st[0xBA + q]}\n")
            for n in apps:
                b = fs[n]
                for i in range(0, len(b) - 8, 4):
                    if b[i:i + 4] == b"\x19\0\0\0" and struct.unpack_from("<H", b, i + 4)[0] == QUEST_BASE + q:
                        f.write(f"  {n} @{i:x} -> {b[i + 6]}\n")
                        for o, op, c in around(b, i, ops):
                            f.write(f"      {'>' if o == i else ' '} {o:5x} {fmt(op, c, ops)}\n")
    with open(os.path.join(out, "eggs.txt"), "w") as f:
        for n in apps:
            b = fs[n]
            for i in range(0, len(b) - 4, 4):
                if b[i:i + 4] == b"\x57\0\0\0":
                    ctx = around(b, i, ops, 6, 2)
                    if len(ctx) >= 2:
                        f.write(f"{n} @{i:x}: " + " | ".join(fmt(op, c, ops) for _, op, c in ctx) + "\n")
    print("wrote", out)


if __name__ == "__main__":
    main(*sys.argv[1:4])
