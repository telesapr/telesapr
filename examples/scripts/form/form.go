// A washer whose sizes are asked for when the script runs.
//
// In the application: Script > Run File shows the form below in a
// dialog, and the washer is made with what was entered. Without a
// window the values are given with --set:
//
//	telesapr-cli run --set outer=40 --set slots=6 examples/scripts/form/form.go
//
// and an AI program's run_script takes them as its form argument. A
// value not given is the one written here. It writes washer.tlspr
// where it is run, unless "save" is false.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Form is what the script asks for; each Value says what its field
// takes: a whole number, a number, true or false, or one of the
// Choices.
var Form = api.Form{
	Title: "Washer",
	Fields: []api.Field{
		{Name: "outer", Label: "Outer diameter (mm)", Value: 30.0},
		{Name: "bore", Label: "Bore (mm)", Value: 10.0},
		{Name: "thickness", Label: "Thickness (mm)", Value: 3.0},
		{Name: "slots", Label: "Slots round the bore", Value: 0},
		{Name: "finish", Label: "Finish", Value: "metal", Choices: []string{"matte", "plastic", "glossy", "metal"}},
		{Name: "save", Label: "Save washer.tlspr", Value: true},
	},
}

func main() {
	outer, bore := Form.Float("outer"), Form.Float("bore")
	if bore >= outer {
		fmt.Fprintln(os.Stderr, "the bore is not smaller than the washer")
		os.Exit(2)
	}
	doc := api.Current()
	rim := doc.Sketch.Circle(api.V(0, 0, 0), outer/2)
	doc.Constrain.Diameter(rim, outer)
	doc.Constrain.Coincident(rim.Center(), doc.Origin())
	hole := doc.Sketch.Circle(api.V(0, 0, 0), bore/2)
	doc.Constrain.Diameter(hole, bore)
	doc.Constrain.Coincident(hole.Center(), doc.Origin())
	// Slots are small holes on a circle half way out.
	slots := Form.Int("slots")
	for i := 0; i < slots; i++ { // the interpreter has no range over an int
		a := 2 * math.Pi * float64(i) / float64(slots)
		r := (outer + bore) / 4
		doc.Sketch.Circle(api.V(r*math.Cos(a), r*math.Sin(a), 0), (outer-bore)/12)
	}
	washer := doc.Features.Extrude(Form.Float("thickness"), "washer")
	washer.SetAppearance(Form.Text("finish"))
	fmt.Printf("%s: %d slots, volume %.0f mm³\n", washer, slots, washer.Analyze().Volume())
	if !Form.Bool("save") {
		return
	}
	if err := doc.File.Save("washer.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
