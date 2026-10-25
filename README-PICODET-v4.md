# People Overlay PicoDet v4 — region-only screen capture

**Experimental, portable Windows 7 SP1/10, 32-bit/64-bit**. No Internet, Python
or model installation on CCTV computers. Keeps PicoDet-S 320 and the previous
NCNN runtime. Default direct person detection, original tracking selectable in
the tray. Alarm samples, sound controls, cooldowns are unchanged from v3.

## What changed from FlexGrid

- **Fast path** for detector-only: win32 BitBlt copies just the selected
  camera region (including 12% overlap), not the whole monitor.
- In multiple monitors one serial neural worker still handles ALL displays.
  The GDI source is the *visible desktop* DC, not PrintWindow or a protected
  VMS internal video surface. Windows 7 compatible.
- GDI DIB stores each crop at its native desktop coordinates. The exact same
  region is copied into the neural request and mapped back to overlay pixels.
- If automatic layout recognition is active, a full monitor capture is still
  performed periodically to notice zooms or layout switches.
- On-demand capture/diagnostic requests force a full screenshot, too.
- The previous full BitBlt pipeline remains unchanged for tracking mode.
- A failed cropped capture invalidates that monitor's detections after three
  consecutive errors, without clearing detections on unrelated displays.

## Limitations

- Not a per-HWND attached capture; Win7/RVi/SmartPSS frequently render video
  inside a *single* top-level window and may give black frames via PrintWindow.
  Screen ROI capture is the safer optimization; no unsupported window hooks.
- The camera grid is equally spaced, manual columns/rows 1-12. Different
  shaped tiles, panels, and 50+ windows require validation against actual RVi.
- The neural model serially processes one region at a time. 50+ cameras are
  NOT 50 simultaneous checks. Bound of 2.3s on corner visibility means
  deliberate blinking if revisit time exceeds the TTL.
- Test unit tests & CI person image on Windows x86/x64 do not prove Win7
  runtime, multi-monitor visibility in third-party VMS or 24/7 stability.

Published package is an **experimental candidate** for one PC test, not a
security-certified version.
