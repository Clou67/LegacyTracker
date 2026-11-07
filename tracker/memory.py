"""Read the game's RAM out of a running melonDS or projShadow PC port.

Both keep the DS's 4 MB of main RAM in one block of their own memory. We find
that block by searching for two pieces of the game's code (gamedata.SIGNATURES)
and then read DS addresses relative to it. The PC port maps DS memory at its
original addresses, so the same search simply finds it at 0x02000000.
"""
import ctypes
import os
import sys

from . import gamedata as G

PROCESS_NAMES = ("melonds", "spshadowlegacy")


class MemoryError_(Exception):
    pass


# ---------------------------------------------------------------- Windows
if sys.platform == "win32":
    from ctypes import wintypes

    k32 = ctypes.WinDLL("kernel32", use_last_error=True)
    PROCESS_VM_READ = 0x0010
    PROCESS_QUERY_INFORMATION = 0x0400
    MEM_COMMIT = 0x1000
    PAGE_NOACCESS = 0x01
    PAGE_GUARD = 0x100

    class MBI(ctypes.Structure):
        _fields_ = [("BaseAddress", ctypes.c_void_p), ("AllocationBase", ctypes.c_void_p),
                    ("AllocationProtect", wintypes.DWORD), ("PartitionId", wintypes.WORD),
                    ("RegionSize", ctypes.c_size_t), ("State", wintypes.DWORD),
                    ("Protect", wintypes.DWORD), ("Type", wintypes.DWORD)]

    class PROCESSENTRY32(ctypes.Structure):
        _fields_ = [("dwSize", wintypes.DWORD), ("cntUsage", wintypes.DWORD),
                    ("th32ProcessID", wintypes.DWORD), ("th32DefaultHeapID", ctypes.c_void_p),
                    ("th32ModuleID", wintypes.DWORD), ("cntThreads", wintypes.DWORD),
                    ("th32ParentProcessID", wintypes.DWORD), ("pcPriClassBase", ctypes.c_long),
                    ("dwFlags", wintypes.DWORD), ("szExeFile", ctypes.c_char * 260)]

    k32.OpenProcess.restype = wintypes.HANDLE
    k32.ReadProcessMemory.argtypes = [wintypes.HANDLE, ctypes.c_void_p, ctypes.c_void_p,
                                      ctypes.c_size_t, ctypes.POINTER(ctypes.c_size_t)]
    k32.VirtualQueryEx.argtypes = [wintypes.HANDLE, ctypes.c_void_p, ctypes.POINTER(MBI), ctypes.c_size_t]
    k32.VirtualQueryEx.restype = ctypes.c_size_t
    k32.CreateToolhelp32Snapshot.restype = wintypes.HANDLE

    def list_processes():
        snap = k32.CreateToolhelp32Snapshot(0x2, 0)
        e = PROCESSENTRY32()
        e.dwSize = ctypes.sizeof(e)
        out = []
        ok = k32.Process32First(snap, ctypes.byref(e))
        while ok:
            out.append((e.th32ProcessID, e.szExeFile.decode(errors="replace")))
            ok = k32.Process32Next(snap, ctypes.byref(e))
        k32.CloseHandle(snap)
        return out

    class Process:
        def __init__(self, pid):
            self.pid = pid
            self.h = k32.OpenProcess(PROCESS_VM_READ | PROCESS_QUERY_INFORMATION, False, pid)
            if not self.h:
                raise MemoryError_(f"can't open process {pid} (error {ctypes.get_last_error()})")

        def read(self, addr, n):
            buf = ctypes.create_string_buffer(n)
            got = ctypes.c_size_t()
            if not k32.ReadProcessMemory(self.h, ctypes.c_void_p(addr), buf, n, ctypes.byref(got)) or got.value != n:
                raise MemoryError_(f"read failed at {addr:#x}")
            return buf.raw

        def regions(self):
            addr = 0
            mbi = MBI()
            while k32.VirtualQueryEx(self.h, ctypes.c_void_p(addr), ctypes.byref(mbi), ctypes.sizeof(mbi)):
                size = mbi.RegionSize
                base = mbi.BaseAddress or 0
                if (mbi.State == MEM_COMMIT and not (mbi.Protect & (PAGE_NOACCESS | PAGE_GUARD))):
                    yield base, size
                addr = base + size
                if addr >= (1 << 47):
                    break

        def close(self):
            k32.CloseHandle(self.h)

# ---------------------------------------------------------------- Linux (melonDS native, or Wine)
else:
    def list_processes():
        out = []
        for p in os.listdir("/proc"):
            if p.isdigit():
                try:
                    with open(f"/proc/{p}/cmdline", "rb") as f:
                        cmd = f.read().split(b"\0")[0].decode(errors="replace")
                    out.append((int(p), os.path.basename(cmd.replace("\\", "/"))))
                except OSError:
                    pass
        return out

    class Process:
        def __init__(self, pid):
            self.pid = pid
            try:
                self.f = open(f"/proc/{pid}/mem", "rb", buffering=0)
            except OSError as e:
                raise MemoryError_(f"can't open process {pid}: {e} (try running as the same user, "
                                   "or: sudo sysctl kernel.yama.ptrace_scope=0)")

        def read(self, addr, n):
            try:
                self.f.seek(addr)
                b = self.f.read(n)
            except (OSError, OverflowError, ValueError) as e:
                raise MemoryError_(f"read failed at {addr:#x}: {e}")
            if len(b) != n:
                raise MemoryError_(f"short read at {addr:#x}")
            return b

        def regions(self):
            with open(f"/proc/{self.pid}/maps") as f:
                for line in f:
                    parts = line.split()
                    if "r" not in parts[1]:
                        continue
                    a, b = (int(x, 16) for x in parts[0].split("-"))
                    if len(parts) > 5 and parts[5].startswith("[v"):  # vvar/vsyscall
                        continue
                    yield a, b - a

        def close(self):
            self.f.close()


def find_game_process(pid=None):
    if pid:
        return pid, "pid %d" % pid
    for p, name in list_processes():
        low = name.lower()
        if any(n in low for n in PROCESS_NAMES):
            return p, name
    raise MemoryError_("no melonDS or spShadowLegacy process found")


def find_ram_base(proc, chunk=8 << 20):
    """Host address of DS address 0x02000000, found by the code signatures."""
    sig_addr, sig = G.SIGNATURES[0]
    check_addr, check = G.SIGNATURES[1]
    off = sig_addr - G.DS_MAIN
    for base, size in proc.regions():
        if size < len(sig):
            continue
        pos = 0
        while pos < size:
            n = min(chunk, size - pos) + len(sig)
            try:
                data = proc.read(base + pos, min(n, size - pos))
            except MemoryError_:
                break
            i = data.find(sig)
            while i != -1:
                cand = base + pos + i - off
                try:
                    if proc.read(cand + check_addr - G.DS_MAIN, len(check)) == check:
                        return cand
                except MemoryError_:
                    pass
                i = data.find(sig, i + 1)
            pos += chunk
    raise MemoryError_("game RAM not found (is the game running?)")


class GameMemory:
    """Reads DS addresses from the game's process."""

    def __init__(self, pid=None):
        pid, self.name = find_game_process(pid)
        self.proc = Process(pid)
        self.base = find_ram_base(self.proc)

    def read(self, ds_addr, n):
        return self.proc.read(self.base + ds_addr - G.DS_MAIN, n)

    def still_valid(self):
        try:
            sig_addr, sig = G.SIGNATURES[0]
            return self.read(sig_addr, len(sig)) == sig
        except MemoryError_:
            return False

    def snapshot(self):
        from .progress import Snapshot
        import struct
        slot = struct.unpack("<i", self.read(G.ADDR_CURRENT_SLOT, 4))[0]
        if not 0 <= slot <= 2:
            slot = 0
        hdr = self.read(G.ADDR_SLOT_HEADERS + slot * 0x20, 0x20)
        return Snapshot(hdr, self.read(G.ADDR_SCRIPT_VARS, 0x240),
                        self.read(G.ADDR_INVENTORY, 0x130),
                        self.read(G.ADDR_ABILITIES, 0x14),
                        source=f"{self.name} (slot {slot + 1})"), slot

    def close(self):
        self.proc.close()
