package script

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// tempDir is t.TempDir() with its links followed, in the form the guard
// reports: /private/var on macOS, the long names for the 8.3 ones
// (RUNNER~1) of a Windows runner.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// guardDirs makes folders a script may read, write, and neither, with
// a file in each, and in the writable one a link to the file outside.
func guardDirs(t *testing.T) (read, write, outside string) {
	t.Helper()
	root := tempDir(t)
	read, write, outside = filepath.Join(root, "read"), filepath.Join(root, "write"), filepath.Join(root, "outside")
	for _, d := range []string{read, write, outside} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "f.txt"), []byte("in "+filepath.Base(d)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "f.txt"), filepath.Join(write, "link.txt")); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	return read, write, outside
}

func TestFilesCheck(t *testing.T) {
	t.Parallel()
	read, write, outside := guardDirs(t)
	docDir := tempDir(t)
	f := &Files{Read: []string{read, ""}, Write: []string{write, filepath.Join(outside, "none")}, Folder: func() string { return docDir }}
	for _, c := range []struct {
		path  string
		write bool
		ok    bool
	}{
		{filepath.Join(read, "f.txt"), false, true},
		{filepath.Join(read, "f.txt"), true, false},
		{read, false, true},
		{filepath.Join(write, "f.txt"), true, true},
		{filepath.Join(write, "new", "deeper.txt"), true, true},
		{filepath.Join(docDir, "part.tlspr"), true, true},
		{filepath.Join(outside, "f.txt"), false, false},
		{filepath.Join(write, "link.txt"), false, false},
		{write + "/../outside/f.txt", false, false},
		{write + "/sub/../f.txt", false, false},
		{filepath.Dir(read), false, false},
		{write + "x", true, false},
	} {
		err := f.Check(c.path, c.write)
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrNotAllowed)) {
			t.Errorf("Check(%s, write %v) = %v, want allowed %v", c.path, c.write, err, c.ok)
		}
	}
	err := f.Check(filepath.Join(outside, "f.txt"), true)
	if err == nil || !strings.Contains(err.Error(), "writes files only in "+write) || !strings.Contains(err.Error(), docDir) {
		t.Errorf("a refused write says %v", err)
	}
	if err := (&Files{}).Check(read, false); err == nil || !strings.Contains(err.Error(), "reads no files here") {
		t.Errorf("with no folders a read says %v", err)
	}
	if err := (&Files{}).Check(read, true); err == nil || !strings.Contains(err.Error(), "writes no files here") {
		t.Errorf("with no folders a write says %v", err)
	}
}

// Everything a guarded script can call that takes a string is guarded
// by the engine, guarded by package api, or takes no file's name; a
// new call has to be put in one of those, with why.
func TestGuardCoversFiles(t *testing.T) {
	t.Parallel()
	e, err := New(Options{Host: newHost(), Files: &Files{}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range fileNamingSymbols(e) {
		if strings.HasPrefix(s, "guarded ") {
			continue
		}
		seen[s] = true
		pkg, sym, _ := strings.Cut(s, " ")
		if pkg == apiPath+"/api" && slices.Contains(apiGuarded, sym) {
			continue
		}
		if _, ok := fileless[s]; !ok {
			t.Errorf("%s takes a string: guard it, or say in fileless why it names no file", s)
		}
	}
	for s := range fileless {
		if !seen[s] {
			t.Errorf("fileless lists %s, which a guarded script hasn't, or which is guarded", s)
		}
	}
	for _, sym := range apiGuarded {
		if !seen[apiPath+"/api "+sym] {
			t.Errorf("apiGuarded lists %s, which a guarded script hasn't", sym)
		}
	}

	exports := e.exports(0)
	for _, path := range unguardedPackages {
		if exports[path] != nil {
			t.Errorf("a guarded script has %s", path)
		}
	}
	for _, name := range unguardedAPI {
		if _, ok := exports[apiPath+"/api"][name]; ok {
			t.Errorf("a guarded script has api.%s", name)
		}
	}
	// The generated tables, which every engine shares, are as they were.
	if v := symbols.Symbols["os/os"]["ReadFile"]; v.Pointer() != reflect.ValueOf(os.ReadFile).Pointer() {
		t.Error("guarding an engine changed the generated table of os")
	}
}

// printing is an engine that prints to a buffer.
type printing struct {
	*Engine
	out *bytes.Buffer
}

// newPrinting makes an engine with opts that prints to a buffer.
func newPrinting(t *testing.T, opts Options) printing {
	t.Helper()
	out := &bytes.Buffer{}
	opts.Stdout, opts.Stderr = out, out
	e, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return printing{e, out}
}

// guardedRun runs src in the engine, and returns what it printed and
// its error.
func guardedRun(t *testing.T, e printing, src string) (string, error) {
	t.Helper()
	e.out.Reset()
	shown, err := e.Run(context.Background(), "guarded", src)
	return e.out.String() + shown, err
}

// What a guarded script names outside its folders is refused, whichever
// way it names it; inside them it works.
func TestGuardedScript(t *testing.T) {
	t.Parallel()
	read, write, outside := guardDirs(t)
	temp := tempDir(t)
	e := newPrinting(t, Options{Host: newHost(), Files: &Files{Read: []string{read}, Write: []string{write}, Temp: temp}})
	q := func(path string) string { return `"` + filepath.ToSlash(path) + `"` }
	in, away := filepath.Join(write, "part"), filepath.Join(outside, "part")
	if _, err := guardedRun(t, e, "r := doc.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))\ndoc.Features.Extrude(5, \"block\")\ndoc.Params.Set(\"p\", \"1\")"); err != nil {
		t.Fatal(err)
	}

	refused := []string{
		// The os and filepath calls the engine replaces.
		"os.ReadFile(" + q(filepath.Join(outside, "f.txt")) + ")",
		"os.ReadFile(" + q(filepath.Join(write, "link.txt")) + ")",
		"os.WriteFile(" + q(filepath.Join(read, "g.txt")) + ", nil, 0o644)",
		"os.Stat(" + q(outside) + ")",
		"os.Remove(" + q(filepath.Join(outside, "f.txt")) + ")",
		"os.OpenFile(" + q(filepath.Join(read, "f.txt")) + ", os.O_WRONLY, 0)",
		"os.Rename(" + q(filepath.Join(write, "f.txt")) + ", " + q(filepath.Join(outside, "g.txt")) + ")",
		"os.Symlink(" + q(filepath.Join(outside, "f.txt")) + ", " + q(filepath.Join(write, "l2")) + ")",
		"os.OpenRoot(" + q(read) + ")",
		"os.CreateTemp(" + q(outside) + ", \"x\")",
		"os.Chdir(" + q(write) + ")",
		"os.StartProcess(\"/bin/true\", nil, nil)",
		"filepath.Glob(" + q(filepath.Join(outside, "*")) + ")",
		"filepath.WalkDir(" + q(outside) + ", nil)",
		"fs.ReadFile(os.DirFS(" + q(write) + "), \"link.txt\")",
		"check.ReadBaseline(" + q(filepath.Join(outside, "f.txt")) + ")",
		"Open(" + q(away+".tlspr") + ")",
		"New().File.Save(" + q(away+".tlspr") + ")",
		// The api calls that ask the host (apiGuarded).
		"doc.File.Load(" + q(away+".tlspr") + ")",
		"doc.File.Save(" + q(away+".tlspr") + ")",
		"doc.File.Export(" + q(away+".stl") + ")",
		"doc.File.ExportAs(ExportMesh, " + q(away+".stl") + ")",
		"doc.File.ExportSection(" + q(away+".svg") + ")",
		"doc.File.Render(" + q(away+".png") + ", 10, 10, \"top\")",
		"doc.File.ImportDXF(" + q(away+".dxf") + ")",
		"doc.Drawing.Export(" + q(away+".pdf") + ")",
		"doc.Assembly.ExportBOM(" + q(away+".csv") + ")",
		"doc.Assembly.Link(" + q(away+".tlspr") + ", \"\")",
		"doc.Assembly.LinkStep(" + q(away+".step") + ", \"\", false)",
		"doc.Assembly.MotionStudy(\"p\", 1, 2, 1, " + q(away+"%d.png") + ")",
		"doc.Sketch.Image(" + q(away+".png") + ", V(0, 0, 0), 10)",
		"doc.Host()",
	}
	for _, call := range refused {
		got, err := guardedRun(t, e, "fmt.Println("+call+")")
		if err != nil {
			got += err.Error()
		}
		if !strings.Contains(got, "not allowed") && !strings.Contains(got, "not available to a script whose files are guarded") {
			t.Errorf("%s: %q, want it refused", call, got)
		}
	}

	for _, c := range []struct{ src, want string }{
		{"fmt.Println(doc.File.Save(" + q(in+".tlspr") + "))", "<nil>"},
		{"fmt.Println(doc.File.Export(" + q(in+".stl") + "))", "<nil>"},
		{"b, err := os.ReadFile(" + q(filepath.Join(read, "f.txt")) + ")\nfmt.Println(string(b), err)", "in read <nil>"},
		{"fmt.Println(os.WriteFile(" + q(filepath.Join(write, "g.txt")) + ", []byte(\"x\"), 0o644))", "<nil>"},
		{"b, err := fs.ReadFile(os.DirFS(" + q(write) + "), \"g.txt\")\nfmt.Println(string(b), err)", "x <nil>"},
		{"m, err := filepath.Glob(" + q(filepath.Join(write, "*.txt")) + ")\nfmt.Println(len(m), err)", "3 <nil>"},
		{"fmt.Printf(\"%q %v\\n\", os.Getenv(\"HOME\"), len(os.Environ()))", `"" 0`},
		{"f, err := os.CreateTemp(\"\", \"x\")\nfmt.Println(filepath.Dir(f.Name()) == " + q(temp) + ", err, f.Close())", "true <nil> <nil>"},
		{"d, err := os.MkdirTemp(\"\", \"x\")\nfmt.Println(filepath.Dir(d) == " + q(temp) + ", err)", "true <nil>"},
		{"d, err := Open(" + q(in+".tlspr") + ")\nfmt.Println(err, d.File.Save(" + q(away+".tlspr") + ") != nil)", "<nil> true"},
	} {
		got, err := guardedRun(t, e, c.src)
		if err != nil || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, %v; want %q", c.src, got, err, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "f.txt")); err != nil {
		t.Errorf("the file outside is gone: %v", err)
	}

	for _, imp := range []string{"github.com/telesapr/telesapr/pkg/sketch", "github.com/telesapr/telesapr/pkg/persist/tlspr"} {
		if _, err := guardedRun(t, e, `import "`+imp+`"`); err == nil {
			t.Errorf("a guarded script imported %s", imp)
		}
	}
	if _, err := guardedRun(t, e, "NewDoc(nil)"); err == nil {
		t.Error("a guarded script has NewDoc")
	}
}

// An unguarded engine's script has what a guarded one hasn't.
func TestUnguardedScript(t *testing.T) {
	t.Parallel()
	_, _, outside := guardDirs(t)
	e := newPrinting(t, Options{Host: newHost()})
	got, err := guardedRun(t, e, "b, err := os.ReadFile(\""+filepath.ToSlash(filepath.Join(outside, "f.txt"))+"\")\nfmt.Println(string(b), err, doc.Host() != nil)")
	if err != nil || !strings.Contains(got, "in outside <nil> true") {
		t.Errorf("an unguarded script read %q, %v", got, err)
	}
}
