# Shadow Legacy 100% Tracker

An automatic 100% tracker for **Spyro: Shadow Legacy** (DS, USA). It reads the
game's memory while you play, either in **melonDS** or in the **projShadow PC
port** (`spShadowLegacy.exe`), and shows the 100% rules live:

* every quest (57; two of the game's 59 quest texts are unused), split into to do / active / done
* every egg, level by level: 41 obtainable (36 placed, 1 from The Hole, 4 from NPCs)
* The Hole's one-egg trap: it warns you live if you're in a hole without an egg
* each upgrade tier (wallet, butterfly bottle, mushroom bag, keychain)
* ability levels, hero level and XP



## Desktop app (Windows)

`SSLTracker.exe` is the tracker in its own window: no browser, no Python,
nothing to install. Double-click it.

* **File > Watch the game**: follows melonDS or `spShadowLegacy.exe` while you play (the default).
* **File > Open save file...**: reads a melonDS `.sav` or the PC port's `spyro.sav`. It updates by itself
  whenever the game saves, which happens on every map change.
* **View**: *Hide finished*, *Compact* (just the checklist and eggs, good next to the game), *Always on top*.
* Scroll with the mouse wheel if the window is smaller than the list.

It keeps `tracker_history.json` next to itself (quests and the Hole egg it has seen done live).
`SSLTracker.exe -text -save FILE` prints the same report in a console.

Build it yourself with `desktop/build.sh` (needs only Go).

## The Hole

Only the first hole you ever go down keeps its egg (details in
[research/RESEARCH.md](research/RESEARCH.md#the-hole-why-only-one-egg)). For
100%, make your first hole one of the five egg holes. Hermit Cave (the Dragon
Mouth Cave) is the early one. The tracker shows The Hole as one egg, and
shows a red warning while you're inside a hole with no egg.

## How it works

See [research/RESEARCH.md](research/RESEARCH.md) for where the game keeps each
piece of progress and how that was found. The tracker uses no game code or data, only
addresses and flag numbers.

## Tests

```sh
cd desktop && go test .
```
