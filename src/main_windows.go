//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	gdi32        = syscall.NewLazyDLL("gdi32.dll")
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	pMetric      = user32.NewProc("GetSystemMetrics")
	pDPI         = user32.NewProc("SetProcessDPIAware")
	pWindow      = user32.NewProc("CreateWindowExW")
	pShow        = user32.NewProc("ShowWindow")
	pPos         = user32.NewProc("SetWindowPos")
	pAffinity    = user32.NewProc("SetWindowDisplayAffinity")
	pRegister    = user32.NewProc("RegisterHotKey")
	pUnregister  = user32.NewProc("UnregisterHotKey")
	pMessage     = user32.NewProc("GetMessageW")
	pTranslate   = user32.NewProc("TranslateMessage")
	pDispatch    = user32.NewProc("DispatchMessageW")
	pAlert       = user32.NewProc("MessageBoxW")
	pDC          = user32.NewProc("GetDC")
	pReleaseDC   = user32.NewProc("ReleaseDC")
	pUpdate      = user32.NewProc("UpdateLayeredWindow")
	pCompatible  = gdi32.NewProc("CreateCompatibleDC")
	pDIB         = gdi32.NewProc("CreateDIBSection")
	pSelect      = gdi32.NewProc("SelectObject")
	pDeleteObj   = gdi32.NewProc("DeleteObject")
	pDeleteDC    = gdi32.NewProc("DeleteDC")
	pBlt         = gdi32.NewProc("BitBlt")
	pGdiFlush    = gdi32.NewProc("GdiFlush")
	pPriority    = kernel32.NewProc("SetPriorityClass")
	pProcess     = kernel32.NewProc("GetCurrentProcess")
	pCreateMutex = kernel32.NewProc("CreateMutexW")
	pCloseHandle = kernel32.NewProc("CloseHandle")
	pLastError   = kernel32.NewProc("GetLastError")
	pFindWindow  = user32.NewProc("FindWindowW")
)

const (
	layered     = 0x80000
	transparent = 0x20
	toolWindow  = 0x80
	noActivate  = 0x08000000
	topMost     = 0x8
	popUp       = 0x80000000
	srcCopy     = 0x00CC0020
	ulwAlpha    = 2
	wmHotkey    = 0x0312
	modCtrlAlt  = 3
	belowNormal = 0x4000
)

type point struct{ X, Y int32 }
type dimensions struct{ CX, CY int32 }
type message struct {
	HWND           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	LPrivate       uint32
}
type bitmapHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}
type bitmapInfo struct {
	Header bitmapHeader
	Colors [4]byte
}
type blend struct{ Op, Flags, ConstantAlpha, AlphaFormat uint8 }
type dib struct {
	dc, bmp, old, bits uintptr
	w, h               int
	pixels             []byte
}

func newDIB(base uintptr, w, h int) (dib, error) {
	if w < 1 || h < 1 || w > 10000 || h > 10000 {
		return dib{}, fmt.Errorf("invalid monitor %dx%d", w, h)
	}
	dc, _, _ := pCompatible.Call(base)
	if dc == 0 {
		return dib{}, fmt.Errorf("CreateCompatibleDC failed")
	}
	info := bitmapInfo{Header: bitmapHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var bits uintptr
	bmp, _, _ := pDIB.Call(base, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		pDeleteDC.Call(dc)
		return dib{}, fmt.Errorf("CreateDIBSection failed")
	}
	old, _, _ := pSelect.Call(dc, bmp)
	return dib{dc: dc, bmp: bmp, old: old, bits: bits, w: w, h: h, pixels: unsafe.Slice((*byte)(unsafe.Pointer(bits)), w*h*4)}, nil
}
func (d *dib) close() {
	if d.dc != 0 {
		pSelect.Call(d.dc, d.old)
		pDeleteObj.Call(d.bmp)
		pDeleteDC.Call(d.dc)
	}
	*d = dib{}
}
func screenSize() (int, int) {
	w, _, _ := pMetric.Call(0)
	h, _, _ := pMetric.Call(1)
	return int(w), int(h)
}
func alert(title, msg string) {
	a, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(msg)
	pAlert.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(a)), 0x40)
}
func logError(e error) {
	if e == nil {
		return
	}
	path := filepath.Join(os.Getenv("LOCALAPPDATA"), "PeopleOverlay", "PeopleOverlay.log")
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), e)
		f.Close()
	}
}

var enabled atomic.Bool
var configurationGeneration atomic.Uint64
var scanMode atomic.Int32
var sensitivity atomic.Int32
var contrastMode atomic.Bool
var captureRequested atomic.Bool
var captureCompat atomic.Bool
var captureProbeRequested atomic.Bool
var captureProbePercent atomic.Int32
var probeReady atomic.Bool
var showNumbers atomic.Bool
var alarmEnabled atomic.Bool
var alarmFull atomic.Bool
var fastAI atomic.Bool
var alarmPauseSecs atomic.Int32
var lastInferenceMS atomic.Int64
var lastCaptureUS atomic.Int64
var lastPrepareUS atomic.Int64
var lastPredictUS atomic.Int64
var lastDecodeUS atomic.Int64
var lastTrackingUS atomic.Int64
var lastRenderUS atomic.Int64
var pendingCamera atomic.Int32
var tentativeCount atomic.Int32
var confirmedCount atomic.Int32
var lostCount atomic.Int32
var trackedCount atomic.Int32
var testOnly atomic.Bool
var colour atomic.Int32
var stopped atomic.Bool
var version atomic.Int64
var boxes struct {
	mu      sync.RWMutex
	items   []TrackView
	updated time.Time
}

func smooth(next, prev []Rect) []Rect {
	for i := range next {
		best, idx := .18, -1
		for j, p := range prev {
			v := overlap(next[i], p)
			if v > best {
				best = v
				idx = j
			}
		}
		if idx >= 0 {
			p := prev[idx]
			next[i].X = .25*p.X + .75*next[i].X
			next[i].Y = .25*p.Y + .75*next[i].Y
			next[i].W = .25*p.W + .75*next[i].W
			next[i].H = .25*p.H + .75*next[i].H
		}
	}
	return next
}
func confidenceThreshold(level int32) float32 {
	switch level {
	case 1:
		return .47
	case 3:
		return .22
	default:
		return .32
	}
}
func saveCapture(pixels []byte, w, h int) {
	// Manual diagnostic only: image never leaves the machine.
	if w < 1 || h < 1 || len(pixels) < w*h*4 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		q := i * 4
		img.Pix[q], img.Pix[q+1], img.Pix[q+2], img.Pix[q+3] = pixels[q+2], pixels[q+1], pixels[q], 255
	}
	path := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "PeopleOverlay_capture_"+time.Now().Format("20060102_150405")+".png")
	file, e := os.Create(path)
	if e != nil {
		logError(e)
		return
	}
	e = png.Encode(file, img)
	if ce := file.Close(); e == nil {
		e = ce
	}
	if e != nil {
		logError(e)
		return
	}
	logError(fmt.Errorf("Diagnostic screenshot saved locally: %s", path))
}
func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func plot(d *dib, x, y int, r, g, b uint8) {
	if x < 0 || y < 0 || x >= d.w || y >= d.h {
		return
	}
	i := (y*d.w + x) * 4
	const alpha = 220
	d.pixels[i] = uint8(uint16(b) * alpha / 255)
	d.pixels[i+1] = uint8(uint16(g) * alpha / 255)
	d.pixels[i+2] = uint8(uint16(r) * alpha / 255)
	d.pixels[i+3] = alpha
}
func stroke(d *dib, x1, y1, x2, y2, thick int, r, g, b uint8) {
	if x1 == x2 {
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		for y := maxI(0, y1); y <= minI(d.h-1, y2); y++ {
			for o := -thick / 2; o <= thick/2; o++ {
				plot(d, x1+o, y, r, g, b)
			}
		}
	} else if y1 == y2 {
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		for x := maxI(0, x1); x <= minI(d.w-1, x2); x++ {
			for o := -thick / 2; o <= thick/2; o++ {
				plot(d, x, y1+o, r, g, b)
			}
		}
	}
}
func drawCorners(d *dib, items []Rect, color int32) {
	var r, g, b uint8
	switch color {
	case 2:
		r, g, b = 245, 50, 50
	case 3:
		r, g, b = 255, 148, 25
	default:
		r, g, b = 0, 237, 95
	}
	for _, p := range items {
		x1, y1 := maxI(0, int(p.X+.5)), maxI(0, int(p.Y+.5))
		x2, y2 := minI(d.w-1, int(p.X+p.W+.5)), minI(d.h-1, int(p.Y+p.H+.5))
		if x2 <= x1 || y2 <= y1 {
			continue
		}
		lx, ly := minI(24, maxI(6, (x2-x1)/4)), minI(24, maxI(6, (y2-y1)/4))
		thick := 2
		if x2-x1 > 120 {
			thick = 3
		}
		for _, x := range []int{x1, x2} {
			for _, y := range []int{y1, y2} {
				dx, dy := lx, ly
				if x == x2 {
					dx = -lx
				}
				if y == y2 {
					dy = -ly
				}
				// Dark outer contour improves visibility on both bright daytime
				// and black-and-white night CCTV without filling the video with boxes.
				stroke(d, x, y, x+dx, y, thick+2, 12, 20, 24)
				stroke(d, x, y, x, y+dy, thick+2, 12, 20, 24)
				stroke(d, x, y, x+dx, y, thick, r, g, b)
				stroke(d, x, y, x, y+dy, thick, r, g, b)
			}
		}
	}
}
func render(hwnd uintptr) {
	screen, _, _ := pDC.Call(0)
	if screen == 0 {
		return
	}
	defer pReleaseDC.Call(0, screen)
	var layer dib
	defer layer.close()
	mode := blend{Op: 0, ConstantAlpha: 255, AlphaFormat: 1}
	var last time.Time
	var oldVersion int64 = -1
	var oldW, oldH int
	var oldEnabled, oldTest bool
	var oldColor int32
	for !stopped.Load() {
		w, h := screenSize()
		if w < 64 || h < 64 {
			time.Sleep(time.Second)
			continue
		}
		if layer.w != w || layer.h != h {
			layer.close()
			var e error
			layer, e = newDIB(screen, w, h)
			if e != nil {
				logError(e)
				time.Sleep(time.Second)
				continue
			}
			pPos.Call(hwnd, ^uintptr(0), 0, 0, uintptr(w), uintptr(h), 0x10|0x40)
		}
		active, test, c := enabled.Load(), testOnly.Load(), colour.Load()
		v := version.Load()
		changed := v != oldVersion || oldW != w || oldH != h || oldEnabled != active || oldTest != test || oldColor != c
		if !changed && time.Since(last) < 3800*time.Millisecond {
			time.Sleep(180 * time.Millisecond)
			continue
		}
		oldVersion, oldW, oldH, oldEnabled, oldTest, oldColor = v, w, h, active, test, c
		for i := range layer.pixels {
			layer.pixels[i] = 0
		}
		if active {
			if test {
				drawCorners(&layer, []Rect{{X: float64(w)/2 - 80, Y: float64(h)/2 - 140, W: 160, H: 280}}, c)
			} else {
				boxes.mu.RLock()
				items := append([]TrackView(nil), boxes.items...)
				t := boxes.updated
				boxes.mu.RUnlock()
				if time.Since(t) < 2800*time.Millisecond {
					drawTrackedCorners(&layer, items, c, showNumbers.Load())
				}
			}
		}
		pos, z, origin := point{}, dimensions{int32(w), int32(h)}, point{}
		renderStart := time.Now()
		ok, _, _ := pUpdate.Call(hwnd, screen, uintptr(unsafe.Pointer(&pos)), uintptr(unsafe.Pointer(&z)), layer.dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&mode)), ulwAlpha)
		lastRenderUS.Store(time.Since(renderStart).Microseconds())
		last = time.Now()
		if ok == 0 {
			logError(fmt.Errorf("UpdateLayeredWindow failed"))
			time.Sleep(time.Second)
		} else {
			// Video mode updates corners at about 7fps while economy mode redraws
			// much less often. Expensive full-desktop layered updates happen only
			// when an ID/position or menu setting actually changes.
			cadence := 240 * time.Millisecond
			if scanMode.Load() == 3 {
				cadence = 100 * time.Millisecond
				if fastAI.Load() {
					cadence = 66 * time.Millisecond
				}
			}
			if scanMode.Load() == 1 {
				cadence = 350 * time.Millisecond
			}
			time.Sleep(cadence)
		}
	}
}
func makeOverlay(w, h int) (uintptr, error) {
	cls, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("People Overlay AI")
	wnd, _, e := pWindow.Call(layered|transparent|toolWindow|noActivate|topMost, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), popUp, 0, 0, uintptr(w), uintptr(h), 0, 0, 0, 0)
	if wnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed: %v", e)
	}
	pShow.Call(wnd, 4)
	ret, _, _ := pAffinity.Call(wnd, 0x11)
	if ret == 0 {
		pAffinity.Call(wnd, 1)
	}
	return wnd, nil
}
func registerHotkeys() error {
	for _, p := range [][2]uintptr{{1, 0x78}, {2, 0x79}, {3, '1'}, {4, '2'}, {5, '3'}, {6, 0x75}, {7, 0x77}, {8, 0x76}} {
		ok, _, e := pRegister.Call(0, p[0], modCtrlAlt, p[1])
		if ok == 0 {
			// Alarm/menu shortcuts are optional; another application can reserve F7/F8.
			if p[0] >= 7 {
				logError(fmt.Errorf("optional hotkey %d unavailable: %v", p[0], e))
				continue
			}
			return fmt.Errorf("cannot register hotkey %d: %v", p[0], e)
		}
	}
	return nil
}

// Windows 7/10/11 share one per-session mutex even across x86/x64 EXEs.
// Previously another build could keep drawing on top of the current one.
func singletonGuard() (func(), error) {
	name := syscall.StringToUTF16Ptr("Local\\PeopleOverlayAI_SingleInstance")
	mutex, _, err := pCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if mutex == 0 {
		return nil, fmt.Errorf("CreateMutexW: %v", err)
	}
	if err == syscall.Errno(183) {
		pCloseHandle.Call(mutex)
		return nil, fmt.Errorf("People Overlay AI уже запущен: закрой предыдущую копию через трей или Диспетчер задач")
	}
	// Older builds do not have this mutex; find their existing overlay window.
	cls := syscall.StringToUTF16Ptr("STATIC")
	title := syscall.StringToUTF16Ptr("People Overlay AI")
	other, _, _ := pFindWindow.Call(uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)))
	if other != 0 {
		pCloseHandle.Call(mutex)
		return nil, fmt.Errorf("Обнаружена другая версия People Overlay AI. Закрой старый EXE перед запуском новой версии, иначе рамки будут накладываться.")
	}
	return func() { pCloseHandle.Call(mutex) }, nil
}
func main() {
 if len(os.Args)==4&&os.Args[1]=="--capture-test"{
  if e:=runCaptureTest(os.Args[2],os.Args[3]);e!=nil{logError(e);os.Exit(26)}
  return
 }
	if len(os.Args) > 1 && (os.Args[1] == "--benchmark" || os.Args[1] == "--detect-image") {
		if e := runBenchmarkCLI(os.Args[1:]); e != nil {
			logError(e)
			fmt.Fprintln(os.Stderr, e)
			os.Exit(24)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--selftest" {
		root, e := locateResources()
		if e != nil {
			logError(e)
			os.Exit(21)
		}
		d, e := NewDetector(root)
		if e != nil {
			logError(e)
			os.Exit(22)
		}
		report, e := imageSelftest(d)
		d.Close()
		if e != nil {
			logError(e)
			os.Exit(23)
		}
		if len(os.Args) > 2 {
			if e = benchmarkWrite(os.Args[2], report); e != nil {
				logError(e)
				os.Exit(25)
			}
		}
		os.Exit(0)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pDPI.Call()
	unlock, guardErr := singletonGuard()
	if guardErr != nil {
		alert("People Overlay AI", guardErr.Error())
		return
	}
	defer unlock()
	proc, _, _ := pProcess.Call()
	pPriority.Call(proc, belowNormal)
	root, e := locateResources()
	if e != nil {
		alert("People Overlay AI", e.Error())
		return
	}
	d, e := NewDetector(root)
	if e != nil {
		logError(e)
		alert("People Overlay AI", e.Error())
		return
	}
	defer d.Close()
	w, h := screenSize()
	hwnd, e := makeOverlay(w, h)
	if e != nil {
		logError(e)
		alert("People Overlay AI", e.Error())
		return
	}
	enabled.Store(true)
	colour.Store(1)
	scanMode.Store(3)
	sensitivity.Store(2)
	alarmEnabled.Store(true)
	alarmPauseSecs.Store(30)
	loadAlarmPreferences()
	if e = registerHotkeys(); e != nil {
		alert("People Overlay AI", e.Error())
		return
	}
	defer func() {
		for i := 1; i <= 8; i++ {
			pUnregister.Call(0, uintptr(i))
		}
	}()
	tray, e := newTray(hwnd)
	if e != nil {
		logError(e)
		alert("People Overlay AI", "Не удалось создать значок трея: "+e.Error())
		return
	}
	defer tray.close()
	defer stopAviationAlert()
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); render(hwnd) }()
	go func() { defer workers.Done(); analyze(d) }()
	defer func() { stopped.Store(true); workers.Wait() }()
	var m message
	for {
		r, _, _ := pMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) {
			break
		}
		if tray.taskbarMessage != 0 && m.Message == tray.taskbarMessage {
			tray.reAdd()
			continue
		}
		if m.Message == trayMessage {
			switch uint32(m.LParam) {
			case wmRButtonUp, wmContextMenu, wmLButtonUp:
				tray.popupMenu()
			case wmLButtonDouble:
				enabled.Store(!enabled.Load())
				configurationGeneration.Add(1)
				version.Add(1)
			}
			continue
		}
		if m.Message == wmHotkey {
			switch m.WParam {
			case 1:
				enabled.Store(!enabled.Load())
				configurationGeneration.Add(1)
				version.Add(1)
			case 2:
				stopped.Store(true)
				return
			case 3:
				colour.Store(1)
			case 4:
				colour.Store(2)
			case 5:
				colour.Store(3)
			case 6:
				testOnly.Store(!testOnly.Load())
				configurationGeneration.Add(1)
				version.Add(1)
			case 7:
				tray.popupMenu()
			case 8:
				alarmEnabled.Store(!alarmEnabled.Load())
				if !alarmEnabled.Load() {
					stopAviationAlert()
				}
				saveAlarmPreferences()
			}
		} else {
			pTranslate.Call(uintptr(unsafe.Pointer(&m)))
			pDispatch.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
	stopped.Store(true)
}
