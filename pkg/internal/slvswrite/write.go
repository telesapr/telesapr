// Package slvswrite writes SolveSpace .slvs files, as C++
// SolveSpaceUI::SaveToFile does. spacesolve never writes .slvs files:
// this package exists only for the tests that compare with files C++
// saved, until ROADMAP-JUMP.md package 5.1 freezes them as Go goldens
// and deletes it. No non-test file may import it
// (TestOnlyTestsImport).
package slvswrite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

const versionString = "\xB1\xB2\xB3SolveSpaceREVa"

// Save writes the sketch as C++ SolveSpaceUI::SaveToFile does: the
// sketch is regenerated first, and each entity's numeric values and
// visibility are refreshed as for export.
func Save(sk *sketch.Sketch) []byte {
	// Make sure all the entities are regenerated up to date, since they
	// will be exported.
	sk.GenerateAll()
	return SaveRegenerated(sk)
}

// SaveRegenerated is Save for a sketch the caller has just regenerated,
// as a user interface does through its own GenerateAll.
func SaveRegenerated(sk *sketch.Sketch) []byte {
	for i := range sk.Entities {
		e := &sk.Entities[i]
		func() {
			defer func() { _ = recover() }()
			e.CalculateNumerical(sk, true)
		}()
	}

	var b strings.Builder
	b.WriteString(versionString + "\n\n\n")

	for i := range sk.Groups {
		writeGroup(&b, &sk.Groups[i])
		b.WriteString("AddGroup\n\n")
	}

	for i := range sk.Params {
		writeParam(&b, &sk.Params[i])
		b.WriteString("AddParam\n\n")
	}

	for i := range sk.Requests {
		writeRequest(&b, &sk.Requests[i])
		b.WriteString("AddRequest\n\n")
	}

	for i := range sk.Entities {
		writeEntity(&b, &sk.Entities[i])
		b.WriteString("AddEntity\n\n")
	}

	for i := range sk.Constraints {
		writeConstraint(&b, &sk.Constraints[i])
		b.WriteString("AddConstraint\n\n")
	}

	for i := range sk.Styles {
		if sk.Styles[i].H.V >= core.StyleFirstCustom {
			writeStyle(&b, &sk.Styles[i])
			b.WriteString("AddStyle\n\n")
		}
	}

	// A group will have either a mesh or a shell, but not both; but the code
	// to print either of those just does nothing if the mesh/shell is empty.
	if len(sk.GroupOrder) > 0 {
		g := sk.GetGroup(sk.GroupOrder[len(sk.GroupOrder)-1])
		writeShell(&b, &g.RunningMesh, &g.RunningShell)
	}
	return []byte(b.String())
}

func writeGroup(b *strings.Builder, g *sketch.Group) {
	writeHandle(b, "Group.h.v", g.H.V)
	writeInt(b, "Group.type", int(g.Type))
	writeInt(b, "Group.order", g.Order)
	writeString(b, "Group.name", g.Name)
	writeHandle(b, "Group.activeWorkplane.v", g.ActiveWorkplane.V)
	writeHandle(b, "Group.opA.v", g.OpA.V)
	writeHandle(b, "Group.opB.v", g.OpB.V)
	writeFloat(b, "Group.valA", g.ValA)
	writeFloat(b, "Group.valB", g.ValB)
	writeFloat(b, "Group.valC", g.ValC)
	writeColor(b, "Group.color", g.Color)
	writeInt(b, "Group.subtype", int(g.Subtype))
	writeBool(b, "Group.skipFirst", g.SkipFirst)
	writeInt(b, "Group.meshCombine", int(g.MeshCombine))
	if g.ForceToMesh {
		writeInt(b, "Group.forceToMesh", 1)
	}
	writeFloat(b, "Group.predef.q.w", g.Predef.Q.W)
	writeFloat(b, "Group.predef.q.vx", g.Predef.Q.X)
	writeFloat(b, "Group.predef.q.vy", g.Predef.Q.Y)
	writeFloat(b, "Group.predef.q.vz", g.Predef.Q.Z)
	writeHandle(b, "Group.predef.origin.v", g.Predef.Origin.V)
	writeHandle(b, "Group.predef.entityB.v", g.Predef.EntityB.V)
	writeHandle(b, "Group.predef.entityC.v", g.Predef.EntityC.V)
	writeBool(b, "Group.predef.swapUV", g.Predef.SwapUV)
	writeBool(b, "Group.predef.negateU", g.Predef.NegateU)
	writeBool(b, "Group.predef.negateV", g.Predef.NegateV)
	writeBool(b, "Group.visible", g.Visible)
	writeBool(b, "Group.suppress", g.Suppress)
	writeBool(b, "Group.relaxConstraints", g.RelaxConstraints)
	writeBool(b, "Group.allowRedundant", g.AllowRedundant)
	writeBool(b, "Group.allDimsReference", g.AllDimsReference)
	writeFloat(b, "Group.scale", g.Scale)
	writeRemap(b, "Group.remap", g.Remap)
	if g.LinkFile != "" {
		writeString(b, "Group.impFileRel", g.LinkFile)
	}
}

func writeParam(b *strings.Builder, p *core.Param) {
	writeHandle(b, "Param.h.v.", p.H.V)
	writeFloat(b, "Param.val", p.Val)
}

func writeRequest(b *strings.Builder, r *sketch.Request) {
	writeHandle(b, "Request.h.v", r.H.V)
	writeInt(b, "Request.type", int(r.Type))
	writeInt(b, "Request.extraPoints", r.ExtraPoints)
	if !core.IsFreeIn3D(r.Workplane) {
		writeHandle(b, "Request.workplane.v", r.Workplane.V)
	}
	writeHandle(b, "Request.group.v", r.Group.V)
	writeBool(b, "Request.construction", r.Construction)
	writeHandle(b, "Request.style", r.Style.V)
	writeString(b, "Request.str", r.Str)
	writeString(b, "Request.font", r.Font)
	if r.File != "" {
		writeString(b, "Request.file", r.File)
	}
	writeFloat(b, "Request.aspectRatio", r.AspectRatio)
}

func writeEntity(b *strings.Builder, e *core.Entity) {
	writeHandle(b, "Entity.h.v", e.H.V)
	writeInt(b, "Entity.type", int(e.Type))
	writeBool(b, "Entity.construction", e.Construction)
	writeHandle(b, "Entity.style", e.Style.V)
	writeString(b, "Entity.str", e.Str)
	writeString(b, "Entity.font", e.Font)
	if e.File != "" {
		writeString(b, "Entity.file", e.File)
	}
	for i := 0; i < len(e.Point); i++ {
		writeHandle(b, fmt.Sprintf("Entity.point[%d].v", i), e.Point[i].V)
	}
	// Entity.param[] is not persisted
	writeInt(b, "Entity.extraPoints", e.ExtraPoints)
	writeHandle(b, "Entity.normal.v", e.Normal.V)
	writeHandle(b, "Entity.distance.v", e.Distance.V)
	if !core.IsFreeIn3D(e.Workplane) {
		writeHandle(b, "Entity.workplane.v", e.Workplane.V)
	}
	writeFloat(b, "Entity.actPoint.x", e.ActPoint.X)
	writeFloat(b, "Entity.actPoint.y", e.ActPoint.Y)
	writeFloat(b, "Entity.actPoint.z", e.ActPoint.Z)
	writeFloat(b, "Entity.actNormal.w", e.ActNormal.W)
	writeFloat(b, "Entity.actNormal.vx", e.ActNormal.X)
	writeFloat(b, "Entity.actNormal.vy", e.ActNormal.Y)
	writeFloat(b, "Entity.actNormal.vz", e.ActNormal.Z)
	writeFloat(b, "Entity.actDistance", e.ActDistance)
	writeBool(b, "Entity.actVisible", e.ActVisible)
	// Note: Entity.param[] is not persisted to .slvs files
}

func writeConstraint(b *strings.Builder, c *core.Constraint) {
	writeHandle(b, "Constraint.h.v", c.H.V)
	writeInt(b, "Constraint.type", int(c.Type))
	writeHandle(b, "Constraint.group.v", c.Group.V)
	writeHandle(b, "Constraint.workplane.v", c.Workplane.V)
	writeFloat(b, "Constraint.valA", c.ValA)
	writeHandle(b, "Constraint.valP.v", c.ValP.V)
	writeHandle(b, "Constraint.ptA.v", c.PtA.V)
	writeHandle(b, "Constraint.ptB.v", c.PtB.V)
	writeHandle(b, "Constraint.entityA.v", c.EntityA.V)
	writeHandle(b, "Constraint.entityB.v", c.EntityB.V)
	writeHandle(b, "Constraint.entityC.v", c.EntityC.V)
	writeHandle(b, "Constraint.entityD.v", c.EntityD.V)
	writeBool(b, "Constraint.other", c.Other)
	writeBool(b, "Constraint.other2", c.Other2)
	writeBool(b, "Constraint.reference", c.Reference)
	writeString(b, "Constraint.comment", c.Comment)
	writeFloat(b, "Constraint.disp.offset.x", c.Disp.Offset.X)
	writeFloat(b, "Constraint.disp.offset.y", c.Disp.Offset.Y)
	writeFloat(b, "Constraint.disp.offset.z", c.Disp.Offset.Z)
	writeHandle(b, "Constraint.disp.style", c.Disp.Style.V)
}

func writeStyle(b *strings.Builder, s *core.Style) {
	writeHandle(b, "Style.h.v", s.H.V)
	writeString(b, "Style.name", s.Name)
	writeFloat(b, "Style.width", s.Width)
	writeInt(b, "Style.widthAs", int(s.WidthAs))
	writeFloat(b, "Style.textHeight", s.TextHeight)
	writeInt(b, "Style.textHeightAs", int(s.TextHeightAs))
	writeFloat(b, "Style.textAngle", s.TextAngle)
	writeHandle(b, "Style.textOrigin", uint32(s.TextOrigin))
	writeColor(b, "Style.color", s.Color)
	writeColor(b, "Style.fillColor", s.FillColor)
	writeBool(b, "Style.filled", s.Filled)
	writeBool(b, "Style.visible", s.Visible)
	writeBool(b, "Style.exportable", s.Exportable)
	writeInt(b, "Style.stippleType", int(s.StippleType))
	writeFloat(b, "Style.stippleScale", s.StippleScale)
}

func writeBool(b *strings.Builder, desc string, v bool) {
	b.WriteString(desc)
	b.WriteByte('=')
	if v {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteByte('\n')
}

func writeInt(b *strings.Builder, desc string, v int) {
	if v == 0 {
		return
	}
	b.WriteString(desc)
	b.WriteByte('=')
	b.WriteString(strconv.Itoa(v))
	b.WriteByte('\n')
}

func writeFloat(b *strings.Builder, desc string, v float64) {
	if v == 0 {
		return
	}
	fmt.Fprintf(b, "%s=%.20f\n", desc, v)
}

func writeString(b *strings.Builder, desc, v string) {
	if v == "" {
		return
	}
	b.WriteString(desc)
	b.WriteByte('=')
	b.WriteString(v)
	b.WriteByte('\n')
}

func writeHandle(b *strings.Builder, desc string, v uint32) {
	if v == 0 {
		return
	}
	fmt.Fprintf(b, "%s=%08x\n", desc, v)
}

func writeColor(b *strings.Builder, desc string, c core.RgbaColor) {
	fmt.Fprintf(b, "%s=%08x\n", desc, c.ToPackedInt())
}

func writeRemap(b *strings.Builder, desc string, m core.EntityMap) {
	b.WriteString(desc)
	b.WriteString("={\n")
	if len(m) > 0 {
		type entry struct {
			id  uint32
			key core.EntityKey
		}
		entries := make([]entry, 0, len(m))
		for k, v := range m {
			entries = append(entries, entry{id: v.V, key: k})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].id == entries[j].id {
				if entries[i].key.Input.V == entries[j].key.Input.V {
					return entries[i].key.CopyNumber < entries[j].key.CopyNumber
				}
				return entries[i].key.Input.V < entries[j].key.Input.V
			}
			return entries[i].id < entries[j].id
		})
		for _, e := range entries {
			b.WriteString("    ")
			b.WriteString(strconv.FormatUint(uint64(e.id), 10))
			b.WriteByte(' ')
			fmt.Fprintf(b, "%08x", e.key.Input.V)
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(e.key.CopyNumber))
			b.WriteByte('\n')
		}
	}
	b.WriteString("}\n")
}

// writeShell writes a mesh and shell as C++ SaveToFile does, for files
// that link this one.
func writeShell(b *strings.Builder, m *srf.SMesh, s *srf.SShell) {
	co := func(v geom.Vector) string { return fmt.Sprintf("%.20f %.20f %.20f", v.X, v.Y, v.Z) }
	for _, tr := range m.Triangles {
		fmt.Fprintf(b, "Triangle %08x %08x %s  %s  %s\n",
			uint32(tr.Meta.Face), tr.Meta.ToPackedInt(), co(tr.A), co(tr.B), co(tr.C))
	}

	for i := range s.Surface {
		sf := &s.Surface[i]
		fmt.Fprintf(b, "Surface %08x %08x %08x %d %d\n",
			sf.H.V, sf.Color.ToPackedInt(), uint32(sf.Face), sf.DegM, sf.DegN)
		for i := 0; i <= sf.DegM; i++ {
			for j := 0; j <= sf.DegN; j++ {
				fmt.Fprintf(b, "SCtrl %d %d %s Weight %20.20f\n", i, j, co(sf.Ctrl[i][j]), sf.Weight[i][j])
			}
		}

		for _, stb := range sf.Trim {
			backwards := 0
			if stb.Backwards {
				backwards = 1
			}
			fmt.Fprintf(b, "TrimBy %08x %d %s  %s\n", stb.Curve.V, backwards, co(stb.StartP), co(stb.FinishP))
		}

		b.WriteString("AddSurface\n")
	}
	for i := range s.Curve {
		sc := &s.Curve[i]
		exact := 0
		if sc.IsExact {
			exact = 1
		}
		fmt.Fprintf(b, "Curve %08x %d %d %08x %08x\n", sc.H.V, exact, sc.Exact.Degree, sc.SurfA.V, sc.SurfB.V)

		if sc.IsExact {
			for i := 0; i <= sc.Exact.Degree; i++ {
				fmt.Fprintf(b, "CCtrl %d %s Weight %.20f\n", i, co(sc.Exact.Ctrl[i]), sc.Exact.Weight[i])
			}
		}
		for _, scpt := range sc.Points {
			vertex := 0
			if scpt.Vertex {
				vertex = 1
			}
			fmt.Fprintf(b, "CurvePt %d %s\n", vertex, co(scpt.P))
		}

		b.WriteString("AddCurve\n")
	}
}
