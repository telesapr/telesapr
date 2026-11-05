// Opening a damaged file from a script: what the file lacks is said,
// what used it is left out of the model but kept in the file, and a
// save writes it back until the file is mended or what was left out
// is discarded.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/damaged/damaged.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/damaged/damaged.go
//
// It writes damaged.tlspr where it is run: a sketch, and a second
// sketch placed on the origin of a workplane the file doesn't hold,
// with a point drawn in it. It opens that file, saves it, opens the
// save, mends the file's text and opens it whole, and last discards
// what was left out. Between those, it mends the model from the
// script, giving the origin the file lacks, and then the group a point
// is in when the file lacks that group.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// damaged is a model whose group 3 is placed on the origin of request
// 7's workplane, which the file lacks.
const damaged = `telesapr 2

units mm

group 0x1 sketch-3d "#references" {
  visible
}

group 0x2 sketch-in-plane "sketch-in-plane" {
  order 1
  subtype point-ortho
  workplane 0x80020000
  predef {
    q 1 0 0 0
    origin 0x10001
  }
  visible
}

group 0x3 sketch-in-plane "on-missing" {
  order 2
  subtype point-ortho
  workplane 0x80030000
  predef {
    q 1 0 0 0
    origin 0x70001
  }
  visible
}

request 0x1 workplane {
  group 0x1
}

request 0x2 workplane {
  group 0x1
}

request 0x3 workplane {
  group 0x1
}

request 0x4 datum-point {
  group 0x2
  workplane 0x80020000
}

request 0x8 datum-point {
  group 0x3
  workplane 0x80030000
}
`

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// path is the damaged file the example writes and opens again.
const path = "damaged.tlspr"

// open opens the file and says what was left out of its model.
func open() *api.Doc {
	doc, err := api.Open(path)
	must(err)
	fmt.Printf("%s: %d groups, left out %q\n", path, len(doc.Groups()), doc.Features.LeftOut())
	return doc
}

func main() {
	must(os.WriteFile(path, []byte(damaged), 0o644))

	// The sketch on the missing origin and its point are left out, and
	// the Broken References list says why.
	doc := open()
	for _, b := range doc.Features.BrokenReferences() {
		fmt.Println("  ", b.Text)
	}
	left := doc.Features.LeftOut()
	if len(left) != 2 {
		fail("left out %q, want the sketch and its point", left)
	}

	// A save keeps them in the file, so they are left out again.
	must(doc.File.Save(path))
	if again := open().Features.LeftOut(); strings.Join(again, "\n") != strings.Join(left, "\n") {
		fail("after a save, left out %q", again)
	}

	// Mended by hand, the sketch placed on the reference XY's origin,
	// the file opens whole.
	data, err := os.ReadFile(path)
	must(err)
	mended := strings.Replace(string(data), "origin 0x70001", "origin 0x10001", 1)
	must(os.WriteFile(path, []byte(mended), 0o644))
	if doc := open(); len(doc.Features.LeftOut()) != 0 || len(doc.Groups()) != 2 {
		fail("the mended file didn't open whole")
	}

	// Or mended in the model: the origin the file lacks is given, the
	// reference XY's, and the sketch and its point come back.
	must(os.WriteFile(path, []byte(damaged), 0o644))
	doc = open()
	missing := doc.Features.LeftOutMissing()
	if len(missing) != 1 || doc.Features.LeftOutMissingKind(missing[0]) != "a point" {
		fail("missing %v, want the one origin, a point", missing)
	}
	doc.Features.MendLeftOut(missing[0], doc.Entity(0x10001))
	if len(doc.Features.LeftOut()) != 0 || len(doc.Groups()) != 2 {
		fail("the mend didn't bring the sketch back")
	}
	fmt.Println("mended in the model:", len(doc.Groups()), "groups")

	// A group the file lacks is given the same way: here the point's
	// group, 9, which the file doesn't hold, is given the second sketch.
	lacksGroup := strings.Replace(strings.Replace(damaged, "origin 0x70001", "origin 0x10001", 1),
		"  group 0x3\n  workplane 0x80030000", "  group 0x9\n  workplane 0x80030000", 1)
	must(os.WriteFile(path, []byte(lacksGroup), 0o644))
	doc = open()
	groups := doc.Features.LeftOutMissingGroups()
	if len(groups) != 1 {
		fail("missing groups %v, want the point's", groups)
	}
	doc.Features.MendLeftOutGroup(groups[0], doc.Group("on-missing"))
	if len(doc.Features.LeftOut()) != 0 {
		fail("the mend didn't bring the point back")
	}
	fmt.Println("group mended in the model:", len(doc.Groups()), "groups")

	// Or what was left out is discarded, and the next save drops it.
	must(os.WriteFile(path, []byte(damaged), 0o644))
	doc = open()
	doc.Features.DiscardLeftOut()
	must(doc.File.Save(path))
	if doc := open(); len(doc.Features.LeftOut()) != 0 || len(doc.Groups()) != 1 {
		fail("the discarded items came back")
	}
}
