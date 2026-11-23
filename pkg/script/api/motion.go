package api

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// MotionFrame is a step of a motion study: the parameter's value, the
// file its frame went to, if any, whether every group solved, and the
// volume of the solid up to the active group.
type MotionFrame struct {
	Step int
	// Value is the first drive's value at the step, Values each drive's,
	// in the order the study was given them.
	Value  float64
	Values []float64
	File   string
	Solved bool
	Volume float64
}

// Drive is a parameter a motion study drives, and the range it drives
// it through, from From at the first step to To at the last.
type Drive struct {
	Param    string
	From, To float64
}

// isAnimation reports whether a motion study's frames name one animated
// glTF file, a .glb or a .gltf with no verb for the step's number.
func isAnimation(frames string) bool {
	ext := strings.ToLower(filepath.Ext(frames))
	return (ext == ".glb" || ext == ".gltf") && !strings.Contains(frames, "%")
}

// MotionStudyOf drives several parameters together, each through its
// own range, over the same steps: steps equal steps, steps+1 frames
// with both ends, regenerating the model at each, as MotionStudy does
// for one; MotionFrame.Values holds each drive's value at the step.
// frames is as MotionStudy takes it, a file name with a verb for the
// step's number, or one .glb or .gltf file with none: then the study is
// written as one glTF animation, a node per linked part and, for each
// part that moves in a mechanism, a translation and a rotation channel
// with a keyframe a step, a tenth of a second apart, so that a glTF
// viewer plays it. The animation needs a solid that is an assembly of
// linked parts, each read as a solid. Each parameter gets its
// expression back after, whatever happened. A step whose groups don't
// all solve is kept, marked so.
func (m AssemblyMenu) MotionStudyOf(drives []Drive, steps int, frames string) (out []MotionFrame, err error) {
	d := m.d
	if len(drives) == 0 {
		return nil, errors.New("a motion study drives at least one parameter")
	}
	exprs := make([]string, len(drives))
	for i, dr := range drives {
		p, ok := d.Params.Get(dr.Param)
		if !ok {
			return nil, fmt.Errorf("no parameter %q", dr.Param)
		}
		if slices.ContainsFunc(drives[:i], func(o Drive) bool { return o.Param == dr.Param }) {
			return nil, fmt.Errorf("parameter %q is driven twice", dr.Param)
		}
		exprs[i] = p.Expr
	}
	if steps < 1 || steps > 10000 {
		return nil, fmt.Errorf("a motion study takes from 1 to 10000 steps, not %d", steps)
	}
	animation := isAnimation(frames)
	if frames != "" && !animation && !strings.Contains(frames, "%") {
		return nil, errors.New("the frames' file name needs a verb for the step's number, as f%03d.png, or is one .glb or .gltf for an animation")
	}
	var track *sketch.MotionTrack
	if animation {
		if err := d.checkFile(frames, true); err != nil {
			return nil, err
		}
		var terr error
		d.read(func(sk *sketch.Sketch) { track, terr = sk.NewMotionTrack() })
		if terr != nil {
			return nil, terr
		}
	}
	defer func() {
		for i, dr := range drives {
			if rerr := d.Params.Set(dr.Param, exprs[i]); rerr != nil && err == nil {
				err = rerr
			}
		}
		d.Edit.Regen()
	}()
	for i := 0; i <= steps; i++ {
		f := MotionFrame{Step: i, Solved: true}
		for _, dr := range drives {
			v := dr.From + (dr.To-dr.From)*float64(i)/float64(steps)
			if err := d.Params.Set(dr.Param, strconv.FormatFloat(v, 'g', -1, 64)); err != nil {
				return out, fmt.Errorf("step %d, %s = %g: %w", i, dr.Param, v, err)
			}
			f.Values = append(f.Values, v)
		}
		f.Value = f.Values[0]
		d.Edit.Regen()
		for _, g := range d.Groups() {
			f.Solved = f.Solved && g.Solved()
		}
		f.Volume = d.Active().Analyze().Check().Volume
		switch {
		case animation:
			d.read(func(sk *sketch.Sketch) { track.Record(sk) })
		case frames != "":
			f.File = fmt.Sprintf(frames, i)
			var ferr error
			if strings.EqualFold(filepath.Ext(f.File), ".png") {
				ferr = d.File.Render(f.File, 800, 600, "window")
			} else {
				ferr = d.File.Export(f.File)
			}
			if ferr != nil {
				return out, fmt.Errorf("frame %d: %w", i, ferr)
			}
		}
		out = append(out, f)
	}
	if animation {
		o := d.h.ExportOptions()
		es := sketch.ExportSettings{ChordTol: o.ChordTol, MaxSegments: o.MaxSegments, Scale: o.Scale, AngleTol: o.AngleTol}
		var werr error
		d.read(func(sk *sketch.Sketch) { werr = sk.ExportMotionTo(frames, track, es) })
		if werr != nil {
			return out, fmt.Errorf("writing the animation: %w", werr)
		}
		for i := range out {
			out[i].File = frames
		}
	}
	return out, nil
}
