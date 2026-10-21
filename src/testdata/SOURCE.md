# Embedded neural self-test image

`bus-reference.jpg` is the unchanged `ultralytics/assets/bus.jpg` distributed in
Ultralytics 8.4.78 and the public AGPL-3.0 repository:
https://github.com/ultralytics/ultralytics/blob/v8.4.78/ultralytics/assets/bus.jpg

SHA256: c02019c4979c191eb739ddd944445ef408dad5679acab6fd520ef9d434bfbc63

The upstream repository does not separately identify the photographer or an
independent photo license. This copy follows that repository distribution;
no independent Creative Commons/public-domain claim is made. Preserve this
notice and the project AGPL license when redistributing the self-test fixture.

Three complete pedestrian rectangles were manually inspected in source pixels:
`[50,398,246,904]`, `[220,403,349,862]`, `[667,397,810,877]` (xyxy).
The partly clipped left-edge pedestrian is not required by the self-test.
This photograph tests model loading, color conversion, decoding, and coordinates;
it does not establish CCTV field accuracy.
