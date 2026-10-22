# 2026-10-22: user-trained one-class YOLOv8n vs original YOLOv8n — HOLD

## Provenance and protocol

The user fine-tuned YOLOv8n on eight public person-video samples using automatically generated YOLO pseudo-labels and returned the trained weights/NCNN export. The resulting experimental single-class model has NCNN output 5x8400, adapted for the existing 84x8400 runtime. User ran the Windows x64 A/B offline test locally and uploaded **RESULT_TO_SEND.zip**.

Both standalone NCNN EXE positive-person/blank-image selftests **PASS**. They processed the same 33 held-out frames from `person_bicycle_car_detection` at 640x640 and confidence threshold 0.32. This does not demonstrate compatibility with hardware-rendered RVi, SmartPSS, iVMS, Win7, or multi-camera 4x4.

| Offline metric | Original YOLOv8n | Fine-tuned one-class |
|---|---:|---:|
| Person boxes predicted across 33 frames | 12 | 6 |
| Frames with any predicted person | 12 | 6 |
| Average CPU inference ms (one pass) | 189.51 | 183.48 |
| p95 CPU inference ms (one pass) | 204.41 | 196.27 |
| Bus selftest positive / blank negative | PASS/PASS | PASS/PASS |

Manual review of divergent predictions on the same supplied images:
- Fine-tuned model **missed genuine people** detected by baseline at frames `0006`, `0042`, `0044`, `0045`, `0047`, `0048` (the latter two contain a cyclist/person).
- Fine-tuned model correctly dropped a **false car box** of baseline at `0014`.
- Fine-tuned model created a new **false car box** at `0016`.
- Five overlapping true person predictions (`0001`-`0005`) are geometrically close (~0.90 mean IoU) but typically score lower in the trained model.
- Avoid treating counts as recall: 33 frames lack full independently annotated ground truth and all come from one source clip. One sequential CPU timing pass is insufficient for a speed claim.

User training stopped at epoch 20 under early stopping, best epoch **2**. The measured validation mAP50 around 0.992 uses **teacher-generated pseudo-labels**, not independently labeled ground truth. It cannot validate accuracy.

## Decision

**Do not promote this fine-tuned pilot to the distributed People Overlay EXE. Keep the original bundled YOLOv8n as default.** Continue tuning/validation offline only.

Before any release: manually correct person annotations (distant, partially visible, bicycle riders), annotate independent multi-camera/night-time test material, compare person-level precision/recall at 0.22 and 0.32, measure false-alarm rate and real CPU time, and run live VMS capture test when available.

This report is only about model output differences. It has no human identification/facial recognition.

## 2026-10-23 threshold sweep on independent frame-presence labels

User ran `RUN_NEXT_TEST.bat` and returned `SEND_THRESHOLD_STUDY_TO_CHAT.zip`, comparing PyTorch models at 7 confidence thresholds on the SAME 33 frames. Manual presence GT: 12 positive, 19 negative, 2 clipped/ambiguous excluded. This is **one video**, not bbox-level truth nor NCNN benchmark.

| conf | original detected positive/12 | trained detected positive/12 | original false frame/19 | trained false frame/19 |
|---:|---:|---:|---:|---:|
| .10 | 12 | 9 | 1 | 2 |
| .16 | 12 | 8 | 1 | 2 |
| .22 | 11 | 6 | 1 | 1 |
| .32 | 11 | 6 | 1 | 1 |
| .50 | 11 | 5 | 1 | 1 |

At .32 trained misses true-person frames `0006,0044,0045,0046,0047,0048`, while original misses `0046`; lowering trained to .10 still misses `0045,0046,0048`. Trained falsely calls a car a person at `0016` at .32 and adds `0014` at .10; baseline falsely calls a car at `0014`. At .16 baseline catches `0046` without *additional* false-frame alerts on this one clip, but this is **not enough** to retune production sensitivity.

Teacher pseudo labels came from `yolo11m.pt`, `imgsz=1280`, conf .28; for 273 train frames 835 person boxes, with 48 frames without any teacher box. Best student epoch 2, training stopped at epoch 20; teacher-labeled val mAP cannot establish true accuracy. **Do not increase epochs or lower threshold as a substitute for independent bbox corrections.** Need manually verified bbox per person in more footage, particularly cyclists, small/partially cropped people and night/crowded CCTV, before retraining. Retain baseline as production default.
