# Drawing a frame: where the time goes, and what was changed

Part of the performance study of October 2026 (`doc/notes/performance-study.md` is the first
part, on regeneration). This note is about drawing a frame of the graphics window: the
software rasteriser (`pkg/render/soft`), the OpenGL 1 renderer (`pkg/render/gl`) and the
window's paint path (`pkg/ui`). Branch `perf-core-drawing`, cut from devel 0335b0d9.

## Summary

- **A software frame of a small model cost 8 ms, nearly all of it fixed.** Not the model:
  the depth buffer was cleared four times a frame (8 MB each), the polygons that cover the
  window (background, text window, toolbar) went through the per-pixel loop on one goroutine,
  and the persistent layer was copied out (16 MB) every frame of an orbit, when no frame comes
  back.
- **The per-pixel loop did work per pixel that is the same for a whole triangle or line:**
  the colour was clamped and rounded at every fragment, the edge functions were evaluated in
  full, and every vertex clipped escaped to the heap because the clip planes were a table of
  functions.
- **The GL renderer made a cgo call for every vertex and normal** (immediate mode), about 70 ns
  each, so a 7766-triangle mesh cost 50 000 calls a frame.
- Four commits change this, each holding every PNG golden, every window frame and the GL test
  frames (under Xvfb) byte-identical: frames of the window are **20–60% faster in software**
  and allocate 50–95% less; GL frames of large models are **about 50% faster**.

## Method

- `BenchmarkFrame` (pkg/ui) orbits three models (80, 4306 and 7766 triangles) at 1920x1080,
  with the software renderer and with GL on an offscreen GLX buffer; `BenchmarkPaint*` orbit
  fixtures in software, and `BenchmarkPaintHover` sweeps the mouse with the view still, so
  the persistent layer comes from its cache. `BenchmarkSoft*` are the rasteriser alone.
- Test binaries before and after each change were run interleaved six times and compared with
  benchstat. The GL numbers are on a Radeon RX 6600 with a desktop running, and vary by up to
  80% between runs; only the bolt's 30% is significant at n = 6.
- Correctness: `go test ./pkg/render/... ./pkg/ui` (PNG goldens, frame goldens,
  `TestPaintParallelSameAsSerial`, `TestInteraction`'s with-and-without-cache frames), plus
  the GL renderer's tests under Xvfb with `TELESAPR_GL_DUMP` before and after, compared with
  `cmp`. `TestGoScripts` fails in the worktree before and after alike (the worktree's path is
  too long for a path a script prints; it passes in the main checkout).
- Profiles: `go test -cpuprofile` on `BenchmarkFrame/bolt/software` and `/ball_mill/opengl`.

## Baseline, and where the time went

| Benchmark | ms | notes |
|---|---:|---|
| Frame/boolean/software | 8.8 | 80 triangles |
| Frame/bolt/software | 25.0 | 4306 |
| Frame/ball_mill/software | 34.5 | 7766 |
| Frame/boolean/opengl | 1.5 | |
| Frame/bolt/opengl | 3.7 | |
| Frame/ball_mill/opengl | 6.6 | |
| PaintTranslate | 14.5 | |
| PaintText | 8.0 | |
| PaintBoolean | 9.2 | |
| PaintHover | 10.6 | layer restored from cache |
| SoftMesh1080p | 3.5 | |
| SoftLines1080p | 69.7 | 100 000 short lines |
| SoftLongLines1080p | 457 | 100 000 long lines |

Profile of the bolt in software (CPU over all goroutines): `rasterTriangleRows` 81% cum (63%
flat), `fragment` 19%, `math.Round` 6%, `clamp01` 4%, `memmove` 3% (the depth clears and the
layer copy). The boolean frame: the same loop at 75%, `clearDepth` 9%, `SaveLayer` 4%,
`SetCamera` 6% (it clears the depth), `StartFrame` 5%. Allocation: `clipSegment` 27% of
objects, `DrawMesh` 14% (a slice of three vertices per triangle), both from vertices passed to
the clip-plane functions.

The 80-triangle frame's 8 ms is a fixed cost: two or three polygons the size of the window
(two million pixels each) drawn serially at about 3 ns a pixel, plus four depth clears, a
frame fill and a layer save, about 40 MB of memory traffic.

Profile of the ball mill in GL (after the arrays change, below): `cgocall` 37%; before it
the per-vertex calls were the whole of it. `DrawPoint` 20%: every point ended the run of
primitives before it (see the third commit). What remains is the GPU: a frame waits in
`glFinish`.

## What changed

| Commit | Change | Effect |
|---|---|---|
| 667b0b54 | soft: depth cleared lazily (`depthStale`, materialised by `readyDepth` on the first read); a batch of few but large triangles drawn in bands (`bigTriangles`, 64k pixels); a solid opaque colour converted to bytes once per triangle (`fragmentOpaque`) and a line's colour clamped once (`fragmentBlend`); the per-row part of each edge function hoisted in the same expression; clip planes a switch (`planeDist`) instead of functions, so vertices stay on the stack | software frames −21 to −29%, PaintHover −59%, lines −10%, allocations −50 to −95% |
| 8d2aa7da | gl: each run of primitives collected (`vertexBatch`: vertex, normal, colour, texture coordinate as current when added) and sent with one `glDrawArrays` from client arrays | bolt −30% |
| eb9d01e0 | gl: arrays stay enabled, pointers set only when a slice moved; `DrawPoint` reads its stroke without selecting it, so a stroke's points are one run of quads | see the final table |
| 273896ec | ui: the persistent layer is saved only when the frame before had the same key (the frame after the view stops), not on every frame of an orbit | see the final table |

Benchstat of the first commit (software), n = 6:

```
Frame/boolean/software-32      8.794m ± 18%   6.275m ± 23%  -28.64% (p=0.002)
Frame/bolt/software-32         24.99m ± 15%   19.67m ± 15%  -21.29% (p=0.002)
Frame/ball_mill/software-32    34.53m ±  7%   28.25m ± 35%        ~ (p=0.065)
PaintTranslate-32              14.45m ± 12%   10.84m ± 67%        ~ (p=0.065)
PaintText-32                   8.004m ±  5%   5.688m ± 22%  -28.94% (p=0.002)
PaintBoolean-32                9.162m ±  4%   6.491m ± 22%  -29.15% (p=0.002)
PaintHover-32                 10.624m ±  2%   4.314m ± 19%  -59.39% (p=0.002)
SoftLines1080p-32              69.65m ±  3%   62.60m ±  4%  -10.11% (p=0.002)
SoftLongLines1080p-32          457.4m ±  3%   407.2m ±  5%  -10.98% (p=0.002)
SoftEdges1080p-32              21.67m ±  5%   19.73m ± 12%   -8.97% (p=0.026)
allocs/op: Frame/bolt 11833 → 646, PaintText 4225 → 258, SoftLines 100k → 0
```

Benchstat of the second commit (GL, real card, noisy):

```
Frame/boolean/opengl-32     1.477m ± 79%   1.312m ± 33%        ~ (p=0.310)
Frame/bolt/opengl-32        3.681m ± 78%   2.571m ± 22%  -30.14% (p=0.002)
Frame/ball_mill/opengl-32   6.647m ± 52%   6.083m ± 32%        ~ (p=0.310)
```

Benchstat of the branch as a whole (all four commits against devel), n = 6, interleaved:

```
                               base             branch
Frame/boolean/software-32      9.145m ±  15%   5.654m ±   5%  -38.17% (p=0.002)
Frame/boolean/opengl-32        1.327m ±  38%   1.152m ±   9%  -13.20% (p=0.004)
Frame/bolt/software-32         26.65m ±  14%   19.45m ±   7%  -27.02% (p=0.002)
Frame/bolt/opengl-32           3.711m ± 132%   1.939m ±  52%  -47.75% (p=0.002)
Frame/ball_mill/software-32    34.88m ±   6%   28.47m ±  49%        ~ (p=0.065)
Frame/ball_mill/opengl-32      6.640m ±  11%   3.310m ±  42%  -50.16% (p=0.002)
PaintTranslate-32              14.76m ±  10%   10.49m ± 101%        ~ (p=0.065)
PaintText-32                   7.996m ±   7%   5.071m ±  41%  -36.57% (p=0.002)
PaintBoolean-32                9.269m ±  12%   5.890m ±  10%  -36.46% (p=0.002)
PaintHover-32                 10.964m ±   8%   4.810m ±   8%  -56.13% (p=0.002)
geomean                        8.960m          5.679m         -36.62%
allocs/op: Frame/bolt/software 11832 → 646, ball_mill 25613 → 3200, PaintText 4225 → 258;
GL frames allocate the same (the arrays are reused).
```

The ± spreads of the branch's software runs (49%, 101%) are from one slow round each, with
the desktop busy; the medians are what the percentages compare.

## Measured and not kept, and what to keep in mind

- **Incremental edge functions** (adding a per-pixel delta instead of multiplying) change the
  last bits of each edge value, and with them which pixels on an edge are drawn: goldens.
  The hoisting kept is the exact per-pixel expression with its x-free part computed once per
  row, which is the same float. On arm64 the compiler may fuse the multiply and subtract
  either way; the goldens are amd64's.
- **A per-row colour conversion for lit meshes** (`fragColor`) can't be hoisted: the colour is
  interpolated per pixel with perspective correction, and reassociating it changes bits.
- `SetCamera` must still clear the depth: the GL renderer does (`glClear` in
  `updateProjection`), and the window relies on it between the 3D scene and the 2D overlay.
  Only the clears that nothing reads between are now free.

## Where the time goes now, and recommendations

Ordered by expected gain over cost. "Same bytes" means the frame goldens hold it.

1. **D1: the software rasteriser's inner loop, same bytes, 1–2 days.** After this branch the
   per-pixel cost is the three edge tests, the depth test and, for lit meshes, the
   perspective-correct colour. Worth doing: a per-triangle fast path when the whole row span is
   inside (all three edges positive at both ends: no per-pixel edge test, the bits are the
   same since the test is only skipped), and a depth-only pass (`colorWrite` false) that
   skips the colour entirely. Expected −20–30% on mesh-heavy frames.
2. **D2: GL with vertex buffers kept across frames, same pixels, 2–3 days.** The arrays are
   rebuilt and copied to GL every frame. A mesh's vertices don't change while the view
   orbits: keep a VBO per `*srf.SMesh` keyed on the sketch generation (the persistent layer's
   key already carries it), invalidate on regeneration. The ball mill's frame would then be a
   handful of calls. The shaded program (`shaded.go`) is the place to start: it is already a
   batch per material.
3. **D3: lines in GL, same pixels, 1 day.** `DrawEdges` and `DrawOutlines` walk every edge
   through the stipple logic per frame; with a continuous stroke this is a run of `LINES` now,
   but still 2 appends and a visibility test per edge. Keep an edge array per outline list and
   view direction, as D2.
4. **D4: draw while regenerating, behavioural, 3–5 days.** The window waits for a regeneration
   before it paints (`ScheduleGenerateAll` runs in the event loop). Painting the last
   generation's meshes while the new one builds off-thread (there is a `ShellJob` already)
   keeps the window moving during a drag of a solid's dimension.
5. **D5: a tiled software rasteriser, goldens, 1–2 weeks.** Binning triangles to 32x32 tiles
   and testing each tile's corners before its pixels is how a software renderer gets to
   hundreds of thousands of triangles. It changes nothing per pixel in principle, but it
   would be a rewrite, and a good moment to move to fixed-point edge functions, which do
   change the goldens.
6. **D6: SIMD for the rasteriser, goldens, later.** Go 1.27's `simd` package is still
   experimental; the inner loop is 4–8 pixels wide at a time, and float64 to keep the bits is
   only two lanes. Not before D5.
7. **D7: the hover frame, same bytes, half a day.** `PaintHover` restores 16 MB (pixels and
   depth) and draws the overlay; it is 4 ms now, of which the copy is about 1.5 ms. Restore
   only the rows the hover's bounding box touches, or draw the highlight to a copy.
8. **D8: the text window.** It paints into the frame with `DrawRect` and `DrawPixmap` per
   character cell; it is not in these benchmarks (the frames have `NoFrame`), and worth a
   benchmark of its own before any work.

## Reproducing

```sh
go test ./pkg/ui -run '^$' -bench 'BenchmarkFrame|BenchmarkPaint' -count 6 > new.txt
go test ./pkg/render/soft -run '^$' -bench Soft -count 6
TELESAPR_GUI_TESTS=1 TELESAPR_GL_DUMP=/tmp/gl xvfb-run -a go test ./pkg/render/gl   # frames to compare
```
