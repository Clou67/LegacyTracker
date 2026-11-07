"""Run: python -m unittest discover tests"""
import os, struct, sys, tempfile, unittest
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from tracker import gamedata as G, progress, savefile
from tracker.__main__ import text_report


def make_state(level_index=24, xp=215000, eggs=48, upgrades=0x7ff, abil=3, quests_done=True):
    hdr = bytearray(0x20)
    hdr[G.HDR_LEVEL_INDEX] = level_index
    struct.pack_into("<I", hdr, G.HDR_XP, xp)
    v = bytearray(0x240)
    inv = bytearray(0x130)
    struct.pack_into("<H", inv, G.INV_UPGRADES, upgrades)
    struct.pack_into("<H", inv, G.INV_GEMS, 1234)
    inv[G.INV_EGGS] = eggs
    ab = bytearray(0x14)
    for _, i in G.ABILITIES:
        ab[i] = G.ABILITY_MAX.get(i, abil) if abil == 3 else abil
    if quests_done:
        for q in G.QUESTS:
            for f in q[4]:
                v[f] = 1
            for f in q[3]:
                v[f] = 1
    return hdr, v, inv, ab


class T(unittest.TestCase):
    def test_empty(self):
        r = progress.evaluate(progress.Snapshot(*make_state(0, 0, 0, 8, 0, False)))
        self.assertEqual(r["quests_done"], 0)
        self.assertEqual(r["level"]["level"], 1)
        self.assertTrue(all(not u["maxed"] for u in r["upgrades"]))
        self.assertEqual(r["upgrades"][0]["capacity"], 999)

    def test_quest_states(self):
        hdr, v, inv, ab = make_state(quests_done=False)
        v[G.QUEST_MARKER_BASE + 11] = 1          # One Horn active
        v[0x36] = 1                              # Flame done flag
        v[0x3B] = 1                              # bears started, no done flag known
        r = progress.evaluate(progress.Snapshot(hdr, v, inv, ab))
        st = {q["id"]: q["state"] for q in r["quests"]}
        self.assertEqual(st[11], "active")
        self.assertEqual(st[14], "done")
        self.assertEqual(st[16], "done")       # inferred
        self.assertEqual(st[0], "todo")

    def test_history(self):
        with tempfile.TemporaryDirectory() as d:
            h = progress.History(os.path.join(d, "h.json"))
            hdr, v, inv, ab = make_state(quests_done=False)
            v[G.QUEST_MARKER_BASE + 10] = 1      # Rescue 6 Dragons: no flags known
            progress.evaluate(progress.Snapshot(hdr, v, inv, ab), h, "r")
            v[G.QUEST_MARKER_BASE + 10] = 0
            h2 = progress.History(os.path.join(d, "h.json"))   # survives a restart
            r = progress.evaluate(progress.Snapshot(hdr, v, inv, ab), h2, "r")
            q = [q for q in r["quests"] if q["id"] == 10][0]
            self.assertEqual((q["state"], q["how"]), ("done", "seen"))

    def test_upgrades_top_tier_only(self):
        hdr, v, inv, ab = make_state(upgrades=0x400 | 0x004 | 0x100 | 0x020)
        r = progress.evaluate(progress.Snapshot(hdr, v, inv, ab))
        self.assertTrue(all(u["maxed"] for u in r["upgrades"]))
        self.assertFalse(any(u["all_bought"] for u in r["upgrades"]))
        self.assertEqual([u["capacity"] for u in r["upgrades"]], [4999, 12, 12, 6])

    def test_full(self):
        r = progress.evaluate(progress.Snapshot(*make_state()))
        eggq = sum(q[5] for q in G.QUESTS)
        self.assertEqual(r["eggs"]["overworld"], 48 - eggq)
        self.assertEqual(r["level"]["level"], 25)
        flagged = sum(1 for q in G.QUESTS if q[3] or q[4])
        self.assertEqual(r["quests_done"], flagged)   # the rest need the live history
        text_report(dict(r, status="save"))

    def test_save_roundtrip(self):
        hdr, v, inv, ab = make_state(level_index=9, xp=61000, eggs=12, upgrades=0x8 | 0x200 | 0x1)
        v[G.QUEST_MARKER_BASE + 5] = 1
        v[0x210] = 7
        stream = savefile.build_stream(v, inv, ab)
        img = savefile.build_save({1: (hdr, stream)})
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "x.sav")
            open(p, "wb").write(img)
            (idx, snap), = savefile.read_save(p)
        self.assertEqual(snap.vars[:0x230], bytes(v[:0x230]))
        self.assertEqual(snap.abilities, bytes(ab))
        for off in (G.INV_GEMS, G.INV_UPGRADES, G.INV_EGGS):
            self.assertEqual(snap.inv[off], inv[off])
        r = progress.evaluate(snap)
        self.assertEqual(r["level"]["level"], 10)
        self.assertEqual(r["eggs"]["total"], 12)

    def test_signatures_match_rom(self):
        rom = os.environ.get("SSL_ROM")
        if not rom:
            self.skipTest("set SSL_ROM to the .nds to check the signatures")
        d = open(rom, "rb").read()
        off, _, load, size = struct.unpack_from("<IIII", d, 0x20)
        for addr, sig in G.SIGNATURES:
            self.assertEqual(d[off + addr - load: off + addr - load + len(sig)], sig)


if __name__ == "__main__":
    unittest.main()
