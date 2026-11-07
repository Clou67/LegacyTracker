# Shadow Legacy 100% Tracker

An automatic 100% tracker for **Spyro: Shadow Legacy** (DS, USA). It reads the
game's memory while you play, either in **melonDS** or in the **projShadow PC
port** (`spShadowLegacy.exe`), and shows the 100% rules live:

* every quest (58), split into to do / active / done
* every egg, level by level: 41 obtainable (36 placed, 1 from The Hole, 4 from NPCs)
* The Hole's one-egg trap: it warns you live if you're in a hole without an egg
* each upgrade tier (wallet, butterfly bottle, mushroom bag, keychain)
* ability levels, hero level and XP

It needs only Python 3.8+ (64-bit) and nothing else to install.

## Use

```sh
python -m tracker              # start melonDS or the PC port first, then this
```

Open http://localhost:8765, or add it to OBS as a **Browser Source**. Add
`?transparent=1&hidedone=1` to the URL to get a see-through background and to
hide finished quests.

| Option | |
|---|---|
| `--save FILE [--slot N]` | read a save file (melonDS `.sav`, the PC port's `spyro.sav`) instead of the running game; it refreshes whenever the file changes |
| `--text` | print to the console instead of serving the page (`--save FILE --text --once` prints once) |
| `--pid N` | pick the emulator process yourself |
| `--run NAME` | name for this run's quest history (see below); use a new name for each attempt, or `--reset-history` |

On Windows, `run_tracker.bat` starts it. On Linux, reading another program's
memory may need `sudo sysctl kernel.yama.ptrace_scope=0`.

## What "seen" means on a quest

The game only marks a quest while it is **active**. After you finish it, the
mark is cleared again. For 50 quests the game also sets a "finished" story
flag, and the tracker reads that. For the other 8 (mostly the "Rescue N …"
quests) no such flag is known. Those count as done once the tracker has seen
them active and then cleared, which it remembers in `tracker_history.json`.
They are tagged `seen` in the list. Reading a save file can't show these 8
unless the tracker watched them happen.

## The Hole

Only the first hole you ever go down keeps its egg (details in
[research/RESEARCH.md](research/RESEARCH.md#the-hole-why-only-one-egg)). For
100%, make your first hole one of the five egg holes. Hermit Cave (the Dragon
Mouth Cave) is the early one. The tracker shows The Hole as one egg, and
shows a red warning while you're inside a hole with no egg.

## How it works

See [research/RESEARCH.md](research/RESEARCH.md) for where the game keeps each
piece of progress and how that was found. `research/extract.py` re-derives the
quest data from your own ROM. The tracker uses no game code or data, only
addresses and flag numbers.

## Tests

```sh
python -m unittest discover tests        # SSL_ROM=path/to/rom.nds also checks the memory signatures
```
