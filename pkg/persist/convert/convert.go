// Package convert imports SolveSpace .slvs files as .spcs documents.
//
// A .slvs file may link other .slvs files, and a .spcs document links
// only .spcs files, so an import converts the whole link tree: each
// linked a.slvs becomes a.spcs beside it, and the links are rewritten.
// Plan walks the tree and lists what would be written without writing
// anything; Run writes it. The .slvs files are never modified.
package convert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/persist/link"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// Item is one file of a plan.
type Item struct {
	// Src is the .slvs file, and Dst the .spcs file made of it.
	Src, Dst string
	// Root is set for the file the import started from.
	Root bool
	// Exists is set when Dst is there already. A linked file that
	// exists is reused or overwritten as the policy says; the root is
	// always written.
	Exists bool
	// Missing is set when Src can't be read, with the reason in Err.
	// What links it is converted with the link pointing at Dst, which
	// then counts as a missing file when it is opened.
	Missing bool
	Err     error
	// LinkedFrom is the file that first linked Src, for messages; empty
	// for the root.
	LinkedFrom string
}

// Plan is what an import would write: every file, links before the
// files that link them, and the root last.
type Plan struct {
	Items []Item
}

// CycleError is the error for files that link each other in a ring.
type CycleError struct {
	// Chain is the ring of files, the first repeated at the end.
	Chain []string
}

func (e *CycleError) Error() string {
	return "the linked files form a cycle: " + strings.Join(e.Chain, " -> ")
}

// SiblingPath is where a linked .slvs file's .spcs copy goes: beside
// it, with the extension changed.
func SiblingPath(src string) string {
	return strings.TrimSuffix(src, filepath.Ext(src)) + "." + spcs.Extension
}

// isSlvs reports whether path names a .slvs file.
func isSlvs(path string) bool { return strings.EqualFold(filepath.Ext(path), ".slvs") }

// MakePlan walks the link tree of root, which is converted to dst. It
// fails when root can't be read or the links form a cycle; a linked
// file that can't be read is in the plan as Missing.
func MakePlan(root, dst string) (*Plan, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return nil, err
	}
	if !isSlvs(rootAbs) {
		return nil, fmt.Errorf("%s is not a SolveSpace model (.slvs)", root)
	}
	if !strings.EqualFold(filepath.Ext(dstAbs), "."+spcs.Extension) {
		return nil, fmt.Errorf("%s is not a spacesolve file (.%s)", dst, spcs.Extension)
	}

	p := &Plan{}
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var stack []string
	var visit func(src, from string) error
	visit = func(src, from string) error {
		switch state[src] {
		case done:
			return nil
		case visiting:
			i := 0
			for stack[i] != src {
				i++
			}
			chain := append(append([]string(nil), stack[i:]...), src)
			return &CycleError{Chain: chain}
		}
		state[src] = visiting
		stack = append(stack, src)

		it := Item{Src: src, Dst: SiblingPath(src), LinkedFrom: from}
		if src == rootAbs {
			it.Root, it.Dst = true, dstAbs
		}
		if _, err := os.Stat(it.Dst); err == nil {
			it.Exists = true
		}
		doc, err := parse(src)
		if err != nil {
			if it.Root {
				return err
			}
			it.Missing, it.Err = true, err
		} else {
			for _, lf := range linkedSlvs(doc.Sketch, src) {
				if err := visit(lf, src); err != nil {
					return err
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[src] = done
		p.Items = append(p.Items, it)
		return nil
	}
	if err := visit(rootAbs, ""); err != nil {
		return nil, err
	}
	return p, nil
}

// parse reads a .slvs file's records.
func parse(path string) (*slvs.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := slvs.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Empty {
		return nil, fmt.Errorf("%s: the file is empty", path)
	}
	return doc, nil
}

// linkedSlvs lists the .slvs files a sketch in file links, as absolute
// paths, each once.
func linkedSlvs(sk *sketch.Sketch, file string) []string {
	var out []string
	seen := map[string]bool{}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type != sketch.GroupLinked || !isSlvs(g.LinkFile) {
			continue
		}
		abs, err := filepath.Abs(link.Path(file, g.LinkFile))
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

// Missing lists the files of the plan that can't be read.
func (p *Plan) Missing() []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Missing {
			out = append(out, it)
		}
	}
	return out
}

// Existing lists the linked files whose .spcs copy exists already, for
// which the user chooses reuse or overwrite once.
func (p *Plan) Existing() []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Exists && !it.Root && !it.Missing {
			out = append(out, it)
		}
	}
	return out
}

// Root is the plan's root item.
func (p *Plan) Root() Item { return p.Items[len(p.Items)-1] }

// Options say how Run treats what is there already.
type Options struct {
	// Overwrite rewrites a linked file's .spcs copy that exists
	// already; without it the copy is reused as it is.
	Overwrite bool
	// Units is the document unit of the files written.
	Units core.Unit
	// Quiet treats a linked file that can't be read as declined, where
	// otherwise link.LocateMissingFile asks for it.
	Quiet bool
}

// Result is what Run did.
type Result struct {
	// Written and Reused are the .spcs files written and those left as
	// they were, in plan order.
	Written, Reused []string
	// Failed holds the files that could not be converted or written,
	// by .spcs path. Files linking them open with a missing link.
	Failed map[string]error
	// Warnings are about the files written: links that were declined
	// and what regeneration removed with them.
	Warnings []string
}

// ErrCancelled is Run's error when the user cancelled at a missing
// linked file.
var ErrCancelled = link.ErrCancelled

// Run converts the files of the plan, links first. Only a failure to
// write the root, or a cancel, is an error; other failures are in the
// result.
func Run(p *Plan, o Options) (*Result, error) {
	r := &Result{Failed: map[string]error{}}
	for _, it := range p.Items {
		if it.Missing {
			continue
		}
		if !it.Root && it.Exists && !o.Overwrite {
			r.Reused = append(r.Reused, it.Dst)
			continue
		}
		data, warns, err := convertOne(it, o)
		r.Warnings = append(r.Warnings, warns...)
		if err == nil {
			err = os.WriteFile(it.Dst, data, 0o644)
		}
		if err != nil {
			if errors.Is(err, ErrCancelled) || it.Root {
				return r, err
			}
			r.Failed[it.Dst] = err
			continue
		}
		r.Written = append(r.Written, it.Dst)
	}
	return r, nil
}

// convertOne reads one .slvs file and returns it as a .spcs document to be
// saved at dst, with its .slvs links pointing at their .spcs copies
// and every link loaded from there. The caller writes it.
func convertOne(it Item, o Options) (data []byte, warnings []string, err error) {
	doc, err := parse(it.Src)
	if err != nil {
		return nil, nil, err
	}
	sk := doc.Sketch
	if doc.LoadError {
		warnings = append(warnings, it.Src+": lines that are not SolveSpace records were skipped")
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type == sketch.GroupLinked && isSlvs(g.LinkFile) {
			g.LinkFile = strings.TrimSuffix(g.LinkFile, filepath.Ext(g.LinkFile)) + "." + spcs.Extension
		}
	}
	if _, err := link.RebaseLinks(sk, it.Src, it.Dst); err != nil {
		return nil, warnings, err
	}

	// As spcs.LoadFile: the saved entities are regenerated.
	sk.Entities = sk.Entities[:0]
	if !link.ReloadAll(sk, it.Dst, link.Options{CanCancel: !o.Quiet, Quiet: o.Quiet}) {
		return nil, warnings, ErrCancelled
	}
	defer func() {
		if rec := recover(); rec != nil {
			data, err = nil, fmt.Errorf("%s: can't regenerate: %v", it.Src, rec)
		}
	}()
	var requests, constraints, groups int
	sk.OnGenerated = func(r, c, g int) { requests, constraints, groups = requests+r, constraints+c, groups+g }
	sk.UpgradeLegacyData()
	sk.GenerateAll()
	sk.OnGenerated = nil
	if requests+constraints+groups > 0 {
		warnings = append(warnings, fmt.Sprintf("%s: regeneration removed %d requests, %d constraints and %d groups that depended on what is missing",
			it.Src, requests, constraints, groups))
	}
	// The model is in millimetres; the unit only says what typed
	// lengths are in, and an imported model has none typed yet.
	sk.Units = o.Units
	return spcs.Encode(sk, spcs.Options{}), warnings, nil
}

// File converts root and what it links to dst without asking anything:
// the batch conversion the command line runs. Linked copies that exist
// are overwritten when overwrite is set, and reused otherwise.
func File(root, dst string, overwrite bool) (*Plan, *Result, error) {
	p, err := MakePlan(root, dst)
	if err != nil {
		return nil, nil, err
	}
	r, err := Run(p, Options{Overwrite: overwrite, Quiet: true})
	return p, r, err
}
