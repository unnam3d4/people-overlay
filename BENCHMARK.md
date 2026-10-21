# Validation status

The candidate is under validation. This report is updated with measured final
results before delivery; a green self-test alone is not a field-accuracy claim.

## Detection target and real-video acceptance

The application detects **whole humans (COCO class person = 0)**, not faces.
The green corner markers enclose the predicted **body box**. A person's
face need not be visible: back-facing and occluded-face people are in scope.

Optional real GitHub/CVAT footage currently has only manually labelled
**face landmarks**, not full-person bounding boxes. Face-point containment
is an auxiliary diagnostic ONLY. It does NOT validate person-body precision,
recall, IoU, stable body identity or ghost TTL. That job is non-gating.

Real-person acceptance must use frame-indexed **whole-person boxes and
tracking IDs**, such as MOT17 train/val pedestrian ground truth (respect its
CC BY-NC-SA 3.0 terms), and compare baseline/candidate on identical clips.
That real-body-bbox validation is not yet completed; no body metrics are
claimed from the head-landmark replay. The photographic-synthetic bbox
regression gate remains mandatory.
## Protocol

Baseline application: `e0929f20ef4376a32983b472b5087a68ab541e32`. Baseline and
candidate use the same embedded YOLO26n/NCNN resources and identical optional
replay harness. Processing logic and production cadence differ.

Eight 24-second photographic synthetic scenarios cover 15/30 FPS, 16 stationary
people, movement/disappearance/reentry, crossing/occlusion, subject jitter,
brightness and repeated frames. Three people are manually selected from the
upstream sample photograph; transformations provide geometric ground truth.
Source/license limitations are embedded in each manifest. These are not
independently licensed real CCTV video clips. Real articulated walking,
background-camera jitter, arbitrary layout switching, multi-hour leaks and
actual RVi operation remain separate unverified conditions.

Real PNG pixels pass through production preprocessing, native NCNN, decoder,
scheduler and tracker. One measured-duration pending worker and capture cadence
are replayed on a deterministic video clock. PNG loading is excluded from
simulated neural delivery. CPU/RAM includes loading/report generation. Concurrent
capture/inference contention and display latency are not simulated by this model.

Matching: one-to-one maximum-cardinality/IoU at IoU >= .5. Precision/recall are
frame weighted. TT-first-track percentiles exclude missed episodes, which are
explicitly counted. Reentry starts another episode. Ghost duration follows the
last matched ID until removal, marking reentry/end censoring. Center error uses
matches only. Partly occluded full-body synthetic GT boxes are approximate.

## Verified external surveillance footage: PETS2009 body boxes

**Source:** PETS2009, University of Reading S2/L1, View001, mirrored
by Kaggle (video images) and OpenTraj (manual person boxes and IDs).
The bench downloaded 795 real video frames and verified 4650 manually
annotated *whole-person* boxes from the pinned annotation checksum.
Source footage is approximately 7 FPS, replayed at 15 FPS by repeating
original frames. One-camera and 4x4 mosaics are constructed from the real
image sequence; 4x4 is NOT a recording of 16 actual RVi feeds.

These are measured **frame-weighted body IoU>=0.5 recall** from the
Windows CI run 39679157909, not face-point coverage or field RVi recall:

| Scenario (real body GT, replay 15 FPS) | v1.9 P | v1.9 R | v20 P | v20 R |
|---|---:|---:|---:|---:|
| One camera, fewer pedestrians | 51.8% | 52.3% | 46.9% | 44.0% |
| One camera, crowded | 26.1% | 28.2% | 27.1% | 25.8% |
| 4x4 screen, 4 actual feeds shown | 18.6% | 11.8% | 24.1% | 15.2% |
| 4x4 screen, 16 feeds shown | 20.3% | 6.8% | 36.6% | 6.8% |

Ground-truth appearance misses: 16-feed mosaic **58 (v1.9) versus 73
(v20)**. v20 has a serious one-camera and arrival regression and **fails**
the release quality gate. These figures pertain to the specific candidate
commit associated with CI 39679157909; later commits require a fresh
same-conditions replay before the numbers can be attributed to them.
Reference: https://github.com/unnam3d4/people-overlay/actions/runs/39679157909

**Engineering implication:** at 16 feeds and 7-FPS source frames,
scanning a small 240x135 camera tile using a CPU-only 640-input detector
does not provide acceptable full-body detection coverage. Strong claims
about reliable detection on every camera are unsupported; improving
tracking alone cannot discover a previously missed person.

## Native reference

Ultralytics 8.4.78, FP32, identical Go-preprocessed input, two photographs:
YOLOv8n and YOLO26n both match PyTorch's decoded person counts (4 bus, 2 zidane).
Maximum final-coordinate error is below .001 source pixel; matched IoU exceeds
.99999. This establishes output layout, class decoding and bundle consistency
on these inputs, not field recall. Report: `native-validation.json`.

Single warmed NCNN samples were approximately 179–182 ms on the host CPU.
They are not an old-CPU benchmark or latency percentiles. The compatible DLL
reports extractor thread selection as a no-op; no 2-thread speedup is claimed.

Final replay tables and known regressions will be added after review repairs.
