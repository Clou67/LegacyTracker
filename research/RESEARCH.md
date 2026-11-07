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

`tracker/gamedata.py` lists each quest's start and done flags.

### Rescue quests count, they don't flag

The "Rescue N …" quests have no done flag. Each one counts rescues in a byte
variable (the 0x30 saved bytes at `g_ScriptVars[0x200..0x22F]`, changed with
`VarAdd`), and the quest is done when the count reaches its target. Both
finished 100% saves have every counter at its target:

| Counter | Quest | Target |
|---|---|---|
| 0x200 | Rescue 6 dragons (Dragon Shores) | 6 |
| 0x202 | Rescue 4 Elders (counts the Elders and Sensei) | 6 |
| 0x203 | Rescue 5 armadillos | 5 |
| 0x205 | Rescue 6 bears | 4 |
| 0x206 | Rescue 6 Dragons (Dragon Village) | 6 |
| 0x209 | Rescue Sensei (set to 2 when he is freed) | 2 |
| 0x20A | Rescue 4 cavemen | 4 |
| 0x20C | Rescue 5 cheetahs | 5 |
| 0x20D | Rescue 4 fairies | 4 |
| 0x20E | Rescue 4 bunnies | 4 |
| 0x20F | Rescue 3 moles | 3 |
| 0x21F | Rescue 5 wizards | 5 |

(The bears' script compares against 4, though the quest text says 6.)

Other quests with no done flag of their own are cleared by another quest's
script: **Rescue Professor** is cleared in the same cutscene as the fire
Shadow Minion quest (done flag 0x8C). **Find Bianca** (Moleville) is cleared
in the Defeat Red script (done flag 0xFD). **Find dragon eggs for Kaitlin**
is still worked out as "started and cleared".

The three boss fights set their done flags in code (`BossBattle1.c` sets
0x78, 0x8C and 0x100 when they are won).

### Unused quests: 100% is 57 quests

* **#58 "Defeat Shadow Minion"** is never given by any script.
* **#52 "Help Willard get home"** (put out the torches so Willard can walk
  home in the dark) is cut content. Its scripts exist in `Moleville.app`:
  the question "Can you put out the torches?" (script 42,
  conversation `Willard_Torch_Start`), the start (43: sets 0xF9 and the
  marker), the torch counters (34-37, 54), their resets (119-124) and the
  finish (125: sets 0xFA, clears the marker). But no script, object, path or
  entry point ever runs script 42. Willard's talk script (26) checks 0xF9
  and, because it is never set, always goes straight to "Get sunglasses for
  Willard". Both finished saves have 0xF9 = 0.

So the game's 59 quest texts give **57 real quests**. Both finished saves
read 57/57.

## Eggs

**Correction:** an earlier version of this file said the egg counter was
inventory +0x12F. That was wrong. +0x12F is a separate counter that quest
rewards add to (`AddCounter12F`, script opcode 87; it's probably the
route sheet's "relics"). Eggs are **item 25**, held at inventory **+0x12E**:
`Inventory_Give` case 25 adds to it, Kaitlin's script takes one back when you
return her lost egg (`AddCounter -1`), and her reward script checks the count
against 30 (`SoundByCounter 30`).

### Where every egg is

Each overworld egg is a **level object**, not a story flag:

* the level list is table 25 of the game database (`TablesR_*.rom`, rows of
  `g_LevelAreas`). Its `ScriptData` column is the FAT file id of the level's
  `.app` file.
* a `.app` file holds the level's object groups (`Appearances_BuildTables`).
  Each object record starts with its type. **Eggs are type-10 items with item
  25 at +8 (22 of them), and type-6 chests with item 25 in a drop slot
  (+0x0A, +0x10 or +0x16; 19 of them).**

That makes **41 egg objects in 23 levels** (`tracker/levels_data.py`, made by
`research/extract_eggs.py`). 5 of them are in The Hole (next section), and
only one of those can ever be collected.

Four more eggs are handed over by NPCs (script `GiveOneItem 25`):

| NPC | Quest | Flag set alongside |
|---|---|---|
| Laird | Retrieve lotion from Laird's room | 0x13 |
| Aqua | Put out fires for Aqua | 0x131 |
| Dirt | Build altar for Dirt | 0x83 |
| Lucia | Zap chickens for Lucia | 0xDB |

### Totals, and where the route's "40" comes from

| | Eggs |
|---|---|
| Placed outside The Hole | 36 |
| The Hole (5 placed, only 1 obtainable) | 1 |
| Given by NPCs | 4 |
| **Obtainable on one file** | **41** |
| Existing in the game data | 45 |

The route's Eggs sheet lists 40: the 36 placed eggs plus the 4 NPC eggs. It
matches every map except The Hole: Dragonfly Dojo, Dragon Village, Dragon
Shores, Savannah, Treetops, Lost Fleet and Moleville match egg for egg, and
the remaining areas match in total. (The route's "Dragon Mouth Cave" is
`HermitCave`, and "outside Sensei's house" is `HermitHouse`.) **The 41st egg
is the one Hole egg.**

## The Hole: why only one egg

The Hole (level 0x19, `Hole.app`) is a single map holding **28 holes**. Each
hole is one object group with its own enemies, gates and items. The holes are
entered from all over the world: a `WarpToMap` script command (opcode 85)
sends you to map 0x19 and stores the hole number in `g_ScriptVars[0x184]`. A
script in `Hole.app` then compares 0x184 against each hole number and spawns
that hole's group (hole *n* → group *n* + 1).

Every other map loads all of its object groups when you enter it.
`Map_InitAppearances` (Map1.c) has a special case for level 0x19: it loads only
the shared groups 0 and 1, and the one hole's group is spawned by that script.

When you leave any map, the game auto-saves it (`Map_Exit` → `SaveGame_Save`).
`Appearances_WriteGroup` writes one bit per saved object: 1 if the object's
actor exists, 0 if not. It doesn't care whether a group was never loaded. So
when you leave The Hole, every saved object of the 27 holes you aren't in is
written as **gone**. The next time it loads them (`Appearances_LoadGroup`), the
game sees a 0 and doesn't spawn them. Whichever hole you enter first is the
only one that keeps its contents.

| Hole | Group | Entrance (map with the `WarpToMap`) | Egg |
|---|---|---|---|
| 21 | 22 | Hermit Cave (Dragon Mouth Cave) | loose |
| 22 | 23 | Wilderness, Forgotten Worlds | in a chest |
| 25 | 26 | Dragon Shores, Dragon Shores Cave | loose |
| 26 | 27 | Wilderness, Ice boss area (both versions) | loose |
| 27 | 28 | Wilderness, Avalar | loose |

The other 23 holes have no egg. Consequences:

* **Exactly one Hole egg can be collected, and only if the first hole you ever
  go down is one of the five above.** Hermit Cave (hole 21) is the early one.
* Entering any other hole first (and leaving) loses all five for that file.
  That matches the in-game observation: "after you collect one egg from the
  Hole, no hole ever has an egg again".
* If you enter an egg hole and leave without the egg, it stays: that hole's
  group was loaded, so its egg is saved as still there. You can go back down
  the same hole for it.
* The same happens to everything else saved in the holes (chests, breakables),
  not just eggs.

**The two finished saves agree with this.** In both, The Hole was entered and
all 5 eggs read as gone. The egg count is 39: 36 placed + 4 NPC eggs, minus
Kaitlin's lost egg, which you hand back. A Hole egg would make it 40. So
both runs entered a hole without an egg (or left an egg hole after another
one), and the Hole egg was lost.

A cleaner check on a fresh file:
(1) enter Hermit Cave's hole first, take the egg, then check that the
Dragon Shores hole is empty; (2) on another file, enter a hole with no egg
first, then check that every egg hole is empty.

How the tracker handles it: it counts The Hole as one egg. Reading the save
(or RAM) gives "not entered yet" (all 5 still there), "the egg is still in
hole *n*" (one left), or none left. When none are left, the egg count
settles it: every egg you collect stays in the count except Kaitlin's (flag
0x32 marks it returned). One more than expected means the Hole egg was
taken, exactly the expected number means it was lost. The live tracker
also records an egg picked up inside The Hole. It
also reads 0x184 while you're in The Hole and warns you if you're in a hole
without an egg before anything has been taken.

### How the game remembers them (your byte-array hunch, almost)

The game keeps no flag per egg in RAM. Each level's object state is kept as a
**bit array in the save stream** (`Appearances_WriteGroup`). For every object
that keeps state there is one bit: 1 = still there, 0 = gone. A few object
types follow their bit with a state bit (signs, platforms) or a state byte
(counters). The level data sits after the abilities: a table of 0x29 u16
positions, then per level a u16 group count, u16 group offsets, and the
groups' bits. A picked-up egg and an opened egg chest both read as 0.

The game auto-saves on **every map exit** (`Map_Exit` → `SaveGame_Save`). So
the save file, and the stream buffer in RAM (`g_SaveStream`), cover every
level up to the last map change. The tracker reads both. In the level you
are standing in, it shows eggs picked up since you entered as "+N" until you
leave.

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

* Confirm The Hole behaviour in game (tests above), and check whether the
  `WarpToMap` entries with hole number 0 (Boat, Titan's house, Hermit House,
  DR North) are real hole entrances or another use of the command.
* ~~Confirm the save layout on a real save~~: done with two finished 100%
  saves (melonDS, 8 KB). Every placed egg outside The Hole reads as collected.
* Each ability's real maximum. The tracker assumes 3, and 4 for Tail
  (`Abilities_SetLevel` only allows 4 for that counter). Confirm against the
  dojo.
* The rescue counters (e.g. `VarAdd v0x202` in Hermit's house). Mapping
  them would give done flags for the last 8 quests, so they would show
  correctly from a save file too.

## Route check (hundo_route.xlsx, "Quests" sheet)

All quests were matched by name against the route's quest list:

* The route has every real quest. The one it lacks, "Help Willard get home",
  is unused content (see "Unused quests").
* "Kill Shelly" is a story event, not one of the game's quests.
* The route lists three Shadow Minion fights (ice, Savannah, fire), but the
  game has two Shadow Minion quests: the ice minion (1st boss) and one that
  is handed out in Savannah and finished by the fire minion (2nd boss). The
  third "Defeat Shadow Minion" text is never used.

## Graphics used by the desktop theme

The desktop tracker's "Game theme" loads these from the user's own ROM at
run time. They are never stored in the repository or the program.

* `B_Stats_BKG.bin`: the pause menu's status (XP) page background
  (`StatusMenu_Enter` loads resource 0x179). It's a BG blob (`BG_LoadBlob`,
  BG2.c): flags 0xF9 = 256-colour palette, map, tiles, 8 bpp, tiles packed
  with the game's own compressor (kind 3, `Unpack_Data` at 0x02000DF4; the
  native version is `N_Unpack_Data` in projShadow `pc/src/game_asm.c`). It is
  32×28 tiles (256×224 px).
* Fonts (`g_FontFiles`, `Font_Load` in Font.c): `drift8psa.bin` (8 px),
  `hachipochi12p.bin` (12 px), `GoblinBold15_White.bin` (14 px),
  `KataFont.bin`. Each holds u16 first/last character, the height, and
  big-endian offsets to a glyph offset table, a width table and 4-bit
  pixels stored column by column.
