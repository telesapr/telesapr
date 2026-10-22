package solver

import (
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/telesapr/telesapr/pkg/core"
)

// Reseed makes s ready to solve again, as a new System of the same
// unknowns would be: each unknown's value is the sketch's again, as sk
// gives it, and nothing is dragged. The Jacobians it wrote are kept.
func (s *System) Reseed(sk func(core.HParam) *core.Param) {
	s.sortParams()
	for _, p := range s.Param {
		p.Param = *sk(p.H)
		p.Known = false
		p.Tag = 0
	}
	s.Dragged = nil
}

// JacobiansKept is how many times s evaluated a Jacobian it had
// written before rather than writing it again.
func (s *System) JacobiansKept() int { return s.written.hits }

// maxWritten bounds the Jacobians a System keeps. A solve writes one
// for the whole system and one for each equation solved alone; finding
// the constraints to blame writes one per constraint, which are seldom
// written again.
const maxWritten = 512

// jacobianMemo keeps the Jacobians a System wrote, so that a System
// kept from one solve of a group to the next (a drag, a regeneration
// with nothing in the group changed) evaluates the symbolic Jacobian it
// wrote before rather than copying, folding and differentiating every
// equation again: that is most of a solve's allocation.
//
// A written Jacobian is a function of the equations tagged, written out
// in full, of which params are unknowns, and of the value of every
// known param an equation reads, which folding bakes in as a constant.
// The key holds all of them, so that a Jacobian kept is the one writing
// it again would give, to the bit. The unknowns' values are not in it:
// the written equations read them through the System's copies, which
// are the same params from one solve to the next.
type jacobianMemo struct {
	buf []byte
	m   map[string]writtenJacobian
	// hits counts the Jacobians taken from m.
	hits int
	// lastID is the id of the Jacobian last written.
	lastID uint64
}

// writtenJacobian is what WriteJacobian leaves in mat.A and mat.B.
type writtenJacobian struct {
	A  [][]*core.Expr
	B  []*core.Expr
	id uint64
}

// key is what WriteJacobian(tag) would write from, as it now stands:
// mat.eq and mat.param are those of tag already.
func (j *jacobianMemo) key(s *System, tag int) []byte {
	b := j.buf[:0]
	b = binary.AppendUvarint(b, uint64(len(s.mat.param)))
	for _, h := range s.mat.param {
		b = binary.AppendUvarint(b, uint64(h.V))
	}
	b = binary.AppendUvarint(b, uint64(len(s.mat.eq)))
	for _, e := range s.mat.eq {
		b = s.appendExpr(b, e.E)
	}
	b = binary.AppendVarint(b, int64(tag))
	j.buf = b
	return b
}

// Kinds of node in a key.
const (
	keyNil byte = iota
	keyNode
	keyUnknown
	keyKnown
	keySketchParam
	keyParamPtr
)

// appendExpr writes e into a key: its shape, constants and variable
// names, and each param as deepCopyWithParamsAsPointers binds it.
func (s *System) appendExpr(b []byte, e *core.Expr) []byte {
	if e == nil {
		return append(b, keyNil)
	}
	switch e.Op {
	case core.OpParam:
		if sp := s.findParam(e.Param); sp != nil {
			b = append(b, keyUnknown)
			b = binary.AppendUvarint(b, uint64(e.Param.V))
			return binary.AppendUvarint(b, boolBit(sp.Known))
		}
		p := s.SK(e.Param)
		if p == nil {
			// deepCopyWithParamsAsPointers panics; the key is never used.
			return append(b, keyNil)
		}
		if p.Known {
			b = append(b, keyKnown)
			return binary.LittleEndian.AppendUint64(b, math.Float64bits(p.Val))
		}
		// A param of the sketch that isn't known yet is read through
		// its pointer, which the kept Jacobian holds, so that no other
		// param takes its address while the key is kept.
		b = append(b, keySketchParam)
		return binary.AppendUvarint(b, uint64(uintptr(unsafe.Pointer(p))))
	case core.OpParamPtr:
		b = append(b, keyParamPtr)
		return binary.AppendUvarint(b, uint64(uintptr(unsafe.Pointer(e.ParamPtr))))
	}
	b = append(b, keyNode)
	b = binary.AppendUvarint(b, uint64(e.Op))
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(e.Value))
	b = binary.AppendUvarint(b, uint64(e.Param.V))
	b = binary.AppendUvarint(b, uint64(len(e.VarID)))
	b = append(b, e.VarID...)
	b = binary.AppendUvarint(b, uint64(len(e.Args)))
	for _, a := range e.Args {
		b = s.appendExpr(b, a)
	}
	b = s.appendExpr(b, e.Left)
	return s.appendExpr(b, e.Right)
}

func boolBit(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

// get is the Jacobian kept under key.
func (j *jacobianMemo) get(key []byte) (writtenJacobian, bool) {
	w, ok := j.m[string(key)]
	if ok {
		j.hits++
	}
	return w, ok
}

// newID names a Jacobian written: no other the System wrote has it.
func (j *jacobianMemo) newID() uint64 {
	j.lastID++
	return j.lastID
}

// put keeps a Jacobian under key; when maxWritten are kept, those kept
// before are let go.
func (j *jacobianMemo) put(key []byte, w writtenJacobian) {
	if j.m == nil || len(j.m) >= maxWritten {
		j.m = make(map[string]writtenJacobian)
	}
	j.m[string(key)] = w
}
