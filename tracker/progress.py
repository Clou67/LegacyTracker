"""Turn raw game memory into 100% progress (one snapshot -> one dict)."""
import json
import os
import struct

from . import gamedata as G


class Snapshot:
    """The raw bytes the tracker needs, from RAM or from a save file."""

    def __init__(self, header, script_vars, inventory, abilities, source=""):
        self.header = bytes(header)          # 0x20
        self.vars = bytes(script_vars)       # 0x240
        self.inv = bytes(inventory)          # 0x130 (RAM layout) or None fields from a save
        self.abilities = bytes(abilities)    # 0x14
        self.source = source

    def key(self):
        return (self.header, self.vars, self.inv, self.abilities)


def hero_level(header):
    return min(header[G.HDR_LEVEL_INDEX] + 1, G.MAX_LEVEL)


class History:
    """Quests the tracker has watched become active, kept per save slot in a
    JSON file so that quests with no known "done" flag can still be marked done
    once their marker is cleared."""

    def __init__(self, path):
        self.path = path
        self.data = {}
        if path and os.path.exists(path):
            try:
                with open(path) as f:
                    self.data = json.load(f)
            except (OSError, ValueError):
                self.data = {}

    def seen(self, run):
        return set(self.data.get(run, []))

    def mark(self, run, quests):
        cur = self.seen(run)
        if quests - cur:
            self.data[run] = sorted(cur | quests)
            if self.path:
                tmp = self.path + ".tmp"
                with open(tmp, "w") as f:
                    json.dump(self.data, f, indent=1)
                os.replace(tmp, self.path)

    def reset(self, run):
        self.data.pop(run, None)
        if self.path:
            with open(self.path, "w") as f:
                json.dump(self.data, f, indent=1)


def evaluate(snap, history=None, run="default"):
    v = snap.vars
    inv = snap.inv
    hdr = snap.header

    # ---- quests
    active_now = set()
    for q in G.QUESTS:
        if v[G.QUEST_MARKER_BASE + q[0]]:
            active_now.add(q[0])
    if history is not None:
        history.mark(run, active_now)
        seen = history.seen(run)
    else:
        seen = set()

    quests = []
    for idx, area, name, start, done, egg in G.QUESTS:
        marker = v[G.QUEST_MARKER_BASE + idx] != 0
        done_flag = any(v[f] for f in done)
        started_flag = any(v[f] for f in start)
        if done_flag:
            state, how = "done", "flag"
        elif marker:
            state, how = "active", "marker"
        elif idx in seen:
            state, how = "done", "seen"
        elif started_flag:
            # handed out, marker gone, but no known completion flag
            state, how = ("done", "inferred") if not done else ("started", "flag")
        else:
            state, how = "todo", ""
        quests.append(dict(id=idx, area=area, name=name, state=state, how=how,
                           egg=egg, sure=bool(done)))

    n_done = sum(q["state"] == "done" for q in quests)

    # ---- eggs
    eggs_total = inv[G.INV_EGGS]
    quest_eggs = sum(1 for q in quests if q["egg"] and q["state"] == "done")
    overworld = max(0, eggs_total - quest_eggs)

    # ---- upgrades
    bits = struct.unpack_from("<H", inv, G.INV_UPGRADES)[0]
    upgrades = []
    for name, tiers, base in G.UPGRADES:
        owned = [bool(bits & b) for b, _ in tiers]
        cap = base
        for (b, c) in tiers:
            if bits & b:
                cap = c
        upgrades.append(dict(name=name, tiers=owned, capacity=cap,
                             max_capacity=tiers[-1][1], maxed=owned[-1],
                             all_bought=all(owned)))

    # ---- abilities
    abilities = []
    for name, i in G.ABILITIES:
        mx = G.ABILITY_MAX.get(i, G.DEFAULT_ABILITY_MAX)
        lv = snap.abilities[i]
        abilities.append(dict(name=name, level=lv, max=mx))

    # ---- level
    xp = struct.unpack_from("<I", hdr, G.HDR_XP)[0]
    lvl = hero_level(hdr)
    next_xp = G.XP_LEVEL_END[lvl - 1] if lvl < G.MAX_LEVEL else None

    res = dict(
        source=snap.source,
        level=dict(level=lvl, max=G.MAX_LEVEL, xp=xp, next_xp=next_xp),
        gems=struct.unpack_from("<H", inv, G.INV_GEMS)[0],
        quests=quests,
        quests_done=n_done,
        quests_total=len(quests),
        eggs=dict(total=eggs_total, from_quests=quest_eggs, overworld=overworld,
                  overworld_target=G.OVERWORLD_EGG_TARGET,
                  unattributed=G.UNATTRIBUTED_SCRIPT_EGGS),
        upgrades=upgrades,
        upgrade_bits=bits,
        abilities=abilities,
    )
    res["checklist"] = checklist(res)
    return res


def checklist(r):
    """The SSL 100% rules, each with done/total."""
    abil_done = sum(a["level"] >= a["max"] for a in r["abilities"])
    return [
        dict(rule="All quests", have=r["quests_done"], need=r["quests_total"]),
        dict(rule="40 overworld eggs (estimate)", have=min(r["eggs"]["overworld"], 40), need=40),
        dict(rule="All upgrade tiers bought", have=sum(sum(u["tiers"]) for u in r["upgrades"]),
             need=sum(len(u["tiers"]) for u in r["upgrades"])),
        dict(rule="Max upgrades (top tier only)", have=sum(u["maxed"] for u in r["upgrades"]),
             need=len(r["upgrades"])),
        dict(rule="All abilities maxed", have=abil_done, need=len(r["abilities"])),
        dict(rule="Level 25", have=r["level"]["level"], need=r["level"]["max"]),
    ]
