# PicoDet-S 320 for People Overlay — isolated research branch

This is **not** a replacement for stable/v3.6 or v3.6.3. Do not use it
for operational security without CCTV trials.

## Why PicoDet and what actually changes

- Fast, small **PicoDet-S 320** (official pre-converted NCNN demo model).
- Custom Windows 7 x86/x64 NCNN DLL: four PicoDet cls/DFL feature heads,
  person-only COCO class 0, DFL distances and bounded NMS.
- Go preprocesses screenshots to 320x320 RGB letterboxed input; the
  custom DLL converts to official PicoDet BGR mean/std normalization.
- Custom decoder maps detected person boxes back onto Windows monitor
  coordinates. Existing sound, tracking, corner style, multiple monitors,
  offline packaging and tray remain unchanged.
- Model runtime is strictly offline on operator PCs. The CI builder
  downloads both PicoDet model files before embedding them into EXE.
- The launcher **refuses to use a YOLO NCNN bridge** via PPLModelKind check.

**Known limitations:**
- 320px input is cheaper but may be less accurate for tiny CCTV bodies.
- PicoDet-S 416 would require independently converted/tested model and DLL;
  it is NOT included here.
- Screen grids with ~50 cameras per display are NOT supported as individually
  calibrated camera cells: inherited auto mode covers up to 4x4.
- No real 48/100 camera performance, Windows 7 or SMART PSS testing yet.
- No speed guarantee: benchmark CPU times on other hardware do not apply.

Official upstream:
PaddlePaddle/PaddleDetection release/2.9
https://github.com/PaddlePaddle/PaddleDetection/tree/release/2.9/deploy/third_engine/demo_ncnn

PicoDet model weights:
https://paddledet.bj.bcebos.com/deploy/third_engine/picodet_s_320_coco_lcnet-opt.param
https://paddledet.bj.bcebos.com/deploy/third_engine/picodet_s_320_coco_lcnet-opt.bin

LICENSE: PaddleDetection upstream Apache 2.0. Existing People Overlay
project licenses remain in effect.

DO NOT publish as ready if GitHub's real-inference job fails. Only that job
produces public EXE downloads after successful positive/blank inference.
