# People Overlay PicoDet Lite — detect-only and flexible CCTV grids

**Experimental** Windows 7 SP1 and Windows 10 offline x64/x86 build.
Derived from PicoDet Lite v3; previous releases and YOLO unchanged.

## Two modes (tray → right-click)
- **Обнаружение** (default): a positive PicoDet person box flashes four
  corner markers for ~1.3–2.3 seconds (depending on layout). No optical flow,
  identity tracking or position guessing. A negative result clears markers
  immediately. Standing people will NOT remain continuously highlighted when
  a dense grid takes longer than that to revisit. This is intentional anti-ghost
  behavior, not 100% detection coverage.
- **Сопровождение**: previous multi-monitor anchor-based tracker for
  ordinary auto/fullscreen/2x2/3x3/4x4 views, if needed.
- For custom rectangular grids the mode is always detector-only; manually
  forcing tracking would lack correct camera ownership geometry.

## Camera layout
- Auto: fullscreen, 2x2, 3x3, 4x4 only, as before.
- **Сетка вручную**: independently select 1 through 12 columns and 1
  through 12 rows in tray pop-up submenus (up to 144 tiles on a monitor).
  Works for e.g. 5x4, 6x5, 8x8, 10x5, 12x6, etc. "50" is not hard-coded.
  This is a *nominal evenly divided rectangle*: toolbars, uneven pane sizes
  or custom RVi camera windows need geometric calibration; otherwise boxes
  may be attributed to the wrong area or never detected.
- **Развернул одну камеру:** select fullscreen (or Auto if its edge detector
  recognises the layout reliably). Manually chosen grids cannot infer a change
  in layout from RVi APIs, because no video client APIs are accessed.
- Each of up to four Windows displays is captured separately, and all share
  **one serial NCNN CPU worker**. 50–144 feeds may take many seconds to scan,
  especially on older CPUs. They are NOT processed simultaneously; no speed
  or detection guarantees for such densities.

All x64/x86 EXEs include NCNN DLL + weights and need no internet/Python.
Do not use as the sole safety/security control. Benchmark and Win7 field tests
on actual video clients, CPU/long-run reliability and multiple displays are
still required.
