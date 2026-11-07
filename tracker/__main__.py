"""Spyro: Shadow Legacy 100% tracker.

  python -m tracker                 watch melonDS / the PC port, overlay on http://localhost:8765
  python -m tracker --save FILE     show progress from a save file (melonDS .sav or spyro.sav)
  python -m tracker --text          print to the console instead of serving the page
"""
import argparse
import json
import os
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import progress
from .memory import GameMemory, MemoryError_
from .savefile import read_save

HERE = os.path.dirname(os.path.abspath(__file__))


class State:
    def __init__(self):
        self.lock = threading.Lock()
        self.result = {"status": "starting"}
        self.version = 0

    def set(self, r):
        with self.lock:
            if r != self.result:
                self.result = r
                self.version += 1


def watch_memory(state, args, history):
    mem = None
    last = None
    while True:
        try:
            if mem is None or not mem.still_valid():
                if mem:
                    mem.close()
                mem = None
                mem = GameMemory(args.pid)
            snap, slot = mem.snapshot()
            if snap.key() != last:
                last = snap.key()
                r = progress.evaluate(snap, history, run=f"{args.run}:slot{slot + 1}")
                r["status"] = "live"
                state.set(r)
        except MemoryError_ as e:
            mem = None
            last = None
            state.set({"status": "waiting", "message": str(e)})
            time.sleep(2)
            continue
        time.sleep(args.interval)


def watch_save(state, args, history):
    mtime = None
    while True:
        try:
            m = os.path.getmtime(args.save)
            if m != mtime:
                mtime = m
                slots = read_save(args.save, args.slot - 1 if args.slot else None)
                idx, snap = slots[0]
                r = progress.evaluate(snap, history, run=f"{os.path.basename(args.save)}:slot{idx + 1}")
                r["status"] = "save"
                r["slots"] = [i + 1 for i, _ in slots]
                state.set(r)
        except (OSError, ValueError) as e:
            state.set({"status": "waiting", "message": str(e)})
        if args.once:
            return
        time.sleep(1)


def text_report(r):
    if r.get("status") not in ("live", "save"):
        return "[%s] %s" % (r.get("status"), r.get("message", ""))
    out = [f"== {r['source']}"]
    for c in r["checklist"]:
        mark = "x" if c["have"] >= c["need"] else " "
        out.append(f" [{mark}] {c['rule']}: {c['have']}/{c['need']}")
    lv = r["level"]
    out.append(f"\nLevel {lv['level']}  XP {lv['xp']}" + (f"/{lv['next_xp']}" if lv["next_xp"] else ""))
    e = r["eggs"]
    out.append(f"Eggs: {e['total']} total = {e['overworld']} overworld (est.) + {e['from_quests']} quest rewards")
    out.append("Upgrades:")
    for u in r["upgrades"]:
        tiers = "".join("#" if t else "-" for t in u["tiers"])
        out.append(f"  {u['name']:<17} [{tiers}] capacity {u['capacity']}/{u['max_capacity']}")
    out.append("Abilities:")
    out.append("  " + ", ".join(f"{a['name']} {a['level']}/{a['max']}" for a in r["abilities"]))
    out.append(f"Quests {r['quests_done']}/{r['quests_total']}:")
    sym = {"done": "x", "active": ">", "started": "?", "todo": " "}
    area = None
    for q in r["quests"]:
        if q["area"] != area:
            area = q["area"]
            out.append(f"  {area}")
        how = f" ({q['how']})" if q["how"] in ("seen", "inferred") else ""
        out.append(f"    [{sym[q['state']]}] {q['name']}{how}")
    return "\n".join(out)


def make_handler(state, history, args):
    class H(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def send(self, code, body, ctype):
            self.send_response(code)
            self.send_header("Content-Type", ctype)
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            if self.path.startswith("/state"):
                with state.lock:
                    body = json.dumps(dict(state.result, version=state.version)).encode()
                self.send(200, body, "application/json")
            elif self.path in ("/", "/index.html") or self.path.startswith("/?"):
                with open(os.path.join(HERE, "web", "index.html"), "rb") as f:
                    self.send(200, f.read(), "text/html; charset=utf-8")
            else:
                self.send(404, b"not found", "text/plain")

    return H


def main(argv=None):
    ap = argparse.ArgumentParser(prog="tracker", description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--save", help="read a save file instead of the running game")
    ap.add_argument("--slot", type=int, choices=[1, 2, 3], help="save slot (with --save)")
    ap.add_argument("--pid", type=int, help="process id of melonDS / the PC port")
    ap.add_argument("--port", type=int, default=8765)
    ap.add_argument("--interval", type=float, default=0.25, help="seconds between reads")
    ap.add_argument("--text", action="store_true", help="print to the console")
    ap.add_argument("--once", action="store_true", help="with --save --text: print once and exit")
    ap.add_argument("--run", default="run", help="name for this run's quest history")
    ap.add_argument("--history", default=os.path.join(os.getcwd(), "tracker_history.json"))
    ap.add_argument("--reset-history", action="store_true")
    args = ap.parse_args(argv)

    history = progress.History(args.history)
    if args.reset_history:
        history.data = {}
        history.reset("")
    state = State()
    worker = watch_save if args.save else watch_memory
    t = threading.Thread(target=worker, args=(state, args, history), daemon=True)
    t.start()

    if args.text:
        if args.once:
            t.join()
            print(text_report(state.result))
            return
        seen = -1
        try:
            while True:
                if state.version != seen:
                    seen = state.version
                    print("\033[2J\033[H" + text_report(state.result), flush=True)
                time.sleep(0.2)
        except KeyboardInterrupt:
            return

    srv = ThreadingHTTPServer(("127.0.0.1", args.port), make_handler(state, history, args))
    print(f"Tracker on http://localhost:{args.port}  (add it to OBS as a Browser Source)")
    print("Watching " + (args.save if args.save else "for melonDS / spShadowLegacy.exe") + " - Ctrl+C to stop")
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
