# Shadow Legacy 100% Tracker

An automatic 100% tracker for **Spyro: Shadow Legacy** (DS, USA). It reads the
game's memory while you play, either in **melonDS** or in the **projShadow PC
port** (`spShadowLegacy.exe`), and shows the 100% rules live:

* every quest (57; two of the game's 59 quest texts are unused), split into to do / active / done
* every egg, level by level: 41 obtainable (36 placed, 1 from The Hole, 4 from NPCs)
* The Hole's one-egg trap: it warns you live if you're in a hole without an egg
* each upgrade tier (wallet, butterfly bottle, mushroom bag, keychain)
* ability levels, hero level and XP

It needs only Python 3.8+ (64-bit) and nothing else to install.

## Desktop app (Windows)

`SSLTracker.exe` is the tracker in its own window: no browser, no Python,
nothing to install. Double-click it.

* **File > Watch the game**: follows melonDS or `spShadowLegacy.exe` while you play (the default).
* **File > Open save file...**: reads a melonDS `.sav` or the PC port's `spyro.sav`. It updates by itself
  whenever the game saves, which happens on every map change.
* **View**: *Hide finished*, *Compact* (just the checklist and eggs, good next to the game), *Always on top*.
* **View > Game theme**: the XP screen background and the game's own pixel font. The first time,
  it asks for your `Spyro - Shadow Legacy (USA).nds`; it reads the picture and fonts from that file every
  time it starts. Nothing from the game is stored in the tracker. Use *Choose ROM for the theme...* if
  you move the ROM.
* Scroll with the mouse wheel if the window is smaller than the list.

It keeps `tracker_history.json` (quests and the Hole egg it has seen done live) and
`tracker_settings.json` (your View choices and where your ROM is) next to itself.
`SSLTracker.exe -text -save FILE` prints the same report in a console.

Build it yourself with `desktop/build.sh` (needs only Go). It's a port of the Python tracker, and the tests
check that both give identical reports on real saves.

## Python version (browser overlay, e.g. for OBS)

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

## How quests are read

The game only marks a quest while it is **active**, then clears the mark.
Whether a quest is finished comes from a separate story flag, or for the
"Rescue N …" quests from a rescue counter reaching its target. That works
from a save file too. One quest (Kaitlin's eggs) has neither: it counts as
done once it was started and its mark cleared (tagged `inferred`).

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
