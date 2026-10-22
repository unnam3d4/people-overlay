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
