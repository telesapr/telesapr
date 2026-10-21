// Parts that declare their interfaces by name, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/datums/datums.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/datums/datums.go
//
// A datum is a name given to an entity a part offers others: a point, a
// workplane, a face, or a line or circle as an axis (Edit > Datums). The
// script saves a block whose top face is the datum "top" and whose
// bottom is "bottom", as parts/block.tlspr where it runs. It links the
// block twice, as base and lid, finds their datums by name, "base.top"
// and "lid.bottom", rather than by where they are, and mates them flush:
// the lid sits on the base, whatever the block's size. It checks that,
// and saves the assembly as datums.tlspr unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// block saves a block of 40 x 30 x h with its top and bottom faces
// named.
func block(h float64) string {
	p := api.New()
	p.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	b := p.Features.Extrude(h, "block")
	p.Datums.Name("top", b.Solid().FaceAt(api.V(20, 15, h)).Entity())
	p.Datums.Name("bottom", b.Solid().FaceAt(api.V(20, 15, 0)).Entity())
	path := filepath.Join("parts", "block.tlspr")
	must(p.File.Save(path))
	return path
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))
	path := block(12)

	doc := api.Current()
	_, err := doc.Assembly.Link(path, "base")
	must(err)
	lid, err := doc.Assembly.Link(path, "lid")
	must(err)
	lid.AsLink().Move(api.V(0, 0, 50))
	fmt.Println("datums:", doc.Datums.Names())

	top, bottom := doc.Datums.Get("base.top"), doc.Datums.Get("lid.bottom")
	doc.Assembly.Flush(top, bottom)
	gap := math.Abs(top.FacePoint().Minus(bottom.FacePoint()).Dot(top.FaceNormal()))
	fmt.Printf("the lid's bottom lies %.3g mm off the base's top\n", gap)
	if gap > 1e-6 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("datums.tlspr"))
	}
}
