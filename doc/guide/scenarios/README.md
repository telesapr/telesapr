# Scenarios: how the guide's pictures are made

Each `<page>.scenario` here replays clicks, keys and menu entries on TeleSAPR without a window
(the headless platform the interface tests use) and saves pictures of what the windows show into
`../images/`. `TestDocScenarios` (`pkg/ui/docshot_test.go`) runs every scenario as a test, so a
change to the interface that breaks a page's steps breaks the build. It also fails when a page
shows an `images/<name>.png` that no scenario, nor `TestDocWindowShots` (below), makes.

    ./scripts/regen-doc-images.sh            # every scenario
    ./scripts/regen-doc-images.sh fillet     # one

    # What a scenario's dump, tw, hover and hovergrid lines print, while writing it:
    TELESAPR_DOC_SCENARIO=fillet go test ./pkg/ui -run '^TestDocScenario$' -count=1 -v

Pictures are committed, in a commit of their own, after looking at each one.

## Menus, popup menus and dialogs

The headless platform draws no menus or dialogs: they are the X server's own windows. Their
pictures come from the real window under an Xvfb of its own, read back from the screen:

    ./scripts/regen-doc-window-images.sh

`TestDocWindowShots` (`pkg/ui/docwindow_test.go`) lists them in `docWindowShots`, each with what
to do before its picture (open a menu, right-click in the drawing area, run a command that opens a
dialog, open a menu's cascade with `OpenSubMenu`) and what to do after it (close the menus or
dialogs); a picture is taken once the screen has stayed the same for half a second. The model is
`examples/scripts/fillet` in the nearest isometric view, the graphics window 800×600 with the
property browser 440 wide beside it, since no window manager places them. The test runs only when
the script sets `TELESAPR_DOC_WINDOW_SHOTS`, never in `make test`. Its pictures are the same bytes
on every run: the script gives the Xvfb a fontconfig that sees only the Liberation fonts (Sans, Serif
and Mono), so another machine with the same Liberation version makes the same bytes.

## Videos

A scenario records a video of some of its steps between `video start NAME` and `video end`:
a frame after each step, the pointer gliding to where it is moved, a ring where it presses, the
key a step presses at the bottom right, and the scenario's `caption` under the windows. ffmpeg
makes `../videos/NAME.webm` (VP9) and `../videos/NAME.mp4` (H.264, yuv420p, faststart) of the frames:

    ./scripts/regen-guide-videos.sh                    # every scenario that records one
    ./scripts/regen-guide-videos.sh getting-started   # one

The commands do nothing unless `TELESAPR_DOC_VIDEOS` names a directory, which the script sets, so
the scenarios run as tests and make their pictures as before. A page shows a video with
`<video controls muted loop playsinline style="width:100%">` holding `<source src="videos/NAME.webm" type="video/webm">` and then `<source src="videos/NAME.mp4" type="video/mp4">`, and links to the files
beside it, for where the page shows none (the site's build, `tools/docs/hooks.py`, makes the
source relative to where the page ends up); `TestDocScenarios` fails on a video a page shows that no
scenario records, and on one in `videos/` that no page shows. Videos are not goldens: ffmpeg's
versions write other bytes for the same frames. Watch every one before committing it, and commit
them on their own. Keep each under about 1 MB.

## The language

One command per line; `#` starts a comment. The graphics window is 600×600 pixels, the property
browser beside it 600×600. Coordinates are pixels from the window's top left.

| Command | Does |
|---|---|
| `file $repo/examples/scripts/x/x.go` then `menu Script\|Run File...` | Builds a model by running an example script |
| `load tlspr/...` | Opens a file under `testdata` |
| `key r`, `key x shift`, `key esc`, `key F3`, `key z ctrl` | A key (`esc del tab bs ret space left right up down F1..F12`) |
| `menu Features\|Fillet Edges` | A menu entry, by its labels |
| `move X Y`, `down L X Y`, `up L X Y`, `dbl L X Y`, `scroll X Y D` | The mouse in the graphics window (buttons L M R) |
| `twmove`, `twdown`, `twup` … | The same in the property browser |
| `click-entity NAME [shift] [ctrl] [right]`, `move-entity NAME` | Clicks a sketch entity, or moves the pointer over it, wherever the view draws it: NAME is its handle in hex, as `dump` prints it, or its name |
| `click-edge NAME …`, `click-face NAME …`, `move-edge`, `move-face` | The same for an edge or a face of the active group's solid, by the name the interface gives it ("top face of g003-block") |
| `click-tw [#N] TEXT` | Clicks the property browser's link showing TEXT, the Nth when several do |
| `click-entity NAME [shift] [ctrl] [right]`, `move-entity NAME` | Clicks (or only points at) an entity by its handle as `dump` prints it (`40000`) or by its name as the interface shows it (`r005-arc-of-circle of g003-path`), wherever the view puts it: four tenths along a line or a curve, or beside it where a point or another line lies on top. Prints `entity missing` or `entity unpicked` when it can't. `right` clicks the right button, for a context menu. Prefer it to `move`/`down`/`up` with coordinates, which break when the layout moves |
| `click-edge NAME [shift] [ctrl] [right]`, `move-edge NAME` | Clicks (or points at) an edge of the active group's solid that no entity draws, as a Boolean or a fillet makes, by its name as the interface writes it (`the edge between face 0 of g004-fillet and top face of g003-extrude`), wherever the view puts it. Prints `edge missing` or `edge unpicked` when it can't |
| `click-face NAME [shift] [ctrl] [right]`, `move-face NAME` | The same for a face: an entity's (`top face of g003-extrude`) or one no entity names (`face 4 of g003-lathe`), at the middle of its largest triangle on the screen that picks it. Prints `face missing` or `face unpicked` |
| `click-tw [#N] TEXT` | Clicks the property browser's link whose text is `TEXT` (its spaces and the radio button's or check box's glyph left out), the Nth such link with `#N`: `click-tw [change]`, `click-tw #2 [change]`, `click-tw held to x`. Prints `click-tw missing` when there is none |
| `edit TEXT` | Types into the field the last click opened, and presses Enter |
| `answer ok\|yes\|no`, `form k=v …`, `file PATH`, `popup LABEL` | Answers the next message dialog, form, file chooser or context menu (queue them *before* the command that opens it) |
| `view default\|rotated\|under` | A fixed view: from above, or turned to show three sides |
| `menu View\|Zoom To Fit`, `menu View\|Zoom Out` | Zoom To Fit fits the model (or what is selected) into the part of the window the toolbar leaves free; Zoom In and Out zoom about its middle |
| `cursor on\|off` | Draws the pointer where the mouse last was |
| `mark X Y W H [tw]` | An orange box round something, on the next shot only (`tw`: property browser coordinates) |
| `shot NAME [gw\|tw\|both]` | Saves `images/NAME.png`: the graphics window, the property browser, or both side by side (default) |
| `size gw\|tw W H` | Resizes a window |
| `video start NAME [gw\|tw\|both]`, `video end` | Records the steps between them as `videos/NAME.webm` and `NAME.mp4` (see Videos), of the graphics window, the property browser or both (default) |
| `caption TEXT`, `pause S` | The caption under the video's frames from here on; hold the last frame S seconds longer |
| `dump`, `tw`, `hover`, `hovergrid N` | Print the model, the property browser's rows, what is under the mouse, or what is under each point of an N×N grid: use them to find where to click |

A scenario runs in a temporary working directory, where the examples it runs save their files:
write paths as `$repo/…`, `$testdata/…` or `$tmp/…`, never relative. An example runs on a new
drawing: after drawing something, `answer no` and `menu File|New` first.

## Conventions

- Name pictures `<page>-<n>-<what>.png`, numbered in the order the page shows them.
- Pictures are written paletted where they have 256 colours or fewer, at zlib's best compression
  (`writeDocPng`, `pkg/ui/docshot_png_test.go`): the same pixels give the same bytes every run.
- A shot of a solid turns the view (`view rotated`) and fits it; pick edges and faces in the view
  where they are easiest to hit, then turn.
- Turn the cursor off for pictures of results; keep it on where a picture shows what to click.
- Zoom To Fit fits what is selected when anything is: fit before picking, not after. It fits the
  solid and what the sketches draw, but not the circles and sketches a screw thread is made of,
  which may run out of the window.
- Click a solid's faces and edges by name (`click-face`, `click-edge`), as entities
  (`click-entity`): a click by coordinates breaks when the view or the mesh moves. Where the place
  on the face matters (a datum placed where it is clicked), keep the coordinates.
- A click within 10 pixels of a line, a point or a label picks that before the face under it,
  but not a line or a point the solid hides: there the face in front is picked. A reference
  plane's dashed square crosses most views. Check picks with `hover` before relying on them.
- Click what is there by name (`click-entity`, `click-edge`, `click-face`, `click-tw`): the step
  keeps working when the view or the layout moves. Coordinates stay where the place itself is
  the point: drawing (a command pending places a point where the pointer is), dragging, a
  constraint's label, empty space, and a click whose pointer a picture or a video shows.
- Look at every picture before using it on a page.
