"""Build tracker/levels_data.py (levels, their saved-object layout and their eggs) from your ROM.

  python research/extract_eggs.py "Spyro - Shadow Legacy (USA).nds" tracker/levels_data.py

How it works (see RESEARCH.md, "Eggs per level"):
* TablesR_str.rom / TablesR_bin.rom hold the game's database; table 25 is the
  level list (g_LevelAreas), whose ScriptData column is the FAT file id of the
  level's .app file.
* A .app file holds the level's object groups (Appearances_BuildTables). Each
  object starts with its type (u32). Eggs are item 25: type-10 items with item
  25 at +8, and type-6 chests with item 25 in one of their three drop slots
  (+0x0a, +0x10, +0x16).
* The layout lists, per group, how each object is written to the save
  (Appearances_WriteGroup): '-' not saved, 'n' a presence bit only, 'b' a
  presence bit then a state bit (signs, platforms), 'B' a presence bit then a
  state byte (counters).
"""
import struct, sys

TABLE_LEVELS = 25
EGG = 25
SAVED_BYTE = {0: None, 2: 0x10, 4: 0x1B, 5: 0x8, 6: 0x19, 7: 0x7, 10: 0x6, 12: 0x6, 13: 0x2, 14: 0x4, 17: 0xE}
STATE = {5: "b", 17: "b", 14: "B"}

NICE = {
    "Airship": "Airship", "BearForest": "Bear Forest", "BearForestMazeOne": "Bear Forest Maze 1",
    "BearForestMazeTwo": "Bear Forest Maze 2", "BearForestWarehouse": "Bear Forest Warehouse",
    "BearHouseInt": "Bear House", "Boat": "Boat", "CliffTown": "Cliff Town", "DojoHouse2": "Dojo House 2",
    "DragonFlyDojoTempleInt": "Dojo Temple", "DragonShores": "Dragon Shores", "DragonShoresCave": "Dragon Shores Cave",
    "DragonShoresHouse1": "Dragon Shores House 1", "DragonShoresHouse2": "Dragon Shores House 2",
    "DragonShoresInnFloor1": "Dragon Shores Inn 1F", "DragonShoresInnFloor2": "Dragon Shores Inn 2F",
    "DragonShoresIslands": "Dragon Shores Islands", "DragonVillage": "Dragon Village",
    "DragonVillageHouse2": "Dragon Village House", "DragonVillageInt": "Dragon Village Interior",
    "DragonflyDojo": "Dragonfly Dojo", "DragonflyDojoHouse1": "Dojo House 1 (Titan)", "HermitCave": "Hermit Cave",
    "HermitHouse": "Hermit House", "HermitHouseInt": "Hermit House Interior", "Hole": "The Hole",
    "LostFleet": "Lost Fleet", "Moleville": "Moleville", "Savannah": "Savannah", "SavannahInterior": "Savannah Interior",
    "SkelosProfLab": "Skelos Badlands / Prof. Lab", "SorcerersLand": "Sorcerer's Land", "Treetops": "Treetops",
    "WildernessAVCentral": "Wilderness (Avalar)", "WildernessDRBoss": "Wilderness (Ice boss)",
    "WildernessDRCentral": "Wilderness (Dragon Realms)", "WildernessDRNoIceBoss": "Wilderness (after Ice boss)",
    "WildernessDRNorth": "Wilderness (DR North)", "WildernessFWCentral": "Wilderness (Forgotten Worlds)",
    "WizardPeak": "Wizard Peak", "WizardPeakAviary": "Wizard Peak Aviary",
}

U16 = lambda b, o: struct.unpack_from("<H", b, o)[0]
U32 = lambda b, o: struct.unpack_from("<I", b, o)[0]


def rom_fs(rom):
    fnt, _, fat, _ = struct.unpack_from("<IIII", rom, 0x40)
    byid, byname = {}, {}

    def walk(did, path):
        off, first, _ = struct.unpack_from("<IHH", rom, fnt + (did & 0xFFF) * 8)
        p, fid = fnt + off, first
        while rom[p]:
            b = rom[p]; p += 1
            n = rom[p:p + (b & 0x7F)].decode("latin1"); p += b & 0x7F
            if b & 0x80:
                walk(U16(rom, p), path + n + "/"); p += 2
            else:
                s, e = struct.unpack_from("<II", rom, fat + fid * 8)
                byid[fid] = path + n; byname[path + n] = rom[s:e]; fid += 1
    walk(0xF000, "")
    return byid, byname


def level_table(fs):
    s, b = fs["TablesR_str.rom"], fs["TablesR_bin.rom"]
    c0, c1 = 4, 0
    for i in range(U32(s, 0)):
        ln = U32(s, c0); rec = s[c0 + 4:c0 + 4 + 0xA20]; c0 += 0xA24 + ln + (4 - (ln & 3))
        cols, rs = U32(rec, 8), U32(rec, 0xC)
        rows, ln2 = struct.unpack_from("<II", b, c1); c1 += 8
        data = b[c1:c1 + rs * rows]; c1 += rs * rows + ln2 + (4 - (ln2 & 3))
        if i == TABLE_LEVELS:
            return [struct.unpack_from("<i", data, k * rs + 6 * 4)[0] for k in range(rows)]


def groups(res, tbl):
    out = []
    for g in range(U16(res, tbl)):
        e = tbl + U16(res, tbl + 2 + 2 * g)
        n = U16(res, e) & 0xFF
        src = e + ((n + 2 + 3) & ~3)
        objs = []
        for k in range(n):
            ln = res[e + 2 + k]; objs.append(res[src:src + ln]); src += ln
        out.append(objs)
    return out


def parse_app(res):
    hdr = U16(res, 0)
    idx = res[hdr + 1]
    a, b = U16(res, hdr + 0xC), U16(res, hdr + idx * 8 + 0xC)
    gs = groups(res, b + 8)
    if U16(res, b + 4) & 1:
        gs = [groups(res, a + 8)[0]] + gs
    return gs


def describe(gs):
    layout, eggs = [], []
    for g, objs in enumerate(gs):
        codes = ""
        for k, o in enumerate(objs):
            t = U32(o, 0)
            sb = SAVED_BYTE.get(t, None)
            saved = t == 0 or (sb is not None and o[8 + sb] != 0)
            codes += STATE.get(t, "n") if saved else "-"
            x, y = struct.unpack_from("<hh", o, 4)
            if t == 10 and U16(o, 8) == EGG:
                eggs.append((g, k, "item", x, y, saved))
            if t == 6 and EGG in (U16(o, 0xA), U16(o, 0x10), U16(o, 0x16)):
                eggs.append((g, k, "chest", x, y, saved))
        layout.append(codes)
    return layout, eggs


def main(rom_path, out_path):
    rom = open(rom_path, "rb").read()
    byid, fs = rom_fs(rom)
    levels = []
    for i, fid in enumerate(level_table(fs)):
        name = byid[fid].rsplit(".", 1)[0]
        layout, eggs = describe(parse_app(fs[byid[fid]]))
        bad = [e for e in eggs if not e[5]]
        assert not bad, f"egg without saved state in {name}: {bad}"
        levels.append((i, name, NICE.get(name, name), layout, [e[:5] for e in eggs]))
    with open(out_path, "w") as f:
        f.write('"""Generated by research/extract_eggs.py from the game ROM. Do not edit by hand.\n\n'
                "LEVELS: (level index, file, name, save layout per object group, eggs)\n"
                "egg: (group, object index in the group, 'item' | 'chest', map x, map y)\n"
                '"""\n\nLEVELS = [\n')
        for lv in levels:
            f.write(f"    {lv!r},\n")
        f.write("]\n")
    print(f"{sum(len(l[4]) for l in levels)} eggs in {sum(1 for l in levels if l[4])} levels -> {out_path}")


if __name__ == "__main__":
    main(*sys.argv[1:3])
