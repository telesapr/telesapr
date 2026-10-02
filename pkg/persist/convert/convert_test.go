package convert_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/convert"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// linkFixture is a model that links rect_v20.slvs, read from testdata.
func linkFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdata, "group", "link", "normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeLinking writes the link fixture to path, linking link.
func writeLinking(t *testing.T, path, link string) {
	t.Helper()
	data := strings.Replace(linkFixture(t), "Group.impFileRel=rect_v20.slvs", "Group.impFileRel="+link, 1)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeRect writes the model the link fixture links to path.
func writeRect(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdata, "group", "link", "rect_v20.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanAndRun(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "asm.slvs")
	writeLinking(t, root, "parts/rect.slvs")
	writeRect(t, filepath.Join(dir, "parts", "rect.slvs"))
	dst := filepath.Join(dir, "out", "asm.spcs")
	if err := os.Mkdir(filepath.Join(dir, "out"), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := convert.MakePlan(root, dst)
	if err != nil {
		t.Fatal(err)
	}
	rect := filepath.Join(dir, "parts", "rect.slvs")
	want := []convert.Item{
		{Src: rect, Dst: filepath.Join(dir, "parts", "rect.spcs"), LinkedFrom: root},
		{Src: root, Dst: dst, Root: true},
	}
	if !reflect.DeepEqual(p.Items, want) {
		t.Fatalf("plan %+v\nwant %+v", p.Items, want)
	}
	if p.Root() != want[1] || len(p.Existing()) != 0 || len(p.Missing()) != 0 {
		t.Errorf("root %+v, existing %v, missing %v", p.Root(), p.Existing(), p.Missing())
	}

	r, err := convert.Run(p, convert.Options{Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Written, []string{want[0].Dst, dst}) || len(r.Reused)+len(r.Failed)+len(r.Warnings) > 0 {
		t.Errorf("result %+v", r)
	}
	// The root moved to another folder: its link is rewritten from
	// there.
	doc, err := spcs.LoadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range doc.Sketch.Groups {
		if g.Type == sketch.GroupLinked && (g.LinkFile != "../parts/rect.spcs" || len(g.ImpEntity) == 0) {
			t.Errorf("link %q with %d entities", g.LinkFile, len(g.ImpEntity))
		}
	}

	// Again: the linked copy exists, and is reused unless overwritten.
	p, err = convert.MakePlan(root, dst)
	if err != nil {
		t.Fatal(err)
	}
	if ex := p.Existing(); len(ex) != 1 || ex[0].Src != rect || !p.Root().Exists {
		t.Errorf("existing %+v, root %+v", ex, p.Root())
	}
	r, err = convert.Run(p, convert.Options{})
	if err != nil || !reflect.DeepEqual(r.Reused, []string{want[0].Dst}) || !reflect.DeepEqual(r.Written, []string{dst}) {
		t.Errorf("reuse: %v %+v", err, r)
	}
	r, err = convert.Run(p, convert.Options{Overwrite: true})
	if err != nil || len(r.Reused) != 0 || len(r.Written) != 2 {
		t.Errorf("overwrite: %v %+v", err, r)
	}
}

func TestPlanErrors(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "a.slvs")
	writeLinking(t, root, "b.slvs")
	writeLinking(t, filepath.Join(dir, "b.slvs"), "c.slvs")
	writeLinking(t, filepath.Join(dir, "c.slvs"), "b.slvs")

	_, err := convert.MakePlan(root, filepath.Join(dir, "a.spcs"))
	var ce *convert.CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("cycle: %v", err)
	}
	b, c := filepath.Join(dir, "b.slvs"), filepath.Join(dir, "c.slvs")
	if !reflect.DeepEqual(ce.Chain, []string{b, c, b}) || !strings.Contains(err.Error(), b+" -> "+c+" -> "+b) {
		t.Errorf("chain %v: %v", ce.Chain, err)
	}

	empty := filepath.Join(dir, "empty.slvs")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.slvs")
	if err := os.WriteFile(broken, []byte("\xb1\xb2\xb3SolveSpaceREVa\n\nGroup.type=5000\nAddGroup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ src, dst, want string }{
		{filepath.Join(dir, "a.spcs"), filepath.Join(dir, "x.spcs"), "is not a SolveSpace model"},
		{root, filepath.Join(dir, "x.slvs"), "is not a spacesolve file"},
		{filepath.Join(dir, "none.slvs"), filepath.Join(dir, "x.spcs"), "no such file"},
		{empty, filepath.Join(dir, "x.spcs"), "the file is empty"},
		{broken, filepath.Join(dir, "x.spcs"), "group missing handle"},
	} {
		if _, err := convert.MakePlan(c.src, c.dst); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("MakePlan(%s, %s): %v, want %q", filepath.Base(c.src), filepath.Base(c.dst), err, c.want)
		}
	}
}

// TestMissingLink: a linked model that can't be read is in the plan as
// missing, and the model linking it is converted with its link pointing
// at the copy that would have been made.
func TestMissingLink(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "asm.slvs")
	writeLinking(t, root, "gone.slvs")
	p, err := convert.MakePlan(root, filepath.Join(dir, "asm.spcs"))
	if err != nil {
		t.Fatal(err)
	}
	m := p.Missing()
	if len(m) != 1 || m[0].Src != filepath.Join(dir, "gone.slvs") || m[0].LinkedFrom != root || m[0].Err == nil {
		t.Fatalf("missing %+v", m)
	}
	r, err := convert.Run(p, convert.Options{Quiet: true})
	if err != nil || len(r.Written) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	doc, err := spcs.LoadFile(r.Written[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range doc.Sketch.Groups {
		if g.Type == sketch.GroupLinked && g.LinkFile != "gone.spcs" {
			t.Errorf("link %q", g.LinkFile)
		}
	}
}

// TestReadOnlyDirectory: a linked model whose copy can't be written is
// reported, and the rest is converted.
func TestReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions don't bind root")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "asm.slvs")
	writeLinking(t, root, "locked/rect.slvs")
	writeRect(t, filepath.Join(dir, "locked", "rect.slvs"))
	locked := filepath.Join(dir, "locked")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	p, r, err := convert.File(root, filepath.Join(dir, "asm.spcs"), false)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(locked, "rect.spcs")
	if len(p.Items) != 2 || r.Failed[copyPath] == nil || !reflect.DeepEqual(r.Written, []string{filepath.Join(dir, "asm.spcs")}) {
		t.Errorf("plan %+v, result %+v", p.Items, r)
	}

	// The root itself can't be written: that is an error.
	if _, _, err := convert.File(filepath.Join(locked, "rect.slvs"), copyPath, false); err == nil {
		t.Error("wrote into a read-only directory")
	}
}

// TestLoadErrorWarns: a model with lines SolveSpace doesn't know
// converts what it can, and says so.
func TestLoadErrorWarns(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "odd.slvs")
	writeLinking(t, root, "rect.slvs")
	writeRect(t, filepath.Join(dir, "rect.slvs"))
	data, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, append(data, "Nonsense\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, err := convert.File(root, filepath.Join(dir, "odd.spcs"), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "lines that are not SolveSpace records were skipped") {
		t.Errorf("warnings %v", r.Warnings)
	}
}

// TestUnregenerable: a model that can't regenerate fails, and isn't
// written.
func TestUnregenerable(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "bad.slvs")
	// A line segment in a workplane that doesn't exist.
	model := "\xb1\xb2\xb3SolveSpaceREVa\n\nGroup.h.v=00000001\nGroup.type=5000\nGroup.name=#references\nAddGroup\n\n" +
		"Request.h.v=00000004\nRequest.type=200\nRequest.workplane.v=80020000\nRequest.group.v=00000001\nAddRequest\n\n"
	if err := os.WriteFile(root, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := convert.File(root, filepath.Join(dir, "bad.spcs"), false)
	if err == nil || !strings.Contains(err.Error(), "can't regenerate") {
		t.Errorf("error %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.spcs")); err == nil {
		t.Error("an unregenerable model was written")
	}
}
