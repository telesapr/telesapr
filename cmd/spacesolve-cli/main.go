// Command spacesolve-cli performs an action on SolveSpace files without
// a window: it renders a thumbnail, exports a view, wireframe, mesh or
// surfaces, or regenerates a sketch. It ports platform/entrycli.cpp.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/ui"
)

const version = "3.2"

func showUsage(w io.Writer, cmd string) {
	fmt.Fprintf(w, "Usage: %s <command> <options> <filename> [filename...]", cmd)
	io.WriteString(w, `
    When run, performs an action specified by <command> on every <filename>.

Common options:
    -o, --output <pattern>
        For an input file <name>.slvs, replaces the '%' symbol in <pattern>
        with <name> and uses it as output file. For example, when using
        --output %-2d.png for input files f/a.slvs and f/b.slvs, output files
        f/a-2d.png and f/b-2d.png will be written.
    -v, --view <direction>
        Selects the camera direction. <direction> can be one of "top", "bottom",
        "left", "right", "front", "back", or "isometric".
    -t, --chord-tol <tolerance>
        Selects the chord tolerance, used for converting exact curves to
        piecewise linear, and exact surfaces into triangle meshes.
        For export commands, the unit is mm, and the default is 1.0 mm.
        For non-export commands, the unit is %, and the default is 1.0 %.
    -b, --bg-color <on|off>
        Whether to export the background colour in vector formats. Defaults to off.

Commands:
    version
        Prints the current solvespace version.
    thumbnail --output <pattern> --size <size> --view <direction>
              [--chord-tol <tolerance>]
        Outputs a rendered view of the sketch, like the SolveSpace GUI would.
        <size> is <width>x<height>, in pixels. The output is drawn in
        software, and may look slightly different from the GUI.
    export-view --output <pattern> --view <direction> [--chord-tol <tolerance>]
                [--bg-color <on|off>]
        Exports a view of the sketch, in a 2d vector format.
    export-wireframe --output <pattern> [--chord-tol <tolerance>]
        Exports a wireframe of the sketch, in a 3d vector format.
    export-mesh --output <pattern> [--chord-tol <tolerance>]
        Exports a triangle mesh of solids in the sketch, with exact surfaces
        being triangulated first.
    export-surfaces --output <pattern>
        Exports exact surfaces of solids in the sketch, if any.
    regenerate [--chord-tol <tolerance>]
        Reloads all imported files, regenerates the sketch, and saves it.
        Note that, although this is not an export command, it uses absolute
        chord tolerance, and can be used to prepare assemblies for export.
`)

	list := func(filters []platform.FileFilter) string {
		var b strings.Builder
		for _, f := range filters {
			fmt.Fprintf(&b, "\n        %s (%s)", f.Name, strings.Join(f.Extensions, ", "))
		}
		return b.String()
	}
	fmt.Fprintf(w, `
File formats:
    thumbnail:%s
    export-view:%s
    export-wireframe:%s
    export-mesh:%s
    export-surfaces:%s
`, list(platform.RasterFileFilters), list(platform.VectorFileFilters),
		list(platform.Vector3dFileFilters), list(platform.MeshFileFilters),
		list(platform.SurfaceFileFilters))
}

// views are the camera directions --view names, as projRight and
// projUp.
var views = map[string][2]geom.Vector{
	"top":       {{X: 1}, {Z: -1}},
	"bottom":    {{X: 1}, {Z: 1}},
	"left":      {{Z: 1}, {Y: 1}},
	"right":     {{Z: -1}, {Y: 1}},
	"front":     {{X: 1}, {Y: 1}},
	"back":      {{X: -1}, {Y: 1}},
	"isometric": {{X: 0.707, Z: -0.707}, {X: -0.408, Y: 0.816, Z: -0.408}},
}

// options are what the command line asked for.
type options struct {
	inputs        []string
	outputPattern string
	projRight     geom.Vector
	projUp        geom.Vector
	chordTol      float64
	bgColor       bool
	width, height uint
}

// parse reads args[2:] with the options the command accepts, which are
// named by their long forms. It is false, having said why, for an
// option the command doesn't take or a value it can't read.
func (o *options) parse(args []string, accepted []string, stderr io.Writer) bool {
	takes := func(long, short string, arg string) bool {
		if arg != long && arg != short {
			return false
		}
		for _, a := range accepted {
			if a == long {
				return true
			}
		}
		return false
	}
	for n := 2; n < len(args); n++ {
		arg := args[n]
		hasValue := n+1 < len(args)
		ok := false
		switch {
		case arg != "" && arg[0] != '-':
			o.inputs = append(o.inputs, arg)
			ok = true
		case hasValue && takes("--output", "-o", arg):
			n++
			o.outputPattern = args[n]
			ok = true
		case hasValue && takes("--view", "-v", arg):
			n++
			if v, found := views[args[n]]; found {
				o.projRight, o.projUp = v[0], v[1]
			} else {
				fmt.Fprintf(stderr, "Unrecognized view direction '%s'\n", args[n])
			}
			ok = true
		case hasValue && takes("--chord-tol", "-t", arg):
			n++
			_, err := fmt.Sscan(args[n], &o.chordTol)
			ok = err == nil
		case hasValue && takes("--bg-color", "-b", arg):
			n++
			switch args[n] {
			case "on":
				o.bgColor, ok = true, true
			case "off":
				o.bgColor, ok = false, true
			}
		case hasValue && takes("--size", "", arg):
			n++
			var w, h uint
			if c, _ := fmt.Sscanf(args[n], "%dx%d", &w, &h); c == 2 {
				o.width, o.height = w, h
				ok = true
			}
		}
		if !ok {
			fmt.Fprintf(stderr, "Unrecognized option '%s'.\n", arg)
			return false
		}
	}
	return true
}

func (o *options) viewGiven() bool {
	return o.projUp.Magnitude() != 0 && o.projRight.Magnitude() != 0
}

// fileStem is name without its folder and extension.
func fileStem(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// outputFor is the file pattern names for the input file: its first %
// is replaced by the input's folder and stem.
func outputFor(pattern, input string) string {
	i := strings.IndexByte(pattern, '%')
	if i < 0 {
		return pattern
	}
	subst := fileStem(input)
	if dir := filepath.Dir(input); dir != "." || strings.HasPrefix(input, "."+string(filepath.Separator)) {
		subst = filepath.Join(dir, subst)
	}
	return pattern[:i] + subst + pattern[i+1:]
}

// session is a fresh application without windows, in which one file is
// loaded and run on.
type session struct {
	ss      *ui.SolveSpaceUI
	errored bool
}

func newSession(stderr io.Writer) *session {
	p := headless.New()
	p.FontFiles = platform.SystemFontFiles()
	s := &session{}
	p.OnMessageDialog = func(d *headless.MessageDialog) {
		kind := "Message"
		if d.Type == platform.DialogError {
			kind = "Error"
			s.errored = true
		}
		fmt.Fprintf(stderr, "%s: %s\n", kind, d.Message)
		// A message has only its OK button.
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	s.ss = ui.New(p)
	s.ss.Windowless = true
	s.ss.Init()
	return s
}

// runCommand performs args[1] on the files args name. It is false when
// the command line or a file was wrong.
func runCommand(args []string, stderr io.Writer) bool {
	if len(args) < 2 {
		return false
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			showUsage(stderr, args[0])
			return true
		}
	}

	o := &options{chordTol: 1.0}
	var accepted []string
	var runner func(s *session, output string) error

	switch args[1] {
	case "version":
		fmt.Fprintf(stderr, "SolveSpace version %s \n\n", version)
		return true
	case "thumbnail":
		accepted = []string{"--output", "--view", "--chord-tol", "--size"}
	case "export-view":
		accepted = []string{"--output", "--view", "--chord-tol", "--bg-color"}
	case "export-wireframe":
		accepted = []string{"--output", "--chord-tol"}
	case "export-mesh":
		accepted = []string{"--output", "--chord-tol"}
	case "export-surfaces":
		accepted = []string{"--output"}
	case "regenerate":
		accepted = []string{"--chord-tol"}
	default:
		fmt.Fprintf(stderr, "Unrecognized command '%s'.\n", args[1])
		return false
	}
	if !o.parse(args, accepted, stderr) {
		return false
	}

	switch args[1] {
	case "thumbnail":
		if o.width == 0 || o.height == 0 {
			fmt.Fprintln(stderr, "Non-zero viewport size must be specified.")
			return false
		}
		if !o.viewGiven() {
			fmt.Fprintln(stderr, "View direction must be specified.")
			return false
		}
		runner = func(s *session, output string) error {
			return s.ss.WriteThumbnail(output, o.width, o.height, o.projRight, o.projUp)
		}
	case "export-view":
		if !o.viewGiven() {
			fmt.Fprintln(stderr, "View direction must be specified.")
			return false
		}
		runner = func(s *session, output string) error {
			s.ss.GW.SetProjection(o.projRight, o.projUp)
			s.ss.ExportChordTol = o.chordTol
			s.ss.ExportBackgroundColor = o.bgColor
			s.ss.ExportViewOrWireframeTo(output, false)
			return nil
		}
	case "export-wireframe":
		runner = func(s *session, output string) error {
			s.ss.ExportChordTol = o.chordTol
			s.ss.ExportViewOrWireframeTo(output, true)
			return nil
		}
	case "export-mesh":
		runner = func(s *session, output string) error {
			s.ss.ExportChordTol = o.chordTol
			s.ss.ExportMeshTo(output)
			return nil
		}
	case "export-surfaces":
		runner = func(s *session, output string) error {
			s.ss.ExportSurfacesTo(output)
			return nil
		}
	case "regenerate":
		o.outputPattern = "%.slvs"
		runner = func(s *session, output string) error {
			s.ss.ExportChordTol = o.chordTol
			s.ss.ExportMode = true
			if !s.ss.SaveToFile(output) {
				return fmt.Errorf("cannot save")
			}
			return nil
		}
	}

	if o.outputPattern == "" {
		fmt.Fprintln(stderr, "An output pattern must be specified.")
		return false
	} else if !strings.Contains(o.outputPattern, "%") && len(o.inputs) > 1 {
		fmt.Fprintln(stderr, "Output pattern must include a % symbol when using multiple inputs!")
		return false
	}
	if len(o.inputs) == 0 {
		fmt.Fprintln(stderr, "At least one input file must be specified.")
		return false
	}

	for _, input := range o.inputs {
		absInput, err := filepath.Abs(input)
		if err != nil {
			absInput = input
		}
		output := outputFor(o.outputPattern, input)
		absOutput, err := filepath.Abs(output)
		if err != nil {
			absOutput = output
		}

		s := newSession(stderr)
		// Links are relative to the file that names them.
		s.ss.SaveFile = absInput
		if !s.ss.LoadFromFile(absInput) || s.errored {
			fmt.Fprintf(stderr, "Cannot load '%s'!\n", input)
			return false
		}
		s.ss.AfterNewFile()
		err = runner(s, absOutput)
		if err != nil || s.errored {
			if err != nil {
				fmt.Fprintf(stderr, "Cannot write '%s': %v\n", output, err)
			}
			return false
		}
		fmt.Fprintf(stderr, "Written '%s'.\n", output)
	}
	return true
}

func main() {
	srf.SetParallelModeFromEnv()
	args := append([]string{filepath.Base(os.Args[0])}, os.Args[1:]...)
	if len(args) == 1 {
		showUsage(os.Stderr, args[0])
		return
	}
	if !runCommand(args, os.Stderr) {
		os.Exit(1)
	}
}
