package sketch

import (
	"encoding/csv"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// The bill of materials (roadmap 16.7, G2) counts the parts the model
// places by linking files: each link group places one, and a step and
// repeat, a mirror or a pattern of a link group places its copies too,
// but for a copy where the part already is, and a mirror that leaves
// the original out none more. A repeat of a repeat counts
// the copies of what it repeats. Groups left out of the model count
// none. A linked model counts as a part, and the parts it places in
// turn (Group.LinkParts, roadmap 29.3) count too, once for each place
// the model is.

// BOMItem is a line of the bill of materials: the parts from one file.
type BOMItem struct {
	// Part is the file's name without its folder and extension.
	Part string
	// File is the file as the link groups name it.
	File string
	// Count is how many the model places.
	Count int
	// Groups are the groups that place them, by name.
	Groups []string
}

// BillOfMaterials is the model's parts, by file, in the order of the
// first group that places each.
func (sk *Sketch) BillOfMaterials() []BOMItem {
	// What each group places: count by file.
	placed := map[core.HGroup]map[string]int{}
	var items []BOMItem
	index := map[string]int{}
	addAs := func(file string, n int, by string) {
		i, ok := index[file]
		if !ok {
			i = len(items)
			index[file] = i
			base := filepath.Base(file)
			items = append(items, BOMItem{Part: strings.TrimSuffix(base, filepath.Ext(base)), File: file})
		}
		items[i].Count += n
		items[i].Groups = append(items[i].Groups, by)
	}
	add := func(file string, n int, g *Group) { addAs(file, n, g.DescriptionString()) }
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		if g == nil || g.Suppress || g.SuppressAll || sk.SuppressedReason(hg) != "" {
			continue
		}
		var src map[string]int
		copies := 0 // copies placed where the source isn't
		all := 0    // copies made, the one at the source's place too
		switch g.Type {
		case GroupLinked:
			if g.LinkFile == "" {
				continue
			}
			mine := map[string]int{g.LinkFile: 1}
			add(g.LinkFile, 1, g)
			// What the linked model places in turn, named by the group
			// here and the groups there.
			for _, it := range g.LinkParts {
				mine[it.File] += it.Count
				addAs(it.File, it.Count, g.DescriptionString()+" > "+strings.Join(it.Groups, ", "))
			}
			placed[hg] = mine
			continue
		case GroupTranslate, GroupRotate:
			src = placed[g.OpA]
			n, a0 := int(g.ValA), 0
			if g.Subtype == GroupOneSided && g.SkipFirst {
				a0, n = 1, n+1
			}
			for a := a0; a < n; a++ {
				all++
				if a*2-stepOffset(g.Subtype, n) != 0 {
					copies++
				}
			}
		case GroupMirror:
			src = placed[g.OpA]
			if g.MirrorSolid {
				// The whole solid so far: every part placed before.
				src = map[string]int{}
				for _, it := range items {
					src[it.File] += it.Count
				}
			}
			// With the original kept, the reflection is one more;
			// without, it takes the original's place.
			all, copies = 2, 1
			if g.SkipFirst {
				all, copies = 1, 0
			}
		case GroupPathPattern:
			src = placed[g.OpA]
			all = int(math.Round(g.ValA))
			copies = all - 1
		default:
			continue
		}
		if len(src) == 0 {
			continue
		}
		mine := map[string]int{}
		files := make([]string, 0, len(src))
		for f := range src {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			mine[f] = src[f] * all
			if n := src[f] * copies; n > 0 {
				add(f, n, g)
			}
		}
		placed[hg] = mine
	}
	return items
}

// WriteBOMCSV writes the bill of materials as CSV: a header, then a line
// for each part with its count, its file and the groups that place it.
func WriteBOMCSV(w io.Writer, items []BOMItem) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"part", "count", "file", "groups"}); err != nil {
		return err
	}
	for _, it := range items {
		if err := cw.Write([]string{it.Part, strconv.Itoa(it.Count), it.File, strings.Join(it.Groups, "; ")}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// BillOfMaterialsAsRead is BillOfMaterials for a sketch read from a
// file and not regenerated, as a linked model's is when the bill of
// the model linking it counts what it places: which groups are left
// out is worked out first.
func (sk *Sketch) BillOfMaterialsAsRead() []BOMItem {
	sk.markSuppressed()
	return sk.BillOfMaterials()
}
