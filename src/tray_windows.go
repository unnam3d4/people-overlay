//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	trayMessage           = 0x8000 + 73 // WM_APP+73
	trayAdd               = 0
	trayDelete            = 2
	trayNIFMessage        = 1
	trayNIFIcon           = 2
	trayNIFTip            = 4
	menuString            = 0
	menuSeparator         = 0x800
	menuChecked           = 8
	menuReturnCmd         = 0x100
	menuRightButton       = 2
	menuIDToggle          = 100
	menuIDProfileSingle   = 109
	menuIDProfileEco      = 110
	menuIDProfileBalanced = 111
	menuIDProfileCCTV     = 112
    menuIDProfileAuto     = 113
	menuIDSensLow         = 120
	menuIDSensNormal      = 121
	menuIDSensHigh        = 122
	menuIDContrast        = 130
	menuIDGreen           = 140
	menuIDRed             = 141
	menuIDOrange          = 142
	menuIDTest            = 150
	menuIDNumbers         = 152
	menuIDCapture         = 153
	menuIDVideoProbe      = 154
	menuIDCaptureCompat   = 155
	menuIDFastAI          = 159
    menuIDFullAccuracy    = 157
	menuIDAlarmToggle     = 160
	menuIDAlarmShort      = 161
	menuIDAlarmFull       = 162
	menuIDAlarmPause15    = 163
	menuIDAlarmPause30    = 164
	menuIDAlarmPause60    = 165
	menuIDAlarmTest       = 166
	menuIDExit            = 199
    menuIDMonitorStart   = 200 // 200..215 selected physical monitor
	wmRButtonUp           = 0x0205
	wmLButtonUp           = 0x0202
	wmLButtonDouble       = 0x0203
	wmContextMenu         = 0x007B
)

var (
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	trayNotify                = shell32.NewProc("Shell_NotifyIconW")
	trayCreateMenu            = user32.NewProc("CreatePopupMenu")
	trayAppendMenu            = user32.NewProc("AppendMenuW")
	trayDestroyMenu           = user32.NewProc("DestroyMenu")
	trayTrackMenu             = user32.NewProc("TrackPopupMenu")
	trayGetCursor             = user32.NewProc("GetCursorPos")
	traySetForeground         = user32.NewProc("SetForegroundWindow")
	trayRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	trayCreateIcon            = user32.NewProc("CreateIcon")
	trayDestroyIcon           = user32.NewProc("DestroyIcon")
	trayLoadIcon              = user32.NewProc("LoadIconW")
	trayPostQuit              = user32.NewProc("PostQuitMessage")
	trayDestroyWindow         = user32.NewProc("DestroyWindow")
	trayPostMessage           = user32.NewProc("PostMessageW")
	trayDefWindowProc         = user32.NewProc("DefWindowProcW")
	trayRegisterClass         = user32.NewProc("RegisterClassExW")
	traySetWindowPos          = user32.NewProc("SetWindowPos")
	trayBringToTop            = user32.NewProc("BringWindowToTop")
	trayModuleHandle          = kernel32.NewProc("GetModuleHandleW")
	trayDefaultProcCallback   = syscall.NewCallback(trayDefaultWindowProc)
)

// STATIC is a control class and cannot reliably receive the foreground focus
// necessary to own a popup menu on Windows 7. Register a real top-level Win32
// window class; the main GUI message loop still owns tray and hotkey events.
type trayWindowClass struct {
	Size, Style                                                        uint32
	Proc                                                               uintptr
	ClassExtra, WindowExtra                                            int32
	Instance, Icon, Cursor, Background, MenuName, ClassName, SmallIcon uintptr
}

func trayDefaultWindowProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := trayDefWindowProc.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

type trayData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutVersion   uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	Guid             [16]byte
	BalloonIcon      uintptr
}
type appTray struct {
	hwnd           uintptr
	icon           uintptr
	ownsIcon       bool
	data           trayData
	taskbarMessage uint32
}

func trayIcon() (uintptr, bool) {
	// Small green person within detector corner marks; transparent 32x32 icon.
	const n = 32
	andBits := make([]byte, n*4)
	for i := range andBits {
		andBits[i] = 255
	}
	xorBits := make([]byte, n*n*4)
	mark := func(x, y int, r, g, b uint8) {
		if x < 0 || x >= n || y < 0 || y >= n {
			return
		}
		index := (y*n + x) * 4
		xorBits[index], xorBits[index+1], xorBits[index+2], xorBits[index+3] = b, g, r, 255
		mask := y*4 + x/8
		andBits[mask] &^= byte(0x80 >> uint(x%8))
	}
	for i := 5; i <= 12; i++ {
		for t := 0; t < 3; t++ {
			mark(i, 4+t, 6, 229, 97)
			mark(i, 27-t, 6, 229, 97)
			mark(4+t, i, 6, 229, 97)
			mark(27-t, i, 6, 229, 97)
		}
	}
	for x := 13; x <= 18; x++ {
		for y := 10; y <= 14; y++ {
			if (x-16)*(x-16)+(y-12)*(y-12) <= 10 {
				mark(x, y, 250, 250, 250)
			}
		}
	}
	for x := 12; x <= 19; x++ {
		for y := 17; y <= 24; y++ {
			if x >= 13+(y-17)/6 && x <= 18-(y-17)/6 {
				mark(x, y, 250, 250, 250)
			}
		}
	}
	h, _, _ := trayCreateIcon.Call(0, n, n, 1, 32, uintptr(unsafe.Pointer(&andBits[0])), uintptr(unsafe.Pointer(&xorBits[0])))
	if h != 0 {
		return h, true
	}
	fallback, _, _ := trayLoadIcon.Call(0, 32512)
	return fallback, false
}
func newTray(overlay uintptr) (*appTray, error) {
	// The overlay is deliberately click-through and WS_EX_NOACTIVATE.
	// The tray menu needs a *separate focusable* Win32 owner, not STATIC.
	cls := syscall.StringToUTF16Ptr("PeopleOverlayTrayOwner_v16")
	instance, _, _ := trayModuleHandle.Call(0)
	wc := trayWindowClass{Proc: trayDefaultProcCallback, Instance: instance, ClassName: uintptr(unsafe.Pointer(cls))}
	wc.Size = uint32(unsafe.Sizeof(wc))
	atom, _, registerErr := trayRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return nil, fmt.Errorf("RegisterClassExW(tray): %v", registerErr)
	}
	caption := syscall.StringToUTF16Ptr("People Overlay AI - menu")
	offscreen := int32(-32000)
	hwnd, _, createErr := pWindow.Call(toolWindow, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(caption)), popUp, uintptr(uint32(offscreen)), uintptr(uint32(offscreen)), 1, 1, 0, 0, instance, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW(tray owner): %v", createErr)
	}
	icon, owned := trayIcon()
	t := &appTray{hwnd: hwnd, icon: icon, ownsIcon: owned}
	t.data.CbSize = uint32(unsafe.Sizeof(t.data))
	t.data.HWnd = hwnd
	t.data.UID = 1
	t.data.UFlags = trayNIFMessage | trayNIFIcon | trayNIFTip
	t.data.UCallbackMessage = trayMessage
	t.data.HIcon = icon
	copy(t.data.Tip[:], syscall.StringToUTF16("People Overlay AI - video person detection"))
	msg := syscall.StringToUTF16Ptr("TaskbarCreated")
	taskbar, _, _ := trayRegisterWindowMessage.Call(uintptr(unsafe.Pointer(msg)))
	t.taskbarMessage = uint32(taskbar)
	ok, _, e := trayNotify.Call(trayAdd, uintptr(unsafe.Pointer(&t.data)))
	if ok == 0 {
		if owned {
			trayDestroyIcon.Call(icon)
		}
		trayDestroyWindow.Call(hwnd)
		return nil, fmt.Errorf("Shell_NotifyIconW(NIM_ADD): %v", e)
	}
	return t, nil
}
func (t *appTray) reAdd() {
	if t != nil {
		trayNotify.Call(trayAdd, uintptr(unsafe.Pointer(&t.data)))
	}
}
func (t *appTray) close() {
	if t == nil {
		return
	}
	trayNotify.Call(trayDelete, uintptr(unsafe.Pointer(&t.data)))
	if t.ownsIcon {
		trayDestroyIcon.Call(t.icon)
	}
	trayDestroyWindow.Call(t.hwnd)
}
func appendMenu(menu uintptr, id uintptr, s string, checked bool) {
	flags := uintptr(menuString)
	if checked {
		flags |= menuChecked
	}
	str := syscall.StringToUTF16Ptr(s)
	trayAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(str)))
}
func separator(menu uintptr) { trayAppendMenu.Call(menu, menuSeparator, 0, 0) }
func (t *appTray) popupMenu() {
	if t == nil {
		return
	}
	menu, _, createErr := trayCreateMenu.Call()
	if menu == 0 {
		logError(fmt.Errorf("tray: CreatePopupMenu failed: %v", createErr))
		return
	}
	defer trayDestroyMenu.Call(menu)
	active := enabled.Load()
	action := "Временно выключить"
	if !active {
		action = "Включить детекцию"
	}
	appendMenu(menu, menuIDToggle, action, false)
	// Read-only field diagnosis: user can report this without screenshots.
	stats := fmt.Sprintf("AI: %d мс  |  людей: %d", lastInferenceMS.Load(), trackedCount.Load())
	info := syscall.StringToUTF16Ptr(stats)
	trayAppendMenu.Call(menu, menuString|1, 0, uintptr(unsafe.Pointer(info)))
	timings := fmt.Sprintf("мс: захват %.1f / подготовка %.1f / сеть %.1f / трекер %.1f / вывод %.1f", float64(lastCaptureUS.Load())/1000, float64(lastPrepareUS.Load())/1000, float64(lastPredictUS.Load())/1000, float64(lastTrackingUS.Load())/1000, float64(lastRenderUS.Load())/1000)
	timingText := syscall.StringToUTF16Ptr(timings)
	trayAppendMenu.Call(menu, menuString|1, 0, uintptr(unsafe.Pointer(timingText)))
	stateText := syscall.StringToUTF16Ptr(fmt.Sprintf("Ожидают: %d / подтверждены: %d / потеряны: %d | AI камера: %d", tentativeCount.Load(), confirmedCount.Load(), lostCount.Load(), pendingCamera.Load()))
	trayAppendMenu.Call(menu, menuString|1, 0, uintptr(unsafe.Pointer(stateText)))
    blind:=v3WorstCameraAgeMS.Load()
    unchecked:=v3UnvisitedCameras.Load()
    // A diagnostic report, not a guaranteed recognition interval.
    // A displayed >2000ms means the requested 2s detection objective is
    // currently infeasible for at least one camera on this PC.
    coverage:=fmt.Sprintf("Камеры без AI-проверки: %d | макс. возраст AI-проверки: %.1f с",unchecked,float64(blind)/1000)
    coverageText:=syscall.StringToUTF16Ptr(coverage)
    trayAppendMenu.Call(menu,menuString|1,0,uintptr(unsafe.Pointer(coverageText)))

	separator(menu)
	appendMenu(menu, menuIDProfileAuto, "АВТО: сам определяет сетку или одну камеру", scanMode.Load() == -1)
    appendMenu(menu, menuIDProfileSingle, "Режим: одна камера на весь экран", scanMode.Load() == 0)
	appendMenu(menu, menuIDProfileEco, "Режим: экономный (2×2)", scanMode.Load() == 1)
	appendMenu(menu, menuIDProfileBalanced, "Режим: сбалансированный (3×3)", scanMode.Load() == 2)
	appendMenu(menu, menuIDProfileCCTV, "Режим: видеонаблюдение (4×4)", scanMode.Load() == 3)
    separator(menu)
    // v3 always scans up to four physical monitors. These radio commands
    // choose ONLY the target for local screenshot/capture diagnostics.
    infoAll:=syscall.StringToUTF16Ptr("Детекция: на всех экранах (до 4)")
    trayAppendMenu.Call(menu,menuString|1,0,uintptr(unsafe.Pointer(infoAll)))
    monitors:=monitorList()
    if len(monitors)>4 {monitors=monitors[:4]}
    selection:=displaySelection.Load()
    for i,m:=range monitors {
        suffix:=""
        if m.Primary { suffix=" (основной)" }
        caption:=fmt.Sprintf("Снимок: экран %d%s — %dx%d",i+1,suffix,m.Rect.width(),m.Rect.height())
        appendMenu(menu,menuIDMonitorStart+uintptr(i),caption,selection==int32(i))
    }
	separator(menu)
	appendMenu(menu, menuIDSensLow, "Чувствительность: низкая", sensitivity.Load() == 1)
	appendMenu(menu, menuIDSensNormal, "Чувствительность: обычная", sensitivity.Load() == 2)
	appendMenu(menu, menuIDSensHigh, "Чувствительность: высокая", sensitivity.Load() == 3)
	appendMenu(menu, menuIDContrast, "Усилить контраст камер", contrastMode.Load())
	appendMenu(menu, menuIDFullAccuracy, "Точный полный экран: больше AI-проверок (нагрузка CPU)",v3FullAccuracy.Load())
    appendMenu(menu, menuIDFastAI, "Быстрое сопровождение: до 15 Гц (больше нагрузка CPU)", fastAI.Load())
	separator(menu)
	appendMenu(menu, menuIDAlarmToggle, "Звуковая тревога (Ctrl+Alt+F7)", alarmEnabled.Load())
	appendMenu(menu, menuIDAlarmShort, "Сигнал: короткий, 3 импульса", !alarmFull.Load())
	appendMenu(menu, menuIDAlarmFull, "Сигнал: полный, 8 импульсов", alarmFull.Load())
	appendMenu(menu, menuIDAlarmPause15, "Пауза тревоги: 15 секунд", alarmPauseSecs.Load() == 15)
	appendMenu(menu, menuIDAlarmPause30, "Пауза тревоги: 30 секунд", alarmPauseSecs.Load() == 30)
	appendMenu(menu, menuIDAlarmPause60, "Пауза тревоги: 60 секунд", alarmPauseSecs.Load() == 60)
	appendMenu(menu, menuIDAlarmTest, "Прослушать сигнал", false)
	separator(menu)
	appendMenu(menu, menuIDGreen, "Цвет: зелёный", colour.Load() == 1)
	appendMenu(menu, menuIDRed, "Цвет: красный", colour.Load() == 2)
	appendMenu(menu, menuIDOrange, "Цвет: оранжевый", colour.Load() == 3)
	separator(menu)
	appendMenu(menu, menuIDCapture, "Сохранить проверочный кадр", false)
	appendMenu(menu, menuIDCaptureCompat, "Захват видео: совместимый режим (CAPTUREBLT)", captureCompat.Load())
	appendMenu(menu, menuIDVideoProbe, "Проверить видеокадры (2 снимка за 1,4 сек)", false)
	if probeReady.Load() {
		status := fmt.Sprintf("Тест захвата: %d%% изменившихся пикселей", captureProbePercent.Load())
		caption := syscall.StringToUTF16Ptr(status)
		trayAppendMenu.Call(menu, menuString|1, 0, uintptr(unsafe.Pointer(caption)))
	}
	appendMenu(menu, menuIDTest, "Проверить уголки", testOnly.Load())
	separator(menu)
	appendMenu(menu, menuIDExit, "Выход", false)
	var p point
	trayGetCursor.Call(uintptr(unsafe.Pointer(&p)))
	// Win7 could not activate the former STATIC owner off-screen. Put a real
	// focusable 1x1 owner beside the mouse, activate it, and display the popup.
	// It is a toolwindow, so it never clutters the Alt+Tab list or taskbar.
	traySetWindowPos.Call(t.hwnd, 0, uintptr(uint32(p.X)), uintptr(uint32(p.Y)), 1, 1, 0x4|0x10)
	pShow.Call(t.hwnd, 5) // SW_SHOW (focusable, unlike the no-activate overlay)
	focus, _, focusErr := traySetForeground.Call(t.hwnd)
	if focus == 0 {
		trayBringToTop.Call(t.hwnd)
		focus, _, _ = traySetForeground.Call(t.hwnd)
		if focus == 0 {
			logError(fmt.Errorf("tray: SetForegroundWindow denied: %v", focusErr))
		}
	}
	// TPM_RETURNCMD: return selection synchronously on the GUI thread.
	cmd, _, popupErr := trayTrackMenu.Call(menu, menuReturnCmd|menuRightButton, uintptr(uint32(p.X)), uintptr(uint32(p.Y)), 0, t.hwnd, 0)
	trayPostMessage.Call(t.hwnd, 0, 0, 0) // WM_NULL: Win32-required popup dismissal
	pShow.Call(t.hwnd, 0)                 // SW_HIDE
	if cmd != 0 {
		t.action(cmd)
	} else if popupErr != syscall.Errno(0) {
		// Cancellation normally returns 0 and last-error may be stale; only
		// log when the Win32 API reports an actual error code.
		logError(fmt.Errorf("tray: popup returned no command: %v", popupErr))
	}
}
func (t *appTray) action(id uintptr) {
    if id>=menuIDMonitorStart && id<menuIDMonitorStart+4 {
        index:=int32(id-menuIDMonitorStart)
        if index < int32(len(monitorList())) && displaySelection.Load()!=index {
            displaySelection.Store(index)
            // This is diagnostic-only; do not invalidate running detection
            // on any of the four physical screens.
            probeReady.Store(false)
            logError(fmt.Errorf("Diagnostic screenshot target switched to monitor %d",index+1))
        }
        return
    }
	defer func() {
		switch id {
		case menuIDToggle, menuIDProfileAuto, menuIDProfileSingle, menuIDProfileEco, menuIDProfileBalanced, menuIDProfileCCTV,
			menuIDSensLow, menuIDSensNormal, menuIDSensHigh, menuIDContrast, menuIDCaptureCompat, menuIDFastAI, menuIDTest:
			configurationGeneration.Add(1)
		}
	}()
	switch id {
	case menuIDToggle:
		enabled.Store(!enabled.Load())
		version.Add(1)
	case menuIDProfileAuto:
        scanMode.Store(-1)
        version.Add(1)
    case menuIDProfileSingle:
		scanMode.Store(0)
		version.Add(1)
	case menuIDProfileEco:
		scanMode.Store(1)
		version.Add(1)
	case menuIDProfileBalanced:
		scanMode.Store(2)
		version.Add(1)
	case menuIDProfileCCTV:
		scanMode.Store(3)
		version.Add(1)
	case menuIDSensLow:
		sensitivity.Store(1)
	case menuIDSensNormal:
		sensitivity.Store(2)
	case menuIDSensHigh:
		sensitivity.Store(3)
	case menuIDContrast:
		contrastMode.Store(!contrastMode.Load())
	case menuIDFullAccuracy:
        v3FullAccuracy.Store(!v3FullAccuracy.Load())
    case menuIDFastAI:
		fastAI.Store(!fastAI.Load())
	case menuIDAlarmToggle:
		alarmEnabled.Store(!alarmEnabled.Load())
		if !alarmEnabled.Load() {
			stopAviationAlert()
		}
	case menuIDAlarmShort:
		alarmFull.Store(false)
	case menuIDAlarmFull:
		alarmFull.Store(true)
	case menuIDAlarmPause15:
		alarmPauseSecs.Store(15)
	case menuIDAlarmPause30:
		alarmPauseSecs.Store(30)
	case menuIDAlarmPause60:
		alarmPauseSecs.Store(60)
	case menuIDAlarmTest:
		playAviationAlert(alarmFull.Load())
	case menuIDGreen:
		colour.Store(1)
		version.Add(1)
	case menuIDRed:
		colour.Store(2)
		version.Add(1)
	case menuIDOrange:
		colour.Store(3)
		version.Add(1)
	case menuIDNumbers:
		showNumbers.Store(!showNumbers.Load())
		version.Add(1)
	case menuIDCapture:
		captureRequested.Store(true)
	case menuIDCaptureCompat:
		captureCompat.Store(!captureCompat.Load())
		probeReady.Store(false)
	case menuIDVideoProbe:
		captureProbeRequested.Store(true)
		probeReady.Store(false)
	case menuIDTest:
		testOnly.Store(!testOnly.Load())
		version.Add(1)
	case menuIDExit:
		stopped.Store(true)
		trayPostQuit.Call(0)
	}
	// Persist only user-facing sound preferences. In particular, "mute" must
	// remain muted after Windows or RVi is restarted.
	switch id {
	case menuIDAlarmToggle, menuIDAlarmShort, menuIDAlarmFull,
		menuIDAlarmPause15, menuIDAlarmPause30, menuIDAlarmPause60:
		saveAlarmPreferences()
	}
}
