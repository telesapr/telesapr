// Command telesapr-cli performs an action on TeleSAPR models without
// a window: it renders a thumbnail, exports a view, wireframe, mesh or
// surfaces, or regenerates a model. It ports platform/entrycli.cpp, and
// adds converting SolveSpace models and running Go scripts on a model
// (script.go).
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ui"
	"github.com/telesapr/telesapr/pkg/version"
)

func showUsage(w io.Writer, cmd string) {
	_, _ = fmt.Fprintf(w, "Usage: %s <command> <options> <filename> [filename...]", cmd)
	_, _ = io.WriteString(w, `
    When run, performs an action specified by <command> on every <filename>.

Common options:
    -o, --output <pattern>
        For an input file <name>.tlspr, replaces the '%' symbol in <pattern>
        with <name> and uses it as output file. For example, when using
        --output %-2d.png for input files f/a.tlspr and f/b.tlspr, output files
        f/a-2d.png and f/b-2d.png will be written.
    -v, --view <direction>
        Selects the camera direction. <direction> can be one of "top", "bottom",
        "left", "right", "front", "back", or "isometric".
    -t, --chord-tol <tolerance>
        Selects the chord tolerance, used for converting exact curves to
        piecewise linear, and exact surfaces into triangle meshes.
        For export commands, the unit is mm, and the default is 1.0 mm.
        For non-export commands, the unit is %, and the default is 1.0 %.
    -a, --angle-tol <degrees>
        For export-mesh: the largest angle in degrees between the normals of
        neighbouring triangles on a curved surface; 0 for none, the default.
    -m, --max-segments <n>
        For export-mesh: the most segments a curve is divided into.
        The default is 64.
    -q, --quality <draft|standard|fine>
        For export-mesh: sets the chord tolerance, the angular tolerance and
        the segment limit together, to those of the preset. An explicit
        --chord-tol, --angle-tol or --max-segments overrides its part of it.
    -s, --strict
        For export-mesh: fails, writing no file, when the mesh has naked
        edges or self-intersections.
    -b, --bg-color <on|off>
        Whether to export the background colour in vector formats. Defaults to off.
    -g, --group <group>
        For thumbnail and the export commands but export-drawing: the model
        as it is up to and with <group>, as the window shows it with that
        group active, instead of up to the group active when it was saved.
        <group> is the group's name, or its description as the property
        browser shows it ("g004-extrude", or just "g004").

Commands:
    version
        Prints the current TeleSAPR version.
    thumbnail --output <pattern> --size <size> --view <direction>
              [--chord-tol <tolerance>] [--group <group>]
        Outputs a rendered view of the sketch, like the TeleSAPR GUI would.
        <size> is <width>x<height>, in pixels. The output is drawn in
        software, and may look slightly different from the GUI.
    export-view --output <pattern> --view <direction> [--chord-tol <tolerance>]
                [--bg-color <on|off>] [--group <group>]
        Exports a view of the sketch, in a 2d vector format.
    export-drawing --output <pattern> [--chord-tol <tolerance>]
        Exports the sketch's drawing (File > Drawing Sheet), or the default
        one, an A3 sheet of the standard views fitted, in a 2d vector format.
    export-wireframe --output <pattern> [--chord-tol <tolerance>] [--group <group>]
        Exports a wireframe of the sketch, in a 3d vector format.
    export-mesh --output <pattern> [--chord-tol <tolerance>] [--angle-tol <degrees>]
                [--max-segments <n>] [--quality <draft|standard|fine>] [--strict]
                [--group <group>]
        Exports a triangle mesh of solids in the sketch, with exact surfaces
        being triangulated first.
    export-surfaces --output <pattern> [--group <group>]
        Exports exact surfaces of solids in the sketch, if any.
    regenerate [--chord-tol <tolerance>]
        Reloads all imported files, regenerates the sketch, and saves it.
        Note that, although this is not an export command, it uses absolute
        chord tolerance, and can be used to prepare assemblies for export.
    convert [--output <pattern>] [--overwrite] <model.slvs>...
        Converts SolveSpace models to TeleSAPR models, by default each
        to <name>.tlspr beside it. Every SolveSpace model a model links is
        converted too, to a .tlspr file beside it, and the links are pointed
        at those. A converted linked model that exists already is used as it
        is, unless --overwrite is given. Nothing is written for a model with
        a missing linked file or links that form a cycle.
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
    export-view, export-drawing:%s
    export-wireframe:%s
    export-mesh:%s
    export-surfaces:%s
`, list(platform.RasterFileFilters), list(platform.VectorFileFilters),
		list(platform.Vector3dFileFilters), list(platform.MeshFileFilters),
		list(platform.SurfaceFileFilters))
	_, _ = io.WriteString(w, scriptUsage)
	_, _ = io.WriteString(w, mcpUsage)
	_, _ = io.WriteString(w, profileUsage)
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
	overwrite     bool
	group         string

	angleTol    float64 // degrees, 0 for none
	maxSegments int
	strict      bool
	quality     string

	// What was given, so that a preset fills in the rest.
	chordTolSet, angleTolSet, maxSegmentsSet bool
}

// parse reads args[2:] with the options the command accepts, which are
// named by their long forms. It is false, having said why, for an
// option the command doesn't take or a value it can't read.
func (o *options) parse(args []string, accepted []string, stderr io.Writer) bool {
	takes := func(long, short string, arg string) bool {
		if arg != long && arg != short {
			return false
		}
		return slices.Contains(accepted, long)
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
			o.chordTolSet = ok
		case hasValue && takes("--angle-tol", "-a", arg):
			n++
			_, err := fmt.Sscan(args[n], &o.angleTol)
			ok = err == nil && o.angleTol >= 0
			o.angleTolSet = ok
		case hasValue && takes("--max-segments", "-m", arg):
			n++
			_, err := fmt.Sscan(args[n], &o.maxSegments)
			ok = err == nil && o.maxSegments > 0
			o.maxSegmentsSet = ok
		case hasValue && takes("--quality", "-q", arg):
			n++
			if _, err := sketch.ParseMeshQuality(args[n]); err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
			} else {
				o.quality, ok = args[n], true
			}
		case (arg == "--strict" || arg == "-s") && takes("--strict", "", "--strict"):
			o.strict, ok = true, true
		case hasValue && takes("--bg-color", "-b", arg):
			n++
			switch args[n] {
			case "on":
				o.bgColor, ok = true, true
			case "off":
				o.bgColor, ok = false, true
			}
		case hasValue && takes("--group", "-g", arg):
			n++
			o.group = args[n]
			ok = true
		case arg == "--overwrite" && takes("--overwrite", "", "--overwrite"):
			o.overwrite, ok = true, true
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
	before, after, ok := strings.Cut(pattern, "%")
	if !ok {
		return pattern
	}
	subst := fileStem(input)
	if dir := filepath.Dir(input); dir != "." || strings.HasPrefix(input, "."+string(filepath.Separator)) {
		subst = filepath.Join(dir, subst)
	}
	return before + subst + after
}

// session is a fresh application without windows, in which one file is
// loaded and run on.
type session struct {
	ss *ui.Application
}

func newSession(stderr io.Writer) *session {
	p := headless.New()
	p.Fonts = platform.SystemFontFiles()
	s := &session{}
	p.OnMessageDialog = func(d *headless.MessageDialog) {
		kind := "Message"
		if d.Type == platform.DialogError {
			kind = "Error"
		}
		msg := d.Message
		if d.Description != "" {
			msg += " " + d.Description
		}
		_, _ = fmt.Fprintf(stderr, "%s: %s\n", kind, msg)
		// A message has only its OK button.
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	s.ss = ui.New(p)
	s.ss.Windowless = true
	s.ss.Init()
	return s
}

// fileCommand is a command that works on model files: the options it
// takes, what it checks before starting, the output pattern it uses
// when none is given ("" means one must be), and what it does to each
// file. A command that builds the solids again itself, at the export
// tolerance, has the file loaded without building them (solidsLater).
type fileCommand struct {
	accepted    []string
	output      string
	check       func(o *options, stderr io.Writer) bool
	run         func(s *session, o *options, output string) error
	solidsLater bool
}

// needView and needSize are what the view and thumbnail commands
// check.
func needView(o *options, stderr io.Writer) bool {
	if !o.viewGiven() {
		fmt.Fprintln(stderr, "View direction must be specified.")
		return false
	}
	return true
}

func needSize(o *options, stderr io.Writer) bool {
	if o.width == 0 || o.height == 0 {
		fmt.Fprintln(stderr, "Non-zero viewport size must be specified.")
		return false
	}
	return needView(o, stderr)
}

// fileCommands are the commands that work on model files, by name.
var fileCommands = map[string]fileCommand{
	"thumbnail": {
		accepted: []string{"--output", "--view", "--chord-tol", "--size", "--group"},
		check:    needSize,
		run: func(s *session, o *options, output string) error {
			return s.ss.WriteThumbnail(output, o.width, o.height, o.projRight, o.projUp)
		},
	},
	"export-view": {
		accepted:    []string{"--output", "--view", "--chord-tol", "--bg-color", "--group"},
		solidsLater: true,
		check:       needView,
		run: func(s *session, o *options, output string) error {
			s.ss.GW.SetProjection(o.projRight, o.projUp)
			s.ss.ExportChordTol = o.chordTol
			s.ss.ExportBackgroundColor = o.bgColor
			return s.ss.ExportViewOrWireframeTo(output, false)
		},
	},
	"export-wireframe": {
		accepted:    []string{"--output", "--chord-tol", "--group"},
		solidsLater: true,
		run: func(s *session, o *options, output string) error {
			s.ss.ExportChordTol = o.chordTol
			return s.ss.ExportViewOrWireframeTo(output, true)
		},
	},
	"export-drawing": {
		accepted: []string{"--output", "--chord-tol"},
		run: func(s *session, o *options, output string) error {
			s.ss.ExportChordTol = o.chordTol
			d := s.ss.SK.Drawing
			if d == nil {
				d = sketch.DefaultDrawing()
			}
			return s.ss.ExportDrawingTo(output, d)
		},
	},
	"export-mesh": {
		accepted: []string{"--output", "--chord-tol", "--group", "--angle-tol",
			"--max-segments", "--quality", "--strict"},
		solidsLater: true,
		run: func(s *session, o *options, output string) error {
			ss := s.ss
			ss.ExportChordTol = o.chordTol
			if o.quality != "" {
				// parse has checked the name.
				q, _ := sketch.ParseMeshQuality(o.quality)
				c, a, n := sketch.MeshQualitySettings(q)
				ss.ExportAngleTol, ss.ExportMaxSegments = a, n
				if !o.chordTolSet {
					ss.ExportChordTol = c
				}
			}
			if o.angleTolSet {
				ss.ExportAngleTol = o.angleTol
			}
			if o.maxSegmentsSet {
				ss.ExportMaxSegments = o.maxSegments
			}
			ss.ExportStrictMesh = o.strict
			return ss.ExportMeshTo(output)
		},
	},
	"export-surfaces": {
		accepted: []string{"--output", "--group"},
		run: func(s *session, _ *options, output string) error {
			return s.ss.ExportSurfacesTo(output)
		},
	},
	"regenerate": {
		accepted:    []string{"--chord-tol"},
		output:      "%.tlspr",
		solidsLater: true,
		run: func(s *session, o *options, output string) error {
			s.ss.ExportChordTol = o.chordTol
			s.ss.EnterExportMode()
			return s.ss.SaveToFile(output)
		},
	},
}

// runCommand performs args[1] on the files args name, timing each in
// rep when it isn't nil. It is false when the command line or a file
// was wrong.
func runCommand(args []string, stderr io.Writer, rep *runReport) bool {
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
	switch args[1] {
	case "version":
		_, _ = fmt.Fprintf(stderr, "TeleSAPR version %s\n", version.Version)
		return true
	case "convert":
		if !o.parse(args, []string{"--output", "--overwrite"}, stderr) {
			return false
		}
		return convertModels(o, stderr)
	}
	c, ok := fileCommands[args[1]]
	if !ok {
		fmt.Fprintf(stderr, "Unrecognized command '%s'.\n", args[1])
		return false
	}
	if !o.parse(args, c.accepted, stderr) {
		return false
	}
	if c.check != nil && !c.check(o, stderr) {
		return false
	}
	if o.outputPattern == "" {
		o.outputPattern = c.output
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
		f := rep.file(input)
		// Links are relative to the file that names them.
		s.ss.SaveFile = absInput
		if err := s.ss.LoadFromFile(absInput); err != nil {
			fmt.Fprintf(stderr, "Cannot load '%s': %v\n", input, err)
			return false
		}
		f.loaded(s.ss.SK)
		// The loaded sketch is solved; its solids are made at the
		// display tolerance unless the command makes them again.
		s.ss.SK.DeferShells = c.solidsLater
		s.ss.AfterNewFile()
		s.ss.SK.DeferShells = false
		if o.group != "" {
			g := s.ss.SK.GroupNamed(o.group)
			if g == nil {
				fmt.Fprintf(stderr, "Cannot find group '%s' in '%s'.\n", o.group, input)
				return false
			}
			// The model up to and with it, as the window shows it with
			// the group active.
			s.ss.SK.Active = g.H
		}
		f.regenerated()
		if err := c.run(s, o, absOutput); err != nil {
			fmt.Fprintf(stderr, "Cannot write '%s': %v\n", output, err)
			return false
		}
		f.ran(s.ss.SK)
		fmt.Fprintf(stderr, "Written '%s'.\n", output)
	}
	return true
}

// convertModels converts the SolveSpace models o names, with what they
// link. It is false when one could not be converted.
func convertModels(o *options, stderr io.Writer) bool {
	say := func(format string, args ...any) { _, _ = fmt.Fprintf(stderr, format, args...) }
	if o.outputPattern == "" {
		o.outputPattern = "%.tlspr"
	}
	if !strings.Contains(o.outputPattern, "%") && len(o.inputs) > 1 {
		say("%s\n", "Output pattern must include a % symbol when using multiple inputs!")
		return false
	}
	if len(o.inputs) == 0 {
		say("At least one input file must be specified.\n")
		return false
	}
	ok := true
	for _, input := range o.inputs {
		output := outputFor(o.outputPattern, input)
		plan, err := convert.MakePlan(input, output)
		if err != nil {
			say("Cannot convert '%s': %v\n", input, err)
			ok = false
			continue
		}
		if missing := plan.Missing(); len(missing) > 0 {
			for _, it := range missing {
				say("Cannot convert '%s': the linked file '%s' can't be read: %v\n",
					input, it.Src, it.Err)
			}
			ok = false
			continue
		}
		res, err := convert.Run(plan, convert.Options{Overwrite: o.overwrite, Quiet: true})
		for _, w := range res.Written {
			say("Written '%s'.\n", w)
		}
		for _, r := range res.Reused {
			say("Kept '%s', which exists already.\n", r)
		}
		for _, w := range res.Warnings {
			say("Warning: %s\n", w)
		}
		for path, ferr := range res.Failed {
			say("Cannot write '%s': %v\n", path, ferr)
			ok = false
		}
		if err != nil {
			say("Cannot convert '%s': %v\n", input, err)
			ok = false
		}
	}
	return ok
}

// gcPercent is the collector's target for a run of the tool, unless
// GOGC says otherwise: a command lives for one model, and collecting
// four times less often makes it 15-20% quicker for a fifth to two
// thirds more memory at its peak (doc/notes/performance-study.md).
const gcPercent = 400

func main() {
	if _, set := os.LookupEnv("GOGC"); !set {
		debug.SetGCPercent(gcPercent)
	}
	srf.SetParallelModeFromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	args := append([]string{filepath.Base(os.Args[0])}, os.Args[1:]...)
	status := run(ctx, args, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(status)
}

// run performs the command line args, args[0] being the tool's name,
// and returns the exit status: 0 for success, 1 for a command that
// failed, and the scripting commands' own statuses.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return withProfiles(args, stderr, func(args []string, rep *runReport) int {
		return runCommandLine(ctx, args, stdin, stdout, stderr, rep)
	})
}

// runCommandLine is run without the profiling options; rep, when not
// nil, is filled in for --profile.
func runCommandLine(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, rep *runReport) int {
	if len(args) == 1 {
		showUsage(stderr, args[0])
		return exitOK
	}
	switch {
	case isScriptCommand(args[1]):
		return runScript(ctx, args, stdin, stdout, stderr)
	case args[1] == "mcp":
		return runMCP(ctx, args, stdin, stdout, stderr)
	}
	if !runCommand(args, stderr, rep) {
		return 1
	}
	return exitOK
}
