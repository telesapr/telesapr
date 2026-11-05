package link

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A kept tessellation is written in a format of its own rather than
// with encoding/gob, which was a third of a megabyte of the browser's
// module for this one use. It is a cache, not a file a user keeps: a
// file that doesn't read as this version is made again, so the format
// needs no compatibility, only the magic and version to tell.
//
// Little-endian: the magic, then the mesh's flags (one byte), the
// triangles (a count, then each one's tag, face, colour and nine
// coordinates of corners and nine of normals), the vertices and the
// centres (a count, then three coordinates each), and the report's
// fields in their declared order, its maps sorted by key. Counts and
// integers are varints, strings a length and their bytes, coordinates
// float64 bits.

// stepDiskMagic starts every kept tessellation; its last byte is
// stepDiskVersion.
var stepDiskMagic = [8]byte{'T', 'L', 'S', 'P', 'R', 'M', 'S', stepDiskVersion}

// errStepDisk is a kept file that isn't a tessellation of this version.
var errStepDisk = errors.New("not a kept tessellation of this version")

// maxStepDiskCount bounds a count read back, so that a damaged file
// can't make it allocate without end.
const maxStepDiskCount = 1 << 28

type stepDiskWriter struct {
	w   *bufio.Writer
	buf [binary.MaxVarintLen64]byte
}

func (e *stepDiskWriter) int(v int) {
	n := binary.PutVarint(e.buf[:], int64(v))
	_, _ = e.w.Write(e.buf[:n])
}

func (e *stepDiskWriter) float(v float64) {
	binary.LittleEndian.PutUint64(e.buf[:8], math.Float64bits(v))
	_, _ = e.w.Write(e.buf[:8])
}

func (e *stepDiskWriter) vector(v geom.Vector) {
	e.float(v.X)
	e.float(v.Y)
	e.float(v.Z)
}

func (e *stepDiskWriter) string(s string) {
	e.int(len(s))
	_, _ = e.w.WriteString(s)
}

func (e *stepDiskWriter) vectors(vs []geom.Vector) {
	e.int(len(vs))
	for _, v := range vs {
		e.vector(v)
	}
}

// encodeStepDisk writes res to w.
func encodeStepDisk(w io.Writer, res *step.Result) error {
	e := &stepDiskWriter{w: bufio.NewWriter(w)}
	_, _ = e.w.Write(stepDiskMagic[:])
	m := &res.Mesh
	var flags byte
	for i, f := range []bool{m.FlipNormal, m.KeepInsideOtherShell, m.KeepCoplanar, m.AtLeastOneDiscarded, m.IsTransparent} {
		if f {
			flags |= 1 << i
		}
	}
	_ = e.w.WriteByte(flags)
	e.int(len(m.Triangles))
	for i := range m.Triangles {
		t := &m.Triangles[i]
		e.int(t.Tag)
		e.int(int(t.Meta.Face))
		_, _ = e.w.Write([]byte{t.Meta.Red, t.Meta.Green, t.Meta.Blue, t.Meta.Alpha})
		for _, v := range [...]geom.Vector{t.A, t.B, t.C, t.NA, t.NB, t.NC} {
			e.vector(v)
		}
	}
	e.vectors(res.Vertices)
	e.vectors(res.Centers)
	r := &res.Report
	e.string(r.Schema)
	e.string(r.Unit)
	for _, v := range [...]int{r.Shapes, r.Placements, r.Faces, r.Skipped, r.Triangles} {
		e.int(v)
	}
	e.float(r.ChordTol)
	e.int(len(r.Reasons))
	for _, k := range slices.Sorted(maps.Keys(r.Reasons)) {
		e.string(k)
		e.int(r.Reasons[k])
	}
	e.int(len(r.Details))
	for _, k := range slices.Sorted(maps.Keys(r.Details)) {
		e.string(k)
		e.string(r.Details[k])
	}
	e.int(r.Crossed)
	return e.w.Flush()
}

// stepDiskReader reads what stepDiskWriter wrote; the first error
// sticks, and every read after it gives zero.
type stepDiskReader struct {
	r   *bufio.Reader
	err error
	buf [8]byte
}

func (d *stepDiskReader) int() int {
	if d.err != nil {
		return 0
	}
	v, err := binary.ReadVarint(d.r)
	if err != nil {
		d.err = err
		return 0
	}
	return int(v)
}

func (d *stepDiskReader) count() int {
	n := d.int()
	if d.err == nil && (n < 0 || n > maxStepDiskCount) {
		d.err = fmt.Errorf("count %d: %w", n, errStepDisk)
		return 0
	}
	return n
}

func (d *stepDiskReader) bytes(b []byte) {
	if d.err != nil {
		return
	}
	if _, err := io.ReadFull(d.r, b); err != nil {
		d.err = err
	}
}

func (d *stepDiskReader) float() float64 {
	d.bytes(d.buf[:8])
	if d.err != nil {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(d.buf[:8]))
}

func (d *stepDiskReader) vector() geom.Vector {
	x := d.float()
	y := d.float()
	return geom.Vector{X: x, Y: y, Z: d.float()}
}

func (d *stepDiskReader) string() string {
	n := d.count()
	if d.err != nil || n == 0 {
		return ""
	}
	b := make([]byte, n)
	d.bytes(b)
	return string(b)
}

func (d *stepDiskReader) vectors() []geom.Vector {
	n := d.count()
	if n == 0 {
		return nil
	}
	vs := make([]geom.Vector, 0, min(n, 1<<16))
	for range n {
		if d.err != nil {
			return nil
		}
		vs = append(vs, d.vector())
	}
	return vs
}

// decodeStepDisk reads a tessellation encodeStepDisk wrote.
func decodeStepDisk(r io.Reader) (*step.Result, error) {
	d := &stepDiskReader{r: bufio.NewReader(r)}
	var magic [8]byte
	d.bytes(magic[:])
	if d.err != nil || magic != stepDiskMagic {
		return nil, errStepDisk
	}
	res := &step.Result{}
	m := &res.Mesh
	var flag [1]byte
	d.bytes(flag[:])
	m.FlipNormal, m.KeepInsideOtherShell, m.KeepCoplanar = flag[0]&1 != 0, flag[0]&2 != 0, flag[0]&4 != 0
	m.AtLeastOneDiscarded, m.IsTransparent = flag[0]&8 != 0, flag[0]&16 != 0
	if n := d.count(); n > 0 {
		m.Triangles = make([]srf.STriangle, 0, min(n, 1<<16))
		var c [4]byte
		for range n {
			if d.err != nil {
				break
			}
			t := srf.STriangle{Tag: d.int()}
			face := d.int()
			d.bytes(c[:])
			t.Meta = srf.NewSTriMeta(srf.FaceID(face), core.RGBA{Red: c[0], Green: c[1], Blue: c[2], Alpha: c[3]})
			t.A, t.B, t.C = d.vector(), d.vector(), d.vector()
			t.NA, t.NB, t.NC = d.vector(), d.vector(), d.vector()
			m.Triangles = append(m.Triangles, t)
		}
	}
	res.Vertices = d.vectors()
	res.Centers = d.vectors()
	rp := &res.Report
	rp.Schema = d.string()
	rp.Unit = d.string()
	rp.Shapes, rp.Placements, rp.Faces, rp.Skipped, rp.Triangles = d.int(), d.int(), d.int(), d.int(), d.int()
	rp.ChordTol = d.float()
	// The maps are made even when empty, as the reader of a STEP file
	// makes them, so that what is read back is what was kept.
	if n := d.count(); d.err == nil {
		rp.Reasons = make(map[string]int, min(n, 1<<10))
		for range n {
			if d.err != nil {
				break
			}
			k := d.string()
			rp.Reasons[k] = d.int()
		}
	}
	if n := d.count(); d.err == nil {
		rp.Details = make(map[string]string, min(n, 1<<10))
		for range n {
			if d.err != nil {
				break
			}
			k := d.string()
			rp.Details[k] = d.string()
		}
	}
	rp.Crossed = d.int()
	if d.err != nil {
		return nil, fmt.Errorf("reading a kept tessellation: %w", d.err)
	}
	// Nothing may follow: a longer file is not this format.
	if _, err := d.r.ReadByte(); err != io.EOF {
		return nil, errStepDisk
	}
	return res, nil
}
