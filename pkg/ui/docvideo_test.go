package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/res"
)

// A scenario records a video of some of its steps between `video
// start NAME` and `video end`, when TELESAPR_DOC_VIDEOS names a
// directory (scripts/regen-guide-videos.sh): a frame after each step,
// the pointer gliding between the places it is moved to, a ring where
// it presses, the keys pressed, and a caption under the windows. The
// frames go to ffmpeg, which writes NAME.webm. Without the variable the
// commands do nothing, so the scenarios run as tests as before. Videos
// are not goldens: ffmpeg's bytes differ between its versions.

// docVideo is a video being recorded.
type docVideo struct {
	dir     string // where videos go; "" records none
	name    string // the video being recorded; "" when none is
	which   string // gw, tw or both, as for a shot
	frames  string // the directory its frames are written to
	list    bytes.Buffer
	n       int     // frames written
	last    []byte  // the last frame's pixels, to hold it rather than repeat it
	lastDur float64 // how long the last frame is shown
	caption string
	key     string // the key the step pressed, shown on its frame
	click   bool   // the step pressed a button, ringed on its frame
	font    *res.BitmapFont
}

// The pace of a video, in seconds.
const (
	videoFrame    = 1.0 / 25 // one frame of the pointer's glide
	videoStep     = 0.5      // a step's result
	videoPress    = 0.35     // a click's ring
	videoKey      = 0.8      // a key pressed
	videoShot     = 1.6      // a result the page pictures
	videoCaption  = 1.4      // a new caption, to read it
	captionHeight = 32       // the band under the windows the caption is in
)

// videoCommand runs `video start NAME [gw|tw|both]`, `video end`,
// `caption TEXT` and `pause SECONDS`.
func (r *uiScriptRunner) videoCommand(t []string) {
	v := &r.doc.video
	switch t[0] {
	case "video":
		if len(t) > 1 && t[1] == "start" && len(t) > 2 {
			if v.dir == "" {
				return
			}
			which := "both"
			if len(t) > 3 {
				which = t[3]
			}
			r.videoStart(t[2], which)
			return
		}
		if len(t) > 1 && t[1] == "end" {
			r.videoEnd()
			return
		}
		r.t.Fatalf("video start NAME [gw|tw|both] or video end, not %q", strings.Join(t, " "))
	case "caption":
		v.caption = strings.Join(t[1:], " ")
		if v.name != "" {
			r.videoFrameOf(nil, videoCaption)
		}
	case "pause":
		if len(t) < 2 {
			r.t.Fatal("pause SECONDS")
		}
		if v.name != "" {
			v.lastDur += scriptFloat(t[1])
		}
	}
}

func (r *uiScriptRunner) videoStart(name, which string) {
	v := &r.doc.video
	if v.name != "" {
		r.t.Fatalf("video %s started inside video %s", name, v.name)
	}
	dir, err := os.MkdirTemp("", "telesapr-video-")
	if err != nil {
		r.t.Fatal(err)
	}
	*v = docVideo{dir: v.dir, name: name, which: which, frames: dir, font: v.font}
	if v.font == nil {
		v.font = res.NewBitmapFont()
	}
	r.settle()
	r.videoFrameOf(nil, videoStep)
}

// videoEnd hands the frames to ffmpeg.
func (r *uiScriptRunner) videoEnd() {
	v := &r.doc.video
	if v.name == "" {
		return
	}
	defer func() { _ = os.RemoveAll(v.frames) }()
	r.flushFrame()
	// The concat demuxer shows the last file for no time unless it
	// comes again.
	fmt.Fprintf(&v.list, "file 'f%05d.png'\n", v.n-1)
	listPath := filepath.Join(v.frames, "list.txt")
	if err := os.WriteFile(listPath, v.list.Bytes(), 0o644); err != nil {
		r.t.Fatal(err)
	}
	out := filepath.Join(v.dir, v.name+".webm")
	// VP9 at a constant quality: the windows are flat colour and
	// still between steps, so the frames that hold cost next to
	// nothing.
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-vf", "fps=25,format=yuv420p", "-c:v", "libvpx-vp9", "-b:v", "0", "-crf", "42",
		"-row-mt", "1", "-deadline", "good", "-an", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("ffmpeg for %s: %v\n%s", out, err, b)
	}
	v.name = ""
}

// videoMouse glides the pointer to x, y before a move, and notes a
// press.
func (r *uiScriptRunner) videoMouse(op string, x, y float64, inTW bool) {
	v := &r.doc.video
	if v.name == "" {
		return
	}
	switch op {
	case "down", "dbl":
		v.click = true
	}
	from, ok := r.videoPointer(r.doc.x, r.doc.y, r.doc.twCursor)
	to, _ := r.videoPointer(x, y, inTW)
	if !ok || from == to {
		return
	}
	dist := math.Hypot(float64(to.X-from.X), float64(to.Y-from.Y))
	n := min(max(int(dist/30), 4), 14)
	r.settle()
	base := r.videoBase()
	for i := 1; i <= n; i++ {
		s := float64(i) / float64(n)
		s = s * s * (3 - 2*s) // eased at both ends
		p := image.Pt(from.X+int(math.Round(s*float64(to.X-from.X))), from.Y+int(math.Round(s*float64(to.Y-from.Y))))
		img := cloneRGBA(base)
		drawCursor(img, p.X, p.Y)
		r.videoFrameOf(img, videoFrame)
	}
}

// videoPointer is where a mouse position lies in the video's frame;
// false before the first move.
func (r *uiScriptRunner) videoPointer(x, y float64, inTW bool) (image.Point, bool) {
	_, gwOrigin, twOrigin := r.videoOrigins()
	o := gwOrigin
	if inTW {
		o = twOrigin
	}
	return image.Pt(o.X+int(x), o.Y+int(y)), x != 0 || y != 0
}

func (r *uiScriptRunner) videoOrigins() (size image.Point, gwOrigin, twOrigin image.Point) {
	gb, tb := r.gw().Frame.Bounds(), r.tw().Frame.Bounds()
	switch r.doc.video.which {
	case "gw":
		return gb.Size(), image.Point{}, image.Pt(-100000, 0)
	case "tw":
		return tb.Size(), image.Pt(-100000, 0), image.Point{}
	}
	return image.Pt(gb.Dx()+docGap+tb.Dx(), max(gb.Dy(), tb.Dy())), image.Point{}, image.Pt(gb.Dx()+docGap, 0)
}

// videoBase is what the windows show, with the caption band under them
// and no pointer.
func (r *uiScriptRunner) videoBase() *image.RGBA {
	v := &r.doc.video
	shot, _, _ := r.composeShot(v.which)
	b := shot.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()+captionHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 32, G: 32, B: 32, A: 255}), image.Point{}, draw.Src)
	draw.Draw(img, b, shot, b.Min, draw.Src)
	r.videoText(img, v.caption, 12, b.Dy()+(captionHeight-16)/2, color.RGBA{R: 240, G: 240, B: 240, A: 255})
	return img
}

// videoStep adds the frame a step leaves, held as long as the step
// asks to be looked at.
func (r *uiScriptRunner) videoStep(line string) {
	v := &r.doc.video
	if v.name == "" {
		return
	}
	t := strings.Fields(line)
	dur := videoStep
	switch t[0] {
	case "video", "caption", "pause", "cursor", "mark", "dump", "tw", "hover", "hovergrid", "answer", "form", "file", "popup":
		// Nothing to see, or seen already.
		v.key, v.click = "", false
		return
	case "shot":
		v.lastDur += videoShot - videoStep
		return
	case "key":
		v.key = keyLabel(t[1:])
		dur = videoKey
	case "edit", "menu":
		dur = videoKey
	}
	base := r.videoBase()
	p, ok := r.videoPointer(r.doc.x, r.doc.y, r.doc.twCursor)
	if v.click && ok {
		img := cloneRGBA(base)
		drawRing(img, p, 14)
		drawCursor(img, p.X, p.Y)
		r.videoKeyBadge(img)
		r.videoFrameOf(img, videoPress)
		dur = max(dur-videoPress, videoFrame)
	}
	if ok {
		drawCursor(base, p.X, p.Y)
	}
	r.videoKeyBadge(base)
	r.videoFrameOf(base, dur)
	v.key, v.click = "", false
}

// keyLabel is a key as a page names it: Shift+X, Ctrl+Z, Esc.
func keyLabel(t []string) string {
	if len(t) == 0 {
		return ""
	}
	k := t[0]
	switch {
	case len(k) == 1:
		k = strings.ToUpper(k)
	case k == "esc":
		k = "Esc"
	case k == "del":
		k = "Del"
	case k == "ret":
		k = "Enter"
	case k == "bs":
		k = "Backspace"
	case k == "tab", k == "space", k == "left", k == "right", k == "up", k == "down":
		k = strings.ToUpper(k[:1]) + k[1:]
	}
	for _, m := range t[1:] {
		k = strings.ToUpper(m[:1]) + m[1:] + "+" + k
	}
	return k
}

// videoKeyBadge shows the key the step pressed at the caption band's
// right-hand end.
func (r *uiScriptRunner) videoKeyBadge(img *image.RGBA) {
	v := &r.doc.video
	if v.key == "" {
		return
	}
	w := 8*len(v.key) + 16
	b := img.Bounds()
	rc := image.Rect(b.Max.X-w-12, b.Max.Y-captionHeight+4, b.Max.X-12, b.Max.Y-4)
	draw.Draw(img, rc, image.NewUniform(color.RGBA{R: 240, G: 240, B: 240, A: 255}), image.Point{}, draw.Src)
	r.videoText(img, v.key, rc.Min.X+8, rc.Min.Y+(rc.Dy()-16)/2, color.RGBA{R: 16, G: 16, B: 16, A: 255})
}

// videoText draws s in the property browser's font, its top left at x,
// y.
func (r *uiScriptRunner) videoText(img *image.RGBA, s string, x, y int, c color.RGBA) {
	f := r.doc.video.font
	tex := f.Texture
	for _, ch := range s {
		s0, t0, _, _, w, h := f.LocateGlyph(ch)
		tx, ty := int(math.Round(s0*float64(tex.Width))), int(math.Round(t0*float64(tex.Height)))
		for row := range h {
			for col := range w {
				if tex.Data[tex.Stride*(ty+row)+tx+col] != 0 {
					if p := image.Pt(x+col, y+row); p.In(img.Bounds()) {
						img.SetRGBA(p.X, p.Y, c)
					}
				}
			}
		}
		x += w
	}
}

// drawRing draws a ring of radius rad round p, in the callouts' colour.
func drawRing(img *image.RGBA, p image.Point, rad int) {
	for dy := -rad - 2; dy <= rad+2; dy++ {
		for dx := -rad - 2; dx <= rad+2; dx++ {
			d := math.Hypot(float64(dx), float64(dy))
			if d >= float64(rad)-1.5 && d <= float64(rad)+1.5 {
				if q := image.Pt(p.X+dx, p.Y+dy); q.In(img.Bounds()) {
					img.SetRGBA(q.X, q.Y, markColor)
				}
			}
		}
	}
}

// videoFrameOf adds a frame shown for dur seconds: img, or with nil
// what the windows show now with the pointer. A frame the same as the
// last one only holds that one longer.
func (r *uiScriptRunner) videoFrameOf(img *image.RGBA, dur float64) {
	v := &r.doc.video
	if img == nil {
		img = r.videoBase()
		if p, ok := r.videoPointer(r.doc.x, r.doc.y, r.doc.twCursor); ok {
			drawCursor(img, p.X, p.Y)
		}
	}
	if v.last != nil && bytes.Equal(v.last, img.Pix) {
		v.lastDur += dur
		return
	}
	r.flushFrame()
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.frames, fmt.Sprintf("f%05d.png", v.n)), buf.Bytes(), 0o644); err != nil {
		r.t.Fatal(err)
	}
	v.n++
	v.last, v.lastDur = img.Pix, dur
}

// flushFrame writes the last frame's line of the concat list, now that
// how long it is shown is known.
func (r *uiScriptRunner) flushFrame() {
	v := &r.doc.video
	if v.last == nil {
		return
	}
	fmt.Fprintf(&v.list, "file 'f%05d.png'\nduration %s\n", v.n-1, strconv.FormatFloat(v.lastDur, 'f', 3, 64))
}

// videoNames lists the videos a scenario records.
func videoNames(t *testing.T, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for line := range strings.Lines(string(data)) {
		if f := strings.Fields(line); len(f) > 2 && f[0] == "video" && f[1] == "start" {
			names = append(names, f[2])
		}
	}
	return names
}

// checkGuideVideos fails on a video the guide shows that no scenario
// records, or one in videos/ that no page shows.
func checkGuideVideos(t *testing.T, made map[string]bool) {
	pages, err := filepath.Glob(filepath.Join(docScenarioDir, "..", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	shown := map[string]bool{}
	for _, p := range pages {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range videoRefs(string(data)) {
			shown[name] = true
			if !made[name] {
				t.Errorf("%s shows videos/%s.webm, which no scenario records", filepath.Base(p), name)
			}
		}
	}
	files, err := filepath.Glob(filepath.Join(docScenarioDir, "..", "videos", "*.webm"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if name := strings.TrimSuffix(filepath.Base(f), ".webm"); !shown[name] || !made[name] {
			t.Errorf("videos/%s.webm is shown by no page or recorded by no scenario", name)
		}
	}
}

// videoRefs lists the names of the videos/<name>.webm a page shows.
func videoRefs(page string) []string {
	var names []string
	for rest := page; ; {
		i := strings.Index(rest, "videos/")
		if i < 0 {
			return names
		}
		rest = rest[i+len("videos/"):]
		if j := strings.Index(rest, ".webm"); j >= 0 && !strings.ContainsAny(rest[:j], " )\"\n") {
			names = append(names, rest[:j])
		}
	}
}

func TestKeyLabel(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"r": "R", "x shift": "Shift+X", "z ctrl": "Ctrl+Z", "esc": "Esc", "ret": "Enter", "F3": "F3", "left": "Left",
	} {
		if got := keyLabel(strings.Fields(in)); got != want {
			t.Errorf("keyLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVideoRefs(t *testing.T) {
	t.Parallel()
	page := "<video src=\"videos/a-b.webm\" controls></video>\n([a-b.webm](videos/a-b.webm))\nvideos/ and more.webm"
	got := videoRefs(page)
	if len(got) != 2 || got[0] != "a-b" || got[1] != "a-b" {
		t.Errorf("videoRefs = %q", got)
	}
}

// TestDocVideo records a short video through ffmpeg, as
// scripts/regen-guide-videos.sh does.
func TestDocVideo(t *testing.T) {
	// Not parallel: it sets TELESAPR_DOC_VIDEOS.
	if testing.Short() {
		t.Skip("records a video through ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	t.Setenv("TELESAPR_DOC_VIDEOS", dir)
	scenario := filepath.Join(t.TempDir(), "v.scenario")
	if err := os.WriteFile(scenario, []byte(`video start v gw
caption A rectangle.
key r
move 200 200
down L 200 200
up L 200 200
move 380 330
down L 380 330
up L 380 330
key esc
shot v
pause 0.5
video end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	runDocScenario(t, scenario, "")
	fi, err := os.Stat(filepath.Join(dir, "v.webm"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() < 1000 {
		t.Errorf("v.webm is %d bytes", fi.Size())
	}
}
