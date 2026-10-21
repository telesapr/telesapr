package script

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeLibrary makes a library folder with the package gears, which
// draws on the document it is given, and shop/size, which gears uses.
func writeLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"gears/gears.go": `package gears

import (
	"github.com/telesapr/telesapr/pkg/script/api"

	"shop/size"
)

// Disc draws a circle of the module's pitch diameter.
func Disc(doc *api.Doc, teeth int) api.Entity {
	return doc.Sketch.Circle(api.V(0, 0, 0), size.Pitch(teeth)/2)
}
`,
		"shop/size/size.go": `package size

// Module is the gears' module, in mm.
const Module = 2.0

// Pitch is the pitch diameter of a gear of n teeth.
func Pitch(n int) float64 { return Module * float64(n) }
`,
	}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A console, a script given as statements and a whole program import
// the library's packages, and they import each other.
func TestLibraryImports(t *testing.T) {
	t.Parallel()
	lib := writeLibrary(t)
	var out bytes.Buffer
	opts := Options{Host: newHost(), Stdout: &out, Stderr: &out, Library: lib}

	e, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	shown, err := e.Run(ctx, "console", "import \"gears\"\ngears.Disc(doc, 20).Radius()")
	if err != nil || shown != "20" {
		t.Errorf("the library's disc has radius %s (%v), want 20", shown, err)
	}

	src := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"shop/size\"\n)\n\nfunc main() { fmt.Println(size.Pitch(12)) }\n"
	if err := RunSource(ctx, opts, "prog.go", src); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "24") {
		t.Errorf("the program printed %q, want 24", out.String())
	}
}

// An import that is in no library says so, and names the library.
func TestLibraryMissingPackage(t *testing.T) {
	t.Parallel()
	lib := writeLibrary(t)
	for _, tc := range []struct{ lib, want string }{
		{lib, "nor in the script library " + lib},
		{"", "no script library is set"},
	} {
		e, err := New(Options{Host: newHost(), Library: tc.lib})
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Run(context.Background(), "console", `import "nothere"`)
		if !errors.Is(err, errNoPackage) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("with library %q the import gave %v, want one saying %q", tc.lib, err, tc.want)
		}
	}
}

// The library can be changed while the engine lives, and the
// interpreter made again after a cancelled step still has it.
func TestLibraryAfterRestart(t *testing.T) {
	t.Parallel()
	lib := writeLibrary(t)
	e, err := New(Options{Host: newHost()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := e.Run(ctx, "console", `import "gears"`); err == nil {
		t.Fatal("gears imported with no library")
	}
	e.SetLibrary(lib)
	if got := e.Library(); got != lib {
		t.Errorf("Library is %q, want %q", got, lib)
	}

	done := make(chan error, 1)
	go func() {
		_, err := e.Eval(ctx, "for { time.Sleep(time.Millisecond) }")
		done <- err
	}()
	for !e.Running() {
		time.Sleep(time.Millisecond)
	}
	e.Cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("the cancelled step gave %v", err)
	}
	shown, err := e.Run(ctx, "console", "import \"gears\"\ngears.Disc(doc, 10).Radius()")
	if err != nil || shown != "10" {
		t.Errorf("after the restart the disc has radius %s (%v), want 10", shown, err)
	}
}

// The default library is beside the settings, and none where there is
// no configuration directory.
func TestDefaultLibrary(t *testing.T) {
	// It reads the environment, so not in parallel.
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no configuration directory here")
	}
	if got, want := DefaultLibrary(), filepath.Join(dir, "telesapr", "library"); got != want {
		t.Errorf("DefaultLibrary() = %q, want %q", got, want)
	}
	if runtime.GOOS != "linux" {
		return
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if got := DefaultLibrary(); got != "" {
		t.Errorf("with no configuration directory DefaultLibrary() = %q, want none", got)
	}
}
