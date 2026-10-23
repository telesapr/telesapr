package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runScripting runs a scripting command and returns its exit status and
// what it wrote.
func runScripting(t *testing.T, stdin string, args ...string) (status int, stdout, stderr string) {
	t.Helper()
	var out, errs bytes.Buffer
	status = runScript(context.Background(), append([]string{"telesapr-cli"}, args...),
		strings.NewReader(stdin), &out, &errs)
	return status, out.String(), errs.String()
}

const blockScript = `package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	size, _ := strconv.ParseFloat(os.Args[1], 64)
	doc := api.Current()
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(size, size, 0))
	doc.Constrain.Length(sides[0], size)
	doc.Constrain.Length(sides[1], size)
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
	block := doc.Features.Extrude(size, "block")
	fmt.Printf("%s %.0f\n", block, block.Analyze().Volume())
	for _, name := range os.Args[2:] {
		var err error
		if filepath := name; len(filepath) > 6 && filepath[len(filepath)-6:] == ".tlspr" {
			err = doc.File.Save(name)
		} else {
			err = doc.File.Export(name)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed:", err)
			os.Exit(4)
		}
	}
}
`

// A script makes a part from its arguments, saves and exports it, and
// another command reads what it saved.
func TestRunScript(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "block.go")
	if err := os.WriteFile(file, []byte(blockScript), 0o644); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "block.tlspr")
	outputs := []string{model, filepath.Join(dir, "block.stl"), filepath.Join(dir, "block.step"),
		filepath.Join(dir, "block.svg"), filepath.Join(dir, "block.png")}

	status, stdout, stderr := runScripting(t, "", append([]string{"run", file, "10"}, outputs...)...)
	if status != exitOK || stdout != "g003-block 1000\n" || stderr != "" {
		t.Fatalf("run: status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
	for _, name := range outputs {
		if info, err := os.Stat(name); err != nil || info.Size() == 0 {
			t.Errorf("%s wasn't written: %v", filepath.Base(name), err)
		}
	}

	status, stdout, stderr = runScripting(t, "", "eval", model,
		`g := doc.Group("block")`, `math.Round(g.Analyze().Volume())`, `g.Analyze().Bounds()`, `len(doc.Groups())`)
	if status != exitOK || stdout != "1000\n(0, 0, 0), (10, 10, 10)\n2\n" || stderr != "" {
		t.Errorf("eval: status %d, stdout %q, stderr %q", status, stdout, stderr)
	}

	// The saved sketch is one the other commands take.
	var errs bytes.Buffer
	if !runCommand([]string{"telesapr-cli", "export-mesh", "-o", filepath.Join(dir, "again.stl"), model}, &errs, nil) {
		t.Errorf("export-mesh of the script's sketch failed: %s", errs.String())
	}

	// An export that fails is the script's to report.
	status, _, stderr = runScripting(t, "", "run", file, "10", filepath.Join(dir, "none", "block.stl"))
	if status != 4 || !strings.HasPrefix(stderr, "failed: couldn't write to") {
		t.Errorf("failed export: status %d, stderr %q", status, stderr)
	}
}

func TestRepl(t *testing.T) {
	status, stdout, stderr := runScripting(t, "c := doc.Sketch.Circle(V(1, 2, 0), 3)\nc.Radius()\nc.Center().Pos()\nc.Start()\n", "repl")
	want := "3\n(1, 2, 0)\nerror: circle r004-circle, radius 3.000 has no start\n"
	if status != exitOK || stdout != want || stderr != "" {
		t.Errorf("repl: status %d, stdout %q, stderr %q, want stdout %q", status, stdout, stderr, want)
	}
}

func TestScriptFailures(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(bad, []byte("x := 1\n\nfmt.Println(x, y)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gears := filepath.Join(dir, "gears.go")
	if err := os.WriteFile(gears, []byte("import \"gears\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args   []string
		status int
		stderr string
	}{
		{[]string{"run"}, exitUsage, "A script must be specified.\n"},
		{[]string{"eval"}, exitUsage, "Code to run must be specified.\n"},
		{[]string{"repl", "-x"}, exitUsage, "Unrecognized option '-x'.\n"},
		{[]string{"run", "--lib"}, exitUsage, "--lib needs the library's folder.\n"},
		{[]string{"run", "--set"}, exitUsage, "--set needs a field and its value"},
		{[]string{"run", "--set", "teeth"}, exitUsage, "--set needs a field and its value"},
		{[]string{"eval", "--set", "a=1", "1"}, exitUsage, "--set is for run"},
		{[]string{"run", "--set", "a=1", bad}, exitScript, "the script declares no form"},
		{[]string{"run", "--lib", dir, gears}, exitScript, "nor in the script library " + dir},
		{[]string{"run", filepath.Join(dir, "none.tlspr"), bad}, exitLoad, "Cannot load"},
		{[]string{"run", filepath.Join("..", "..", "testdata", "request", "line_segment", "normal.slvs"), bad}, exitLoad, "telesapr-cli convert"},
		{[]string{"run", filepath.Join(dir, "none.go")}, exitScript, "no such file"},
		{[]string{"run", bad}, exitScript, bad + ":3:16: undefined: y\n"},
		{[]string{"eval", "os.Exit(9)"}, 9, ""},
		{[]string{"eval", "if true {"}, exitScript, "the code ends in the middle of a statement\n"},
		{[]string{"eval", "1 + 1", "doc.Sketch.Circle(V(0, 0, 0), 1).Start()", "2"}, exitScript, ""},
	} {
		status, _, stderr := runScripting(t, "", c.args...)
		if status != c.status || !strings.Contains(stderr, c.stderr) {
			t.Errorf("%v: status %d, stderr %q; want %d, %q", c.args, status, stderr, c.status, c.stderr)
		}
	}
}

// --lib names the folder a script's own packages are imported from.
func TestScriptLibrary(t *testing.T) {
	lib, err := filepath.Abs(filepath.Join("..", "..", "examples", "scripts", "library", "lib"))
	if err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr := runScripting(t, "", "eval", "--lib", lib, `import "gears"`,
		`gears.Blank(doc, 10, 2, 4, 3, "gear").Kind()`)
	if status != exitOK || stdout != "\"extrude\"\n" {
		t.Errorf("eval --lib: status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
}

// --set fills the form a script declares.
func TestScriptForm(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "examples", "scripts", "form", "form.go"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	status, stdout, stderr := runScripting(t, "", "run", "--set", "slots=4", "--set", "save=false", src)
	if status != exitOK || !strings.Contains(stdout, ": 4 slots, volume ") {
		t.Errorf("run --set: status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
	if _, err := os.Stat("washer.tlspr"); err == nil {
		t.Error("the washer was saved, though save was false")
	}
	status, _, stderr = runScripting(t, "", "run", "--set", "slots=many", src)
	if status != exitScript || !strings.Contains(stderr, "slots is a whole number") {
		t.Errorf("a value that doesn't fit: status %d, stderr %q", status, stderr)
	}
}
