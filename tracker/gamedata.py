"""
Everything the tracker knows about Spyro: Shadow Legacy (USA, ASSE).

All addresses are DS main-RAM addresses (0x02000000 + offset). They come from
the projShadow decompilation (config/arm9/symbols.txt) and hold for both
melonDS (emulated RAM) and the projShadow PC port, which maps DS memory at
the original addresses. See research/RESEARCH.md for how each one was found.
"""

DS_MAIN = 0x02000000
DS_MAIN_SIZE = 0x400000

# ---- RAM addresses (bss of the main ARM9 module)
ADDR_CURRENT_SLOT = 0x020BF6A0   # s32 g_CurrentSlot (0-2)
ADDR_SLOT_HEADERS = 0x020EBC40   # 3 x 0x20-byte slot headers; the current one holds the live stats
ADDR_SCRIPT_VARS = 0x020D9F9C    # u8[0x240] g_ScriptVars (saved story flags)
ADDR_INVENTORY = 0x020DEF74      # u8[0x130] g_HeroPowerUps
ADDR_ABILITIES = 0x020EBCE4      # u8[0x14]  g_AbilityLevels (progress counters)
ADDR_SAVE_STREAM = 0x020EBBC8    # g_SaveStream: +0 buffer pointer, +0xe u16 size
ADDR_GROUP_TABLE = 0x020D9F90    # u8 *g_GroupTable: the current level's objects (+0 of a record: its actor)
ADDR_CURRENT_MAP = 0x020DF128    # s32 g_CurrentMap: level index (0-0x28)

# Code bytes used to find emulated RAM inside another process: (DS address, bytes).
# Taken from the static ARM9 code, which the game never overwrites.
SIGNATURES = [
    (0x02001000, bytes.fromhex("01b045e22294a0e1ff9019e20200000a1bb9a0e1adffffeb05b08be10854b0e1")),
    (0x02040000, bytes.fromhex("18408ce58c20cce5440095e5042091e5ac1090e564009fe5042081e51b11ffeb")),
]

# ---- slot header (0x20 bytes)
HDR_XP = 0x08          # u32 experience
HDR_LEVEL_INDEX = 0x12 # u8 hero level - 1 (0..24)
HDR_MAP = 0x0F         # bits 1-7: current map
MAX_LEVEL = 25
# experience at which each level ends (g_XPLevelEnd); level n+1 starts at XP_LEVEL_END[n-1]
XP_LEVEL_END = [2600, 6050, 10650, 16400, 23300, 31350, 40550, 50900, 60000, 70000,
                80000, 90000, 100000, 110000, 120000, 130000, 140000, 150000, 160000,
                170000, 180000, 190000, 200000, 210000, 220000]

# ---- inventory (g_HeroPowerUps)
INV_GEMS = 0x11C       # u16 gems in the wallet
INV_UPGRADES = 0x11E   # u16 upgrade bits
INV_KEYS = 0x12B       # u8
INV_HEALTH_POTIONS = 0x12C
INV_MANA_POTIONS = 0x12D
INV_EGGS = 0x12E       # u8 dragon eggs held (item 25; Kaitlin takes back her lost one)
INV_REWARDS = 0x12F    # u8 a counter that quest rewards add to (AddCounter12F); NOT eggs

# Upgrade bits (Inventory_Give / Inventory_ClampCounter). Each purchase ORs in its
# own bit; the capacity comes from the HIGHEST bit owned, so lower tiers have no
# effect once the top one is bought.
UPGRADES = [
    # name, [(bit, capacity), ...] lowest tier first, base capacity
    ("Wallet", [(0x200, 2499), (0x400, 4999)], 999),
    ("Butterfly bottle", [(0x001, 3), (0x002, 6), (0x004, 12)], 1),   # mana potions
    ("Mushroom bag", [(0x040, 3), (0x080, 6), (0x100, 12)], 1),       # health potions
    ("Keychain", [(0x010, 3), (0x020, 6)], 1),                        # keys
]
# bit 0x008 is set by a new game and never read.

# ---- abilities: the twelve lines of the pause menu's status page
# (label string, g_AbilityLevels index); levels 0-3, the TAIL counter can reach 4.
ABILITIES = [
    ("Fire", 4), ("Ice", 5), ("Tail", 2), ("Horn", 1), ("Chi", 3), ("Shield", 6),
    ("Jump", 8), ("Crit Strike", 10), ("Gen. Magic", 11), ("Atk. Magic", 12),
    ("Def. Magic", 13), ("Charge", 7),
]
ABILITY_MAX = {2: 4}   # others: 3
DEFAULT_ABILITY_MAX = 3

# ---- quests
# The game keeps one "quest marker" byte per quest at g_ScriptVars[0x1C2 + i]:
# 1 while the quest is active (it puts an icon on the touch-screen map), 0 both
# before it starts and after it ends. Whether a quest was finished is a separate
# story flag the quest's script sets next to clearing the marker; those were read
# out of the level scripts (*.app files) and are listed here as `done`. `start`
# flags are set when the quest is handed out. Where no done flag is known the
# tracker falls back to what it has seen happen (active -> cleared).
# A done entry is a flag number, or (counter, target): the rescue quests count
# rescues in the byte variables 0x200+ and are done when the count reaches the target.
# Quest #52 is left out: it can't be started (see UNUSED_QUESTS).
QUEST_MARKER_BASE = 0x1C2

# (index, area, name, start flags, done flags, adds to the reward counter +0x12F)
QUESTS = [
    (0, "Dragon Shores", "Rescue 6 dragons", [0x02], [0x104, (0x200, 6)], False),
    (1, "Dragon Shores", "Retrieve lotion from Laird's room", [0x12], [0x13], False),
    (2, "Dragon Shores", "Find fountain talisman for Fonsworthy", [0xD1], [0xD5], True),
    (3, "Dragon Shores", "Deliver invite to Mrs Funderlic from Brigid", [0xD2], [0xD3], False),
    (4, "Dragon Shores", "Clean up inn for Artigan", [0x10], [0x11], False),
    (5, "Dragon Shores", "Meet Hunter at the dock", [], [0x0E], False),
    (6, "Dragon Shores", "Find Bianca", [0x0E], [0x0F], False),
    (7, "Dragonfly Dojo", "Help Tomas", [], [0x14], False),
    (8, "Dragonfly Dojo", "Rescue Sensei", [0x22], [(0x209, 2)], False),
    (9, "Dragonfly Dojo", "Rescue 4 Elders", [0x25], [(0x202, 6)], False),
    (10, "Dragon Village", "Rescue 6 Dragons", [], [(0x206, 6)], False),
    (11, "Dragon Village", "Open gate for One Horn", [0x33], [0x34], True),
    (12, "Dragon Village", "Find Zinc's Octopus", [0x2F], [0x30], False),
    (13, "Dragon Village", "Find dragon eggs for Kaitlin", [0x31], [], False),
    (14, "Dragon Village", "Break boulders for Flame", [0x35], [0x36], False),
    (15, "Dragon Village", "Open western gate", [], [0x37], False),
    (16, "Bear Forest", "Rescue 6 bears", [0x3B], [(0x205, 4)], False),
    (17, "Bear Forest", "Power up conveyors for Merton", [0x42], [0x43], True),
    (18, "Bear Forest", "Find bolweevils for Maxwell", [0x44], [0x45], False),
    (19, "Bear Forest", "Clean Mrs. Moneybags' garden", [0x40], [0x41], False),
    (20, "Wizard Peak", "Rescue 5 wizards", [], [(0x21F, 5)], False),
    (21, "Wizard Peak", "Help Pastemolle cast a spell", [0x61], [0x62], True),
    (22, "Wizard Peak", "Find eyeball for Pullosin", [0x5F], [0x60], False),
    (23, "Wizard Peak", "Free minidragons for Longorin", [0x5B], [0x5C], False),
    (24, "Cliff Town", "Rescue 5 armadillos", [], [0x77, (0x203, 5)], False),
    (25, "Cliff Town", "Escort Bandit to Dragon Village", [0x69], [], False),
    (26, "Cliff Town", "Deliver letter to Ember for Bandit", [0x6E], [0x6F], False),
    (27, "Cliff Town", "Get 3 ingredients for Isabel", [0x6C], [0x6D], True),
    (28, "Bear Forest", "Find digging tools for Pablo & Francisco", [0x70], [0x6B], False),
    (29, "Wilderness", "Defeat Shadow Minion (ice, 1st boss)", [], [0x78], False),
    (30, "Savannah", "Rescue 5 cheetahs", [], [(0x20C, 5)], False),
    (31, "Savannah", "Get Hunter out of jail", [0xA3], [0xA5], False),
    (32, "Savannah", "Deliver note to Akello", [0x98], [0xA0], False),
    (33, "Savannah", "Talk to Neema", [0x98], [], False),
    (34, "Savannah", "Deliver medicine to Akello", [0xA0], [0xA2], True),
    (35, "Savannah", "Find medicine ingredients for Adama", [0xA1], [], False),
    (36, "Treetops", "Rescue 4 fairies", [], [(0x20D, 4)], False),
    (37, "Treetops", "Find medicine leaves for fairies", [0xBF], [0xC0], False),
    (38, "Treetops", "Retrieve Zoe's wand for Amp", [0xC3], [0xC4], True),
    (39, "Treetops", "Find ice cream for Trina/Zoe", [0xBD], [0xBE], False),
    (40, "Treetops", "Put out fires for Aqua", [0xC6], [0x131], False),
    (41, "Skelos / Prof. Lab", "Rescue 4 cavemen", [0x79], [0x97, (0x20A, 4)], False),
    (42, "Skelos / Prof. Lab", "Find mask for Gronk", [0x94], [0x93], False),
    (43, "Skelos / Prof. Lab", "Help Lumpy across lava", [], [0x81], False),
    (44, "Skelos / Prof. Lab", "Build altar for Dirt", [0x82], [0x83], False),
    (45, "Skelos / Prof. Lab", "Rescue Professor", [], [0x8C], False),
    (46, "Savannah", "Defeat Shadow Minion (fire, 2nd boss)", [0xA4], [0x8C], False),
    (47, "Lost Fleet", "Rescue 4 bunnies", [], [(0x20E, 4)], False),
    (48, "Lost Fleet", "Drain fields for Bruno", [0xDF], [], True),
    (49, "Lost Fleet", "Zap chickens for Lucia", [0xDA], [0xDB], False),
    (50, "Lost Fleet", "Help Teresa down", [0xDC], [0xDD], False),
    (51, "Moleville", "Rescue 3 moles", [], [0x103, (0x20F, 3)], False),
    (53, "Moleville", "Get sunglasses for Willard", [0xFB], [0xFC], False),
    (54, "Moleville", "Find magic detector for Sis", [0xFE], [0xFF], False),
    (55, "Moleville", "Find Bianca", [], [0xFD], False),
    (56, "Moleville", "Defeat Red!", [0x102], [0xFD], True),
    (57, "Sorcerer's Land", "Defeat the Sorcerer!", [], [0x100], False),
    # 58 "Defeat Shadow Minion" exists in the quest text table, but no script
    # ever sets its marker: it is unused, so it is not counted.
]
UNUSED_QUESTS = [
    (52, "Help Willard get home (unused: the torch quest's scripts are never called, see RESEARCH.md)"),
    (58, "Defeat Shadow Minion (unused: never given by any script)"),
]

OVERWORLD_EGG_TARGET = 40   # the category's current rule; 41 are obtainable (see RESEARCH.md)

# ---- The Hole (level 0x19): 28 holes in one map, each an object group that is
# spawned only when you come down that hole. Leaving the map auto-saves it and
# writes every saved object of the other, unloaded groups as gone, so only the
# FIRST hole you ever enter keeps its egg. One Hole egg is obtainable per file.
HOLE_LEVEL = 0x19
HOLE_EGG_GROUPS = {   # object group -> (hole number, where its entrance is)
    22: (21, "Hermit Cave (Dragon Mouth Cave)"),
    23: (22, "Wilderness, Forgotten Worlds (chest)"),
    26: (25, "Dragon Shores / Dragon Shores Cave"),
    27: (26, "Wilderness, Ice boss area"),
    28: (27, "Wilderness, Avalar"),
}

KAITLIN_EGG_RETURNED = 0x32   # set when you give Kaitlin her lost egg back (the egg count drops by 1)

# ---- eggs handed over by scripts (GiveOneItem 25), with the quest flag set alongside
NPC_EGGS = [
    ("Laird (Suntan Lotion)", 0x13),
    ("Aqua (put out fires)", 0x131),
    ("Dirt (build altar)", 0x83),
    ("Lucia (zap chickens)", 0xDB),
]
EGGS_OBTAINABLE = 36 + 1 + len(NPC_EGGS)   # placed outside the Hole + 1 Hole egg + NPC eggs = 41
