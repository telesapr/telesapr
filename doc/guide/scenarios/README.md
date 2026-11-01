# Scenarios: how the guide's pictures are made

Each `<page>.scenario` here replays clicks, keys and menu entries on TeleSAPR without a window
(the headless platform the interface tests use) and saves pictures of what the windows show into
`../images/`. `TestDocScenarios` (`pkg/ui/docshot_test.go`) runs every scenario as a test, so a
change to the interface that breaks a page's steps breaks the build. It also fails when a page
shows an `images/<name>.png` that no scenario makes.

    ./scripts/regen-doc-images.sh            # every scenario
    ./scripts/regen-doc-images.sh fillet     # one

    # What a scenario's dump, tw, hover and hovergrid lines print, while writing it:
    TELESAPR_DOC_SCENARIO=fillet go test ./pkg/ui -run '^TestDocScenario$' -count=1 -v

Pictures are committed, in a commit of their own, after looking at each one.

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
| `edit TEXT` | Types into the field the last click opened, and presses Enter |
| `answer ok\|yes\|no`, `form k=v …`, `file PATH`, `popup LABEL` | Answers the next message dialog, form, file chooser or context menu (queue them *before* the command that opens it) |
| `view default\|rotated\|under` | A fixed view: from above, or turned to show three sides |
| `menu View\|Zoom To Fit`, `menu View\|Zoom Out` | Zoom To Fit fits the model (or what is selected) into the part of the window the toolbar leaves free; Zoom In and Out zoom about its middle |
| `cursor on\|off` | Draws the pointer where the mouse last was |
| `mark X Y W H [tw]` | An orange box round something, on the next shot only (`tw`: property browser coordinates) |
| `shot NAME [gw\|tw\|both]` | Saves `images/NAME.png`: the graphics window, the property browser, or both side by side (default) |
| `size gw\|tw W H` | Resizes a window |
| `dump`, `tw`, `hover`, `hovergrid N` | Print the model, the property browser's rows, what is under the mouse, or what is under each point of an N×N grid: use them to find where to click |

A scenario runs in a temporary working directory, where the examples it runs save their files:
write paths as `$repo/…`, `$testdata/…` or `$tmp/…`, never relative. An example runs on a new
drawing: after drawing something, `answer no` and `menu File|New` first.

## Conventions

- Name pictures `<page>-<n>-<what>.png`, numbered in the order the page shows them.
- A shot of a solid turns the view (`view rotated`) and fits it; pick edges and faces in the view
  where they are easiest to hit, then turn.
- Turn the cursor off for pictures of results; keep it on where a picture shows what to click.
- Zoom To Fit fits what is selected when anything is: fit before picking, not after. It fits the
  solid and what the sketches draw, but not the circles and sketches a screw thread is made of,
  which may run out of the window.
- A click within 10 pixels of a line, a point or a label picks that before the face under it,
  even where the line is behind the face: a reference plane's dashed square crosses most views.
  Check picks with `hover` before relying on them.
- Look at every picture before using it on a page.
