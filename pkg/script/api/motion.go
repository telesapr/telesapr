package api

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// MotionFrame is a step of a motion study: the parameter's value, the
// file its frame went to, if any, whether every group solved, and the
// volume of the solid up to the active group.
type MotionFrame struct {
	Step   int
	Value  float64
	File   string
	Solved bool
	Volume float64
}

// MotionStudy drives the parameter param from from to to in steps
// equal steps, steps+1 frames with both ends, regenerating the model at
// each (roadmap 16.3). With frames not "", each frame is saved to the
// file frames names with the step's number in place of its verb, as
// "frames/f%03d.png": a picture of the view the window shows for .png,
// in the application, and otherwise what Export writes for the
// extension. The parameter gets its expression back after, whatever
// happened. A step whose groups don't all solve is kept, marked so.
func (d *Doc) MotionStudy(param string, from, to float64, steps int, frames string) (out []MotionFrame, err error) {
	p, ok := d.Parameter(param)
	if !ok {
		return nil, fmt.Errorf("no parameter %q", param)
	}
	if steps < 1 || steps > 10000 {
		return nil, fmt.Errorf("a motion study takes from 1 to 10000 steps, not %d", steps)
	}
	if frames != "" && !strings.Contains(frames, "%") {
		return nil, errors.New("the frames' file name needs a verb for the step's number, as f%03d.png")
	}
	defer func() {
		if rerr := d.SetParameter(param, p.Expr); rerr != nil && err == nil {
			err = rerr
		}
		d.Regen()
	}()
	for i := 0; i <= steps; i++ {
		v := from + (to-from)*float64(i)/float64(steps)
		if err := d.SetParameter(param, strconv.FormatFloat(v, 'g', -1, 64)); err != nil {
			return out, fmt.Errorf("step %d, %s = %g: %w", i, param, v, err)
		}
		d.Regen()
		f := MotionFrame{Step: i, Value: v, Solved: true}
		for _, g := range d.Groups() {
			f.Solved = f.Solved && g.Solved()
		}
		f.Volume = d.Active().Check().Volume
		if frames != "" {
			f.File = fmt.Sprintf(frames, i)
			var ferr error
			if strings.EqualFold(filepath.Ext(f.File), ".png") {
				ferr = d.Render(f.File, 800, 600, "window")
			} else {
				ferr = d.Export(f.File)
			}
			if ferr != nil {
				return out, fmt.Errorf("frame %d: %w", i, ferr)
			}
		}
		out = append(out, f)
	}
	return out, nil
}
