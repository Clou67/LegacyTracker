# How Spyro: Shadow Legacy tracks 100% progress

Sources: the projShadow decompilation (function and variable names, file
comments) and the game's own data, read out of the USA ROM (`ASSE`) with
`research/extract.py`. Every address below is a DS main-RAM address. The
projShadow PC port maps DS memory at the same addresses.

## Where progress lives

| What | Where | Notes |
|---|---|---|
| Story flags | `g_ScriptVars` 0x020D9F9C, 0x240 bytes | Saved as 0x200 one-bit flags plus 0x30 whole bytes (0x200-0x22F) |
| Hero level / XP | current slot header (`g_SlotHeaders` 0x020EBC40 + 0x20 × `g_CurrentSlot` 0x020BF6A0) | +0x12 level − 1 (0-24), +0x08 u32 XP |
| Inventory | `g_HeroPowerUps` 0x020DEF74, 0x130 bytes | +0x11C u16 gems, +0x11E u16 upgrade bits, +0x12B keys, +0x12C health potions, +0x12D mana potions, +0x12F eggs |
| Abilities | `g_AbilityLevels` 0x020EBCE4, 0x14 bytes | The status page shows 12 of them (`g_StatusCounters`) |

## Quests: there is a hidden quest list

The string table (`American_bin.rom`, strings 0xBA-0xF4) has **59 quest
names**. The touch-screen map (CollectionMenu1.c) puts an icon on the map for
every quest whose **marker** `g_ScriptVars[0x1C2 + i]` is set. Strings 0xBA + i
are the quest descriptions it shows.

The marker is 1 only while the quest is **active**. It is 0 both before the
quest starts and after it is finished. That is why the game itself can't tell
you what you've done. The level scripts (`*.app` files, script opcode 25 =
`VarSet`) always set a separate story flag right next to the line that clears
a marker when a quest is finished:

```
VarSet v0x34 1        <- "One Horn's gate opened" (stays set)
VarSet v0x1cd 0       <- quest marker cleared
StopSpyroAndTalk ...
GainXP 200
AddCounter12F         <- +1 egg
SaveGame
```

`tracker/gamedata.py` lists each quest's start and done flags. For 50 of the
58 real quests a done flag is known (three are the boss fights:
`BossBattle1.c` sets 0x78 / 0x8C / 0x100 when they are won). The other 8
(the "Rescue N …" quests, Rescue Sensei, Rescue Professor, Find Bianca #2,
Kaitlin's eggs) are cleared by scripts that only run once a counter is reached,
with no flag next to them. For those, the live tracker remembers seeing the
marker on (`tracker_history.json`) and counts the quest as done when the marker
goes away.

**Quest 58 "Defeat Shadow Minion" is never given by any script.** It is unused,
so 100% is **58 quests**, not 59.

## Eggs: why the count looks inconsistent

There is a single egg counter, inventory +0x12F. It counts *every* egg:
overworld pickups **and** quest rewards. Quest scripts give eggs with
`AddCounter12F` (script opcode 87). 11 scripted eggs were found:

* 9 for finishing a quest: Fonsworthy's talisman, One Horn's gate, Merton's
  conveyors, Pastemolle's spell, Isabel's ingredients, Akello's medicine,
  Zoe's wand, Bruno's fields and Defeat Red.
* 2 from event scripts with no quest next to them (Dragon Shores Inn floor 1,
  Skelos / Professor's Lab).
* The tracker estimates **overworld eggs = total − eggs from quests it has
  seen finished**. Two scripted eggs aren't tied to a quest yet, so the estimate
  can run up to 2 high until those are pinned down.

So "more than 40 eggs" is expected: about 40 overworld eggs plus 11
scripted eggs. No script takes eggs away. `PayGems` reads the egg counter but is
only used with 0 in the shipped scripts. Kaitlin's 30-egg reward only checks
the count.

## Upgrades: they do stack, but only the top tier matters

`Inventory_Give` (Inventory2.c) ORs a bit into +0x11E for each upgrade bought.
Every tier keeps its own bit, so the save records every tier that was bought.
But capacity is computed from the **highest bit owned** (`Inventory_ClampCounter`
and the potion and key cases of `Inventory_Give`):

| Upgrade | Tiers (bit → capacity) | Base |
|---|---|---|
| Wallet (gems) | 0x200 → 2499, 0x400 → 4999 | 999 |
| Butterfly bottle (mana potions) | 0x001 → 3, 0x002 → 6, 0x004 → 12 | 1 |
| Mushroom bag (health potions) | 0x040 → 3, 0x080 → 6, 0x100 → 12 | 1 |
| Keychain (keys) | 0x010 → 3, 0x020 → 6 | 1 |

Buying a lower tier after the top one changes nothing. So a "max upgrades"
rule (top tier only) is the same as "all upgrades" for gameplay, and the
tracker shows both. Bit 0x008 is set by a new game and never read. The bag
and bottle names are inferred from which potion each one limits. Check them in
game.

## Level

`g_XPLevelEnd` (0x020B6AF4) holds the XP at which each level ends: 2600, 6050,
… 210000, 220000. The level index at +0x12 goes up to 0x18, which is level 25.

## Open questions

* The individual overworld egg pickups. They are saved with the level's
  object state, not in the story flags, so the tracker can only count them.
* Which quest, if any, the two unattributed scripted eggs belong to.
* Each ability's real maximum. The tracker assumes 3, and 4 for Tail
  (`Abilities_SetLevel` only allows 4 for that counter). Confirm against the
  dojo.
* The rescue counters (e.g. `VarAdd v0x202` in Hermit's house). Mapping
  them would give done flags for the last 8 quests, so they would show
  correctly from a save file too.

## Route check (hundo_route.xlsx, "Quests" sheet)

All 58 quests were matched by name against the route's quest list:

* **Missing from the route: "Help Willard get home"** (Moleville, quest 52,
  400 XP; done flag 0xFA). It is a separate quest from "Get sunglasses for
  Willard".
* "Kill Shelly" is a story event, not one of the game's quests.
* The route lists three Shadow Minion fights (ice, Savannah, fire), but the
  game has two Shadow Minion quests: the ice minion (1st boss) and one that
  is handed out in Savannah and finished by the fire minion (2nd boss). The
  third "Defeat Shadow Minion" text is never used.
