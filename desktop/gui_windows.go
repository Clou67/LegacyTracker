//go:build windows

// The tracker window: plain Win32 (user32/gdi32/comdlg32), no libraries.
package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")

	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pFillRect            = user32.NewProc("FillRect")
	pDrawTextW           = user32.NewProc("DrawTextW")
	pLoadCursorW         = user32.NewProc("LoadCursorW")
	pLoadIconW           = user32.NewProc("LoadIconW")
	pCreateMenu          = user32.NewProc("CreateMenu")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenuW         = user32.NewProc("AppendMenuW")
	pSetMenu             = user32.NewProc("SetMenu")
	pCheckMenuItem       = user32.NewProc("CheckMenuItem")
	pSetWindowPos        = user32.NewProc("SetWindowPos")
	pSetWindowTextW      = user32.NewProc("SetWindowTextW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pFrameRect           = user32.NewProc("FrameRect")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject        = gdi32.NewProc("SelectObject")
	pDeleteObject        = gdi32.NewProc("DeleteObject")
	pDeleteDC            = gdi32.NewProc("DeleteDC")
	pBitBlt              = gdi32.NewProc("BitBlt")
	pCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW         = gdi32.NewProc("CreateFontW")
	pSetTextColor        = gdi32.NewProc("SetTextColor")
	pSetBkMode           = gdi32.NewProc("SetBkMode")
	pGetDeviceCaps       = gdi32.NewProc("GetDeviceCaps")
	pGetOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")
	pGetModuleHandleW    = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

const (
	wmDestroy    = 0x0002
	wmSize       = 0x0005
	wmPaint      = 0x000F
	wmClose      = 0x0010
	wmEraseBkgnd = 0x0014
	wmCommand    = 0x0111
	wmMouseWheel = 0x020A
	wmApp        = 0x8000

	idWatch   = 101
	idOpen    = 102
	idExit    = 103
	idHide    = 201
	idTop     = 202
	idCompact = 203
	idReset   = 301
	idAbout   = 302
)

type rect struct{ L, T, R, B int32 }
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
	_       uint32
}
type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}
type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	Reserved2     uint32
	FlagsEx       uint32
}

func u16p(s string) *uint16 { return &utf16z(s)[0] }

// ---------------------------------------------------------------- state

type appState struct {
	mu       sync.Mutex
	res      *Result
	status   string
	live     bool
	savePath string
	slot     int
	hide     bool
	top      bool
	compact  bool
	scroll   int
	contentH int
	hwnd     uintptr
	hist     *History
	wake     chan struct{}
}

var app = &appState{wake: make(chan struct{}, 1)}

func (a *appState) set(r *Result, status string) {
	a.mu.Lock()
	a.res, a.status = r, status
	a.mu.Unlock()
	if a.hwnd != 0 {
		pPostMessageW.Call(a.hwnd, wmApp, 0, 0)
	}
}

func (a *appState) source() (string, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.savePath, a.slot
}

// worker: reads the game or the save file and evaluates it. If something
// unexpected goes wrong, it logs it, shows it in the window and starts over.
func worker() {
	for {
		func() {
			defer func() {
				if e := recover(); e != nil {
					p := logCrash(e)
					app.set(nil, fmt.Sprintf("Error: %v (details in %s)", e, filepath.Base(p)))
					time.Sleep(2 * time.Second)
				}
			}()
			workerLoop()
		}()
	}
}

func workerLoop() {
	var mem *GameMemory
	lastKey, lastSave := "", ""
	var lastMod time.Time
	for {
		save, slot := app.source()
		if save != "" {
			if mem != nil {
				mem.Close()
				mem = nil
			}
			st, err := statMod(save)
			if err != nil {
				app.set(nil, "Can't open "+save)
			} else if save != lastSave || !st.Equal(lastMod) {
				lastSave, lastMod = save, st
				snap, idx, err := ReadSave(save, slot-1)
				if err != nil {
					app.set(nil, err.Error())
				} else {
					r := Evaluate(snap, app.hist, fmt.Sprintf("%s:slot%d", filepath.Base(save), idx+1))
					app.set(r, "Save file: "+filepath.Base(save)+fmt.Sprintf(" (slot %d) - updates when the game saves", idx+1))
				}
			}
			wait(time.Second)
			continue
		}
		lastSave = ""
		if mem == nil || !mem.StillValid() {
			if mem != nil {
				mem.Close()
			}
			m, err := OpenGame()
			if err != nil {
				mem = nil
				lastKey = ""
				app.set(nil, err.Error())
				wait(2 * time.Second)
				continue
			}
			mem = m
		}
		snap, slot, err := mem.Snapshot()
		if err != nil {
			mem.Close()
			mem = nil
			continue
		}
		if k := snap.Key(); k != lastKey {
			lastKey = k
			r := Evaluate(snap, app.hist, fmt.Sprintf("%s:slot%d", *flagRun, slot+1))
			app.set(r, "Live: "+snap.Source)
		}
		wait(250 * time.Millisecond)
	}
}

func wait(d time.Duration) {
	select {
	case <-app.wake:
	case <-time.After(d):
	}
}

func poke() {
	select {
	case app.wake <- struct{}{}:
	default:
	}
}

// ---------------------------------------------------------------- drawing

type painter struct {
	dc      uintptr
	scale   float64
	brushes map[uintptr]uintptr
	fonts   map[string]uintptr
	dy      int // scroll offset
}

func (p *painter) px(v int) int32 { return int32(float64(v) * p.scale) }

func (p *painter) brush(c uintptr) uintptr {
	if b, ok := p.brushes[c]; ok {
		return b
	}
	b, _, _ := pCreateSolidBrush.Call(c)
	p.brushes[c] = b
	return b
}

func (p *painter) font(size int, bold bool) uintptr {
	key := fmt.Sprint(size, bold)
	if f, ok := p.fonts[key]; ok {
		return f
	}
	w := 400
	if bold {
		w = 600
	}
	f, _, _ := pCreateFontW.Call(uintptr(-p.px(size)), 0, 0, 0, uintptr(w), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16p("Segoe UI"))))
	p.fonts[key] = f
	return f
}

func (p *painter) fill(x, y, w, h int, c uintptr) {
	r := rect{p.px(x), p.px(y - p.dy), p.px(x + w), p.px(y + h - p.dy)}
	pFillRect.Call(p.dc, uintptr(unsafe.Pointer(&r)), p.brush(c))
}

func (p *painter) frame(x, y, w, h int, c uintptr) {
	r := rect{p.px(x), p.px(y - p.dy), p.px(x + w), p.px(y + h - p.dy)}
	pFrameRect.Call(p.dc, uintptr(unsafe.Pointer(&r)), p.brush(c))
}

// text draws s in the box; returns the height used (wrap: word-wrapped)
func (p *painter) text(x, y, w int, s string, c uintptr, size int, bold bool, flags uintptr) int {
	pSelectObject.Call(p.dc, p.font(size, bold))
	pSetTextColor.Call(p.dc, c)
	r := rect{p.px(x), p.px(y - p.dy), p.px(x + w), p.px(y - p.dy + 2000)}
	ws := utf16z(s)
	if flags&dtWordBrk != 0 {
		m := r
		pDrawTextW.Call(p.dc, uintptr(unsafe.Pointer(&ws[0])), uintptr(len(ws)-1), uintptr(unsafe.Pointer(&m)), flags|dtNoPrefix|dtCalcRect)
		pDrawTextW.Call(p.dc, uintptr(unsafe.Pointer(&ws[0])), uintptr(len(ws)-1), uintptr(unsafe.Pointer(&r)), flags|dtNoPrefix)
		return int(float64(m.B-m.T) / p.scale)
	}
	pDrawTextW.Call(p.dc, uintptr(unsafe.Pointer(&ws[0])), uintptr(len(ws)-1), uintptr(unsafe.Pointer(&r)), flags|dtSingle|dtNoPrefix|dtEndEll)
	return size + 6
}

func paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var cr rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	W, H := cr.R-cr.L, cr.B-cr.T
	if W <= 0 || H <= 0 {
		return
	}
	mdc, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(hdc, uintptr(W), uintptr(H))
	old, _, _ := pSelectObject.Call(mdc, bmp)
	dpi, _, _ := pGetDeviceCaps.Call(hdc, 90) // LOGPIXELSY
	p := &painter{dc: mdc, scale: float64(dpi) / 96, brushes: map[uintptr]uintptr{}, fonts: map[string]uintptr{}}
	pSetBkMode.Call(mdc, 1)
	full := rect{0, 0, W, H}
	pFillRect.Call(mdc, uintptr(unsafe.Pointer(&full)), p.brush(cBg))

	app.mu.Lock()
	r, status, hide, compact := app.res, app.status, app.hide, app.compact
	p.dy = app.scroll
	app.mu.Unlock()

	lw := int(float64(W) / p.scale) // logical width
	h := draw(p, r, status, hide, compact, lw)

	app.mu.Lock()
	app.contentH = h
	app.mu.Unlock()

	pBitBlt.Call(hdc, 0, 0, uintptr(W), uintptr(H), mdc, 0, 0, 0x00CC0020)
	pSelectObject.Call(mdc, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mdc)
	for _, b := range p.brushes {
		pDeleteObject.Call(b)
	}
	for _, f := range p.fonts {
		pDeleteObject.Call(f)
	}
}

// ---------------------------------------------------------------- window

var menuView uintptr

func openSaveDialog(hwnd uintptr) string {
	buf := make([]uint16, 1024)
	filter := utf16z("Save files (*.sav)\x00*.sav\x00All files\x00*.*\x00")
	ofn := openFileName{
		StructSize: uint32(unsafe.Sizeof(openFileName{})),
		Owner:      hwnd,
		Filter:     &filter[0],
		File:       &buf[0],
		MaxFile:    uint32(len(buf)),
		Title:      u16p("Open a Spyro: Shadow Legacy save file"),
		Flags:      0x00001000 | 0x00000800 | 0x00080000 | 0x8, // FILEMUSTEXIST | PATHMUSTEXIST | EXPLORER | NOCHANGEDIR
	}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func check(id int, on bool) {
	f := uintptr(0)
	if on {
		f = 8 // MF_CHECKED
	}
	pCheckMenuItem.Call(menuView, uintptr(id), f)
}

func wndProc(hwnd, m, wp, lp uintptr) (ret uintptr) {
	defer func() {
		if e := recover(); e != nil {
			showError(hwnd, e)
			ret = 0
		}
	}()
	switch m {
	case wmPaint:
		paint(hwnd)
		return 0
	case wmEraseBkgnd:
		return 1
	case wmApp, wmSize:
		pInvalidateRect.Call(hwnd, 0, 0)
	case wmMouseWheel:
		delta := int(int16(wp >> 16))
		var cr rect
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		app.mu.Lock()
		app.scroll -= delta / 2
		maxS := app.contentH - int(cr.B-cr.T)
		if maxS < 0 {
			maxS = 0
		}
		if app.scroll > maxS {
			app.scroll = maxS
		}
		if app.scroll < 0 {
			app.scroll = 0
		}
		app.mu.Unlock()
		pInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmCommand:
		switch int(wp & 0xffff) {
		case idWatch:
			app.mu.Lock()
			app.savePath, app.res, app.status = "", nil, "Looking for melonDS / spShadowLegacy.exe..."
			app.mu.Unlock()
			poke()
		case idOpen:
			if f := openSaveDialog(hwnd); f != "" {
				app.mu.Lock()
				app.savePath, app.slot, app.res = f, 0, nil
				app.status = "Reading " + filepath.Base(f) + "..."
				app.mu.Unlock()
				poke()
			}
		case idExit:
			pDestroyWindow.Call(hwnd)
		case idHide:
			app.mu.Lock()
			app.hide = !app.hide
			check(idHide, app.hide)
			app.mu.Unlock()
		case idCompact:
			app.mu.Lock()
			app.compact = !app.compact
			check(idCompact, app.compact)
			app.mu.Unlock()
		case idTop:
			app.mu.Lock()
			app.top = !app.top
			after := uintptr(^uintptr(1)) // HWND_NOTOPMOST (-2)
			if app.top {
				after = ^uintptr(0) // HWND_TOPMOST (-1)
			}
			check(idTop, app.top)
			app.mu.Unlock()
			pSetWindowPos.Call(hwnd, after, 0, 0, 0, 0, 0x0001|0x0002|0x0010)
		case idReset:
			r, _, _ := pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(u16p("Forget the quests and the Hole egg this tracker has seen being done live?\n\n(Only needed when starting a new run with the same save slot.)"))),
				uintptr(unsafe.Pointer(u16p("Reset run history"))), 0x4|0x30)
			if r == 6 { // IDYES
				app.hist.data = map[string]interface{}{}
				app.hist.write()
				poke()
			}
		case idAbout:
			pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(u16p("Spyro: Shadow Legacy 100% tracker\n\nWatches melonDS or the projShadow PC port (spShadowLegacy.exe) while you play, or reads a save file.\n\nThe Hole: only the first hole you ever go down keeps its egg.\n\nSee README.md and research/RESEARCH.md for how everything is read."))),
				uintptr(unsafe.Pointer(u16p("About"))), 0x40)
		}
		pInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ = pDefWindowProcW.Call(hwnd, m, wp, lp)
	return ret
}

// showError reports an unexpected error instead of letting the program vanish.
func showError(hwnd uintptr, e interface{}) {
	log := logCrash(e)
	text := fmt.Sprintf("Something went wrong:\n\n%v\n\nThe tracker keeps running. Details were saved to:\n%s\n\nPlease send that file over.", e, log)
	pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(&utf16z(text)[0])), uintptr(unsafe.Pointer(u16p("Shadow Legacy Tracker"))), 0x30)
}

func runGUI() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()
	app.hist = LoadHistory(historyPath())
	if *flagSave != "" {
		app.savePath, app.slot = *flagSave, *flagSlot
	}
	app.status = "Looking for melonDS / spShadowLegacy.exe..."

	cls := u16p("SSLTracker")
	cursor, _, _ := pLoadCursorW.Call(0, 32512)
	icon, _, _ := pLoadIconW.Call(0, 32512)
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := wndClassEx{WndProc: syscall.NewCallback(wndProc), Instance: inst, Cursor: cursor, Icon: icon, IconSm: icon, ClassName: cls}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	menu, _, _ := pCreateMenu.Call()
	file, _, _ := pCreatePopupMenu.Call()
	pAppendMenuW.Call(file, 0, idWatch, uintptr(unsafe.Pointer(u16p("&Watch the game (melonDS / PC port)"))))
	pAppendMenuW.Call(file, 0, idOpen, uintptr(unsafe.Pointer(u16p("&Open save file..."))))
	pAppendMenuW.Call(file, 0x800, 0, 0)
	pAppendMenuW.Call(file, 0, idReset, uintptr(unsafe.Pointer(u16p("Reset run history..."))))
	pAppendMenuW.Call(file, 0x800, 0, 0)
	pAppendMenuW.Call(file, 0, idExit, uintptr(unsafe.Pointer(u16p("E&xit"))))
	menuView, _, _ = pCreatePopupMenu.Call()
	pAppendMenuW.Call(menuView, 0, idHide, uintptr(unsafe.Pointer(u16p("&Hide finished"))))
	pAppendMenuW.Call(menuView, 0, idCompact, uintptr(unsafe.Pointer(u16p("&Compact (checklist and eggs only)"))))
	pAppendMenuW.Call(menuView, 0, idTop, uintptr(unsafe.Pointer(u16p("Always on &top"))))
	help, _, _ := pCreatePopupMenu.Call()
	pAppendMenuW.Call(help, 0, idAbout, uintptr(unsafe.Pointer(u16p("&About"))))
	pAppendMenuW.Call(menu, 0x10, file, uintptr(unsafe.Pointer(u16p("&File"))))
	pAppendMenuW.Call(menu, 0x10, menuView, uintptr(unsafe.Pointer(u16p("&View"))))
	pAppendMenuW.Call(menu, 0x10, help, uintptr(unsafe.Pointer(u16p("&Help"))))

	// size the window for the screen's DPI
	sdc, _, _ := pGetDC.Call(0)
	dpi, _, _ := pGetDeviceCaps.Call(sdc, 90)
	pReleaseDC.Call(0, sdc)
	sc := float64(dpi) / 96
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(u16p("Shadow Legacy 100% Tracker"))),
		0x00CF0000|0x10000000, // WS_OVERLAPPEDWINDOW | WS_VISIBLE
		0x80000000, 0x80000000, uintptr(1180*sc), uintptr(960*sc), 0, menu, inst, 0)
	app.hwnd = hwnd
	pShowWindow.Call(hwnd, 5)
	pUpdateWindow.Call(hwnd)

	go worker()

	var mm msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&mm)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&mm)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&mm)))
	}
}
