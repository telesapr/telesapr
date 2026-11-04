package ui

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The user manual's pictures are made by interface scenarios, as the
// tests' scripts are, with a few commands of their own: a shot is
// written only when TELESAPR_DOC_SHOTS names a directory, so the
// scenarios run as tests everywhere else and break when the interface
// they show changes.

// docScenarioDir holds the manual's scenarios, one per page.
var docScenarioDir = mustAbs(filepath.Join("..", "..", "doc", "guide", "scenarios"))

// docShot is what a scenario adds to the next picture and nothing
// else: callouts drawn on the copy, never into the application.
type docShot struct {
	dir      string // where pictures go; "" writes none
	cursor   bool   // draw the pointer at the last mouse position
	x, y     float64
	marks    []image.Rectangle
	twCursor bool // the last mouse position is in the property browser
}

// cursorMask is the pointer: 'o' outline, '.' fill.
var cursorMask = []string{
	"o",
	"oo",
	"o.o",
	"o..o",
	"o...o",
	"o....o",
	"o.....o",
	"o......o",
	"o.......o",
	"o........o",
	"o.....ooooo",
	"o..o..o",
	"o.o o..o",
	"oo  o..o",
	"o    o..o",
	"     o..o",
	"      oo",
}

// markColor is the callouts' colour: an orange that no theme uses.
var markColor = color.RGBA{R: 255, G: 140, B: 0, A: 255}

// docStep runs a scenario's own command; false when line is not one.
func (r *uiScriptRunner) docStep(t []string) bool {
	d := &r.doc
	switch t[0] {
	case "size":
		// size gw|tw W H: the window's size in pixels.
		w := r.gw()
		if t[1] == "tw" {
			w = r.tw()
		}
		w.Width, w.Height = scriptFloat(t[2]), scriptFloat(t[3])
		r.ss.GW.Invalidate(true)
		r.ss.TW.Show()
		w.Invalidate()
	case "cursor":
		d.cursor = t[1] == "on"
	case "mark":
		// mark x y w h [tw]: a box round what a step is about.
		x, y := int(scriptFloat(t[1])), int(scriptFloat(t[2]))
		w, h := int(scriptFloat(t[3])), int(scriptFloat(t[4]))
		rc := image.Rect(x, y, x+w, y+h)
		if len(t) > 5 && t[5] == "tw" {
			rc = rc.Add(image.Pt(r.gw().Frame.Bounds().Dx()+docGap, 0))
		}
		d.marks = append(d.marks, rc)
	case "shot":
		// shot name [gw|tw|both]: the picture name.png.
		which := "both"
		if len(t) > 2 {
			which = t[2]
		}
		r.writeShot(t[1], which)
		d.marks = nil
	default:
		return false
	}
	return true
}

// noteMouse keeps where the pointer is, for the cursor of a shot.
func (r *uiScriptRunner) noteMouse(t []string, inTW bool) {
	op := strings.TrimPrefix(t[0], "tw")
	var xs, ys string
	switch op {
	case "move", "scroll":
		xs, ys = t[1], t[2]
	case "down", "up", "dbl":
		xs, ys = t[2], t[3]
	default:
		return
	}
	r.doc.x, r.doc.y = scriptFloat(xs), scriptFloat(ys)
	r.doc.twCursor = inTW
}

// docGap is the space between the two windows of a picture.
const docGap = 8

// writeShot composes the windows, the pointer and the marks, and
// writes the picture.
func (r *uiScriptRunner) writeShot(name, which string) {
	d := &r.doc
	if d.dir == "" {
		return
	}
	r.settle()
	gw, tw := r.gw().Frame, r.tw().Frame
	var img *image.RGBA
	gwOrigin, twOrigin := image.Point{}, image.Point{}
	switch which {
	case "gw":
		img = cloneRGBA(gw)
		twOrigin = image.Pt(-100000, 0)
	case "tw":
		img = cloneRGBA(tw)
		gwOrigin = image.Pt(-100000, 0)
	default:
		gb, tb := gw.Bounds(), tw.Bounds()
		twOrigin = image.Pt(gb.Dx()+docGap, 0)
		img = image.NewRGBA(image.Rect(0, 0, gb.Dx()+docGap+tb.Dx(), max(gb.Dy(), tb.Dy())))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 64, G: 64, B: 64, A: 255}), image.Point{}, draw.Src)
		draw.Draw(img, gb.Add(gwOrigin), gw, gb.Min, draw.Src)
		draw.Draw(img, tb.Add(twOrigin), tw, tb.Min, draw.Src)
	}
	for _, m := range d.marks {
		if which == "tw" {
			m = m.Sub(image.Pt(r.gw().Frame.Bounds().Dx()+docGap, 0))
		}
		strokeRect(img, m, 3)
	}
	if d.cursor {
		o := gwOrigin
		if d.twCursor {
			o = twOrigin
		}
		drawCursor(img, o.X+int(d.x), o.Y+int(d.y))
	}
	if err := writeDocPng(img, filepath.Join(d.dir, name+".png")); err != nil {
		r.t.Fatal(err)
	}
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	return img
}

// strokeRect draws a rectangle's outline w pixels wide, inside it.
func strokeRect(img *image.RGBA, rc image.Rectangle, w int) {
	u := image.NewUniform(markColor)
	for _, side := range []image.Rectangle{
		image.Rect(rc.Min.X, rc.Min.Y, rc.Max.X, rc.Min.Y+w),
		image.Rect(rc.Min.X, rc.Max.Y-w, rc.Max.X, rc.Max.Y),
		image.Rect(rc.Min.X, rc.Min.Y, rc.Min.X+w, rc.Max.Y),
		image.Rect(rc.Max.X-w, rc.Min.Y, rc.Max.X, rc.Max.Y),
	} {
		draw.Draw(img, side.Intersect(img.Bounds()), u, image.Point{}, draw.Src)
	}
}

// drawCursor draws the pointer, at twice its mask's size, its tip at
// x, y.
func drawCursor(img *image.RGBA, x, y int) {
	const s = 2
	for row, line := range cursorMask {
		for col, ch := range line {
			var c color.RGBA
			switch ch {
			case 'o':
				c = color.RGBA{A: 255}
			case '.':
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			default:
				continue
			}
			draw.Draw(img, image.Rect(x+col*s, y+row*s, x+col*s+s, y+row*s+s).Intersect(img.Bounds()),
				image.NewUniform(c), image.Point{}, draw.Src)
		}
	}
}

// runDocScenario runs one of the manual's scenarios, writing its
// pictures into dir, or none when dir is "", and returns what its
// dump, tw, hover and hovergrid lines printed.
func runDocScenario(t *testing.T, path, dir string) string {
	r := newUIScriptRunner(t)
	r.doc.dir = dir
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if !r.step(line) {
			t.Fatalf("%s:%d: the scenario stopped at %q:\n%s", filepath.Base(path), n, line, r.b.String())
		}
		r.settle()
	}
	if strings.Contains(r.b.String(), "menu missing") {
		t.Fatalf("%s: %s", filepath.Base(path), r.b.String())
	}
	return r.b.String()
}

// inTempDir makes a temporary directory the working directory until t
// and its subtests end. t.Chdir would forbid the
// subtests to run in parallel; the top-level parallel tests of the
// package wait until this one has ended, so none sees the change.
func inTempDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// One of its own per run, so runs side by side don't share it; no
	// picture shows it, as the bill of materials names its parts'
	// files relative to the model.
	dir, err := os.MkdirTemp("", "telesapr-guide-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil { //nolint:usetesting // t.Chdir forbids parallel subtests
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil { //nolint:usetesting // as above
			t.Error(err)
		}
		_ = os.RemoveAll(dir)
	})
}

// TestDocScenario prints what the scenario TELESAPR_DOC_SCENARIO
// names prints, for finding where to click while writing one.
func TestDocScenario(t *testing.T) {
	// Not parallel, as TestDocScenarios.
	inTempDir(t)
	name := os.Getenv("TELESAPR_DOC_SCENARIO")
	if name == "" {
		t.Skip("set TELESAPR_DOC_SCENARIO to a scenario name")
	}
	fmt.Print(runDocScenario(t, filepath.Join(docScenarioDir, name+".scenario"), os.Getenv("TELESAPR_DOC_SHOTS")))
}

// TestDocScenarios runs the user manual's scenarios. With
// TELESAPR_DOC_SHOTS=dir it writes their pictures there
// (scripts/regen-doc-images.sh); without, it checks they still run and
// that every picture a page shows is one a scenario makes.
func TestDocScenarios(t *testing.T) {
	// Not parallel: the examples a scenario runs save into the working
	// directory, so the scenarios run in a temporary one.
	inTempDir(t)
	files, err := filepath.Glob(filepath.Join(docScenarioDir, "*.scenario"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no scenarios")
	}
	dir := os.Getenv("TELESAPR_DOC_SHOTS")
	made := map[string]bool{}
	for _, f := range files {
		for _, name := range shotNames(t, f) {
			made[name] = true
		}
		t.Run(strings.TrimSuffix(filepath.Base(f), ".scenario"), func(t *testing.T) {
			t.Parallel()
			runDocScenario(t, f, dir)
		})
	}
	// The real window's pictures (TestDocWindowShots) are made too.
	for _, sh := range docWindowShots {
		made[sh.name] = true
	}
	checkGuideImages(t, made)
}

// shotNames lists the pictures a scenario makes.
func shotNames(t *testing.T, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for line := range strings.Lines(string(data)) {
		if f := strings.Fields(line); len(f) > 1 && f[0] == "shot" {
			names = append(names, f[1])
		}
	}
	return names
}

// checkGuideImages fails on a picture the guide shows that no scenario
// or real-window shot makes.
func checkGuideImages(t *testing.T, made map[string]bool) {
	pages, err := filepath.Glob(filepath.Join(docScenarioDir, "..", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range imageRefs(string(data)) {
			if !made[name] {
				t.Errorf("%s shows images/%s.png, which no scenario makes", filepath.Base(p), name)
			}
		}
	}
}

// imageRefs lists the names of the images/<name>.png a page shows.
func imageRefs(page string) []string {
	var names []string
	for rest := page; ; {
		i := strings.Index(rest, "](images/")
		if i < 0 {
			return names
		}
		rest = rest[i+len("](images/"):]
		if j := strings.Index(rest, ".png)"); j >= 0 {
			names = append(names, rest[:j])
		}
	}
}
