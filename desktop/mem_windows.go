//go:build windows

// Reading the game's RAM out of melonDS or the projShadow PC port (Windows).
// Find the DS main RAM block by two pieces
// of the game's code, then read DS addresses relative to it.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procVirtualQueryEx           = kernel32.NewProc("VirtualQueryEx")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procAttachConsole            = kernel32.NewProc("AttachConsole")
	procGetExitCodeProcess       = kernel32.NewProc("GetExitCodeProcess")
)

func attachConsole() {
	procAttachConsole.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS
	// re-open the standard handles on the parent's console
	if h, err := syscall.Open("CONOUT$", syscall.O_RDWR, 0); err == nil {
		os.Stdout = os.NewFile(uintptr(h), "CONOUT$")
		os.Stderr = os.Stdout
	}
}

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

var processNames = []string{"melonds", "spshadowlegacy"}

func findGameProcess() (uint32, string, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x2, 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return 0, "", errors.New("can't list processes")
	}
	defer procCloseHandle.Call(snap)
	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for ok != 0 {
		name := syscall.UTF16ToString(e.ExeFile[:])
		low := strings.ToLower(name)
		for _, n := range processNames {
			if strings.Contains(low, n) {
				return e.ProcessID, name, nil
			}
		}
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return 0, "", errors.New("Waiting for melonDS or spShadowLegacy.exe to start...")
}

type GameMemory struct {
	h          uintptr
	name       string
	base       uintptr
	watchLevel int
	entryEggs  int
}

func (m *GameMemory) readHost(addr uintptr, n int) ([]byte, error) {
	buf := make([]byte, n)
	var got uintptr
	r, _, _ := procReadProcessMemory.Call(m.h, addr, uintptr(unsafe.Pointer(&buf[0])), uintptr(n), uintptr(unsafe.Pointer(&got)))
	if r == 0 || int(got) != n {
		return nil, fmt.Errorf("read failed at %#x", addr)
	}
	return buf, nil
}

func (m *GameMemory) read(ds uint32, n int) ([]byte, error) {
	return m.readHost(m.base+uintptr(ds-DsMain), n)
}

func (m *GameMemory) findRAM() error {
	sig, chk := signatures[0], signatures[1]
	off := uintptr(sig.addr - DsMain)
	var addr uintptr
	var mbi memoryBasicInformation
	const chunk = 8 << 20
	for {
		r, _, _ := procVirtualQueryEx.Call(m.h, addr, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
		if r == 0 {
			break
		}
		base, size := mbi.BaseAddress, mbi.RegionSize
		if mbi.State == 0x1000 && mbi.Protect&(0x01|0x100) == 0 && size >= uintptr(len(sig.bytes)) {
			for pos := uintptr(0); pos < size; pos += chunk {
				n := size - pos
				if n > chunk+uintptr(len(sig.bytes)) {
					n = chunk + uintptr(len(sig.bytes))
				}
				data, err := m.readHost(base+pos, int(n))
				if err != nil {
					break
				}
				for i := 0; ; {
					j := bytes.Index(data[i:], sig.bytes)
					if j < 0 {
						break
					}
					cand := base + pos + uintptr(i+j) - off
					if c, err := m.readHost(cand+uintptr(chk.addr-DsMain), len(chk.bytes)); err == nil && bytes.Equal(c, chk.bytes) {
						m.base = cand
						return nil
					}
					i += j + 1
				}
			}
		}
		next := base + size
		if next <= addr {
			break
		}
		addr = next
	}
	return errors.New("melonDS is running, but the game isn't loaded yet")
}

func OpenGame() (*GameMemory, error) {
	pid, name, err := findGameProcess()
	if err != nil {
		return nil, err
	}
	h, _, _ := procOpenProcess.Call(0x0010|0x0400, 0, uintptr(pid)) // VM_READ | QUERY_INFORMATION
	if h == 0 {
		return nil, fmt.Errorf("can't open %s (try running the tracker as administrator)", name)
	}
	m := &GameMemory{h: h, name: name, watchLevel: -2}
	if err := m.findRAM(); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

func (m *GameMemory) Close() { procCloseHandle.Call(m.h) }

func (m *GameMemory) StillValid() bool {
	var code uint32
	procGetExitCodeProcess.Call(m.h, uintptr(unsafe.Pointer(&code)))
	if code != 259 { // STILL_ACTIVE
		return false
	}
	b, err := m.read(signatures[0].addr, len(signatures[0].bytes))
	return err == nil && bytes.Equal(b, signatures[0].bytes)
}

func (m *GameMemory) Snapshot() (*Snapshot, int, error) {
	rd := func(a uint32, n int) []byte {
		b, err := m.read(a, n)
		if err != nil {
			panic(err)
		}
		return b
	}
	var snap *Snapshot
	var slot int
	err := func() (err error) {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("%v", e)
			}
		}()
		slot = int(int32(binary.LittleEndian.Uint32(rd(AddrCurrentSlot, 4))))
		if slot < 0 || slot > 2 {
			slot = 0
		}
		hdr := rd(AddrSlotHeaders+uint32(slot)*0x20, 0x20)
		level := int(int32(binary.LittleEndian.Uint32(rd(AddrCurrentMap, 4))))
		if level < 0 || level >= 0x29 {
			level = -1
		}
		inv := rd(AddrInventory, 0x130)
		held := int(inv[InvEggs])
		if level != m.watchLevel {
			m.watchLevel, m.entryEggs = level, held
		}
		if held < m.entryEggs {
			m.entryEggs = held
		}
		var stream []byte
		if p, err := m.read(AddrSaveStream, 4); err == nil {
			buf := binary.LittleEndian.Uint32(p)
			sz, _ := m.read(AddrSaveStream+0xe, 2)
			if sz != nil {
				n := int(binary.LittleEndian.Uint16(sz))
				if n > 0 && buf >= DsMain && buf+uint32(n) <= DsMain+DsMainSize {
					stream, _ = m.read(buf, n)
				}
			}
		}
		snap = &Snapshot{Header: hdr, Vars: rd(AddrScriptVars, 0x240), Inv: inv, Abilities: rd(AddrAbilities, 0x14),
			Stream: stream, Level: level, Pending: held - m.entryEggs,
			Source: fmt.Sprintf("%s (slot %d)", m.name, slot+1)}
		return nil
	}()
	return snap, slot, err
}
