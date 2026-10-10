package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTakeProfileFlags(t *testing.T) {
	t.Parallel()
	args, p, err := takeProfileFlags([]string{"telesapr-cli", "regenerate", "--cpuprofile", "c.prof",
		"a.tlspr", "--trace", "t.out", "--memprofile", "m.prof", "--chord-tol", "0.5"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"telesapr-cli", "regenerate", "a.tlspr", "--chord-tol", "0.5"}; !slices.Equal(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
	if want := (profiles{cpu: "c.prof", mem: "m.prof", trace: "t.out"}); p != want {
		t.Errorf("profiles = %+v, want %+v", p, want)
	}
	if _, _, err := takeProfileFlags([]string{"telesapr-cli", "version", "--trace"}); err == nil {
		t.Error("--trace with no file was taken")
	}
}

// TestProfiles regenerates a model with every profile asked for, and
// finds each written.
func TestProfiles(t *testing.T) {
	// runtime.MemProfileRate, which --memprofile sets, is the process's.
	in := model(t, "group/force_to_mesh/union.slvs")
	dir := t.TempDir()
	cpu, mem, tr := filepath.Join(dir, "cpu"), filepath.Join(dir, "mem"), filepath.Join(dir, "trace")
	var stderr bytes.Buffer
	status := run(context.Background(), []string{"telesapr-cli", "regenerate", in,
		"--cpuprofile", cpu, "--memprofile", mem, "--trace", tr}, nil, &stderr, &stderr)
	if status != exitOK {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	for _, f := range []string{cpu, mem, tr} {
		if fi, err := os.Stat(f); err != nil || fi.Size() == 0 {
			t.Errorf("%s not written: %v", f, err)
		}
	}

	status = run(context.Background(), []string{"telesapr-cli", "version", "--cpuprofile",
		filepath.Join(dir, "missing", "cpu")}, nil, &stderr, &stderr)
	if status == exitOK || !strings.Contains(stderr.String(), "Cannot profile") {
		t.Errorf("a profile that can't be created gave status %d: %s", status, stderr.String())
	}
}

// TestProfileReport exports a model with --profile, and reads back how
// long it took, by phase and by group, and what it allocated; a
// command on no model gives the run's figures alone, and a report that
// can't be written fails the run.
func TestProfileReport(t *testing.T) {
	t.Parallel()
	in := filepath.Join("..", "..", "testdata", "solid", "fillet-tee.tlspr")
	dir := t.TempDir()
	report := filepath.Join(dir, "profile.json")
	var stderr bytes.Buffer
	status := run(context.Background(), []string{"telesapr-cli", "export-mesh", "--profile", report,
		"-o", filepath.Join(dir, "out.stl"), in}, nil, &stderr, &stderr)
	if status != exitOK {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	var r runReport
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != 0 || r.TotalMs <= 0 || len(r.Files) != 1 || r.Memory.TotalAllocBytes == 0 || slices.Contains(r.Command, "--profile") {
		t.Fatalf("report %s", data)
	}
	f := r.Files[0]
	solids := 0
	for _, g := range f.Groups {
		if strings.Contains(g.Group, "fillet") && g.Solids > 0 && g.SolidMs > 0 {
			solids++
		}
	}
	if f.Input != in || f.CommandMs <= 0 || solids == 0 {
		t.Errorf("the model's figures: %+v", f)
	}

	status = run(context.Background(), []string{"telesapr-cli", "version", "--profile", report}, nil, &stderr, &stderr)
	data, _ = os.ReadFile(report)
	if status != exitOK || !strings.Contains(string(data), `"total_ms"`) || strings.Contains(string(data), `"files"`) {
		t.Errorf("version: status %d, %s", status, data)
	}
	status = run(context.Background(), []string{"telesapr-cli", "version", "--profile",
		filepath.Join(dir, "missing", "p.json")}, nil, &stderr, &stderr)
	if status == exitOK {
		t.Error("a report that can't be written succeeded")
	}
}
