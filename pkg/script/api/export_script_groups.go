package api

import (
	"fmt"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// groupCall is the call that makes a group like g, and the group it is
// made from, which must be active when it is made; none for a group
// made from the one before it. "" when no call makes one, said in the
// script.
func (w *scriptWriter) groupCall(g *sketch.Group) (call string, from core.HGroup) {
	sk := w.sk
	name := fmt.Sprintf("%q", g.Name)
	axis := func() string { return w.ref(g.Predef.Origin) + ", " + w.ref(g.Predef.EntityB) }
	switch g.Type {
	case sketch.GroupDrawing3D:
		return "doc.Features.Sketch3D(" + name + ")", core.HGroup{}
	case sketch.GroupDrawingWorkplane:
		return w.workplaneCall(g, name), core.HGroup{}
	case sketch.GroupExtrude:
		return "doc.Features.Extrude(" + num(w.extrudeDepth(g)) + ", " + name + ")", g.OpA
	case sketch.GroupLathe:
		return "doc.Features.LatheAbout(" + axis() + ", " + name + ")", g.OpA
	case sketch.GroupRevolve:
		return "doc.Features.RevolveAbout(" + axis() + ", 90, " + name + ")", g.OpA
	case sketch.GroupHelix:
		if g.Thread.IsThread() {
			return w.threadCall(g)
		}
		return "doc.Features.HelixAbout(" + axis() + ", 360, 1, " + name + ")", g.OpA
	case sketch.GroupTranslate:
		return fmt.Sprintf("doc.Features.Translate(%d, api.V(1, 0, 0), %s)", int(g.ValA), name), g.OpA
	case sketch.GroupRotate:
		return fmt.Sprintf("doc.Features.Rotate(%d, %s, %s, 90, %s)", int(g.ValA),
			w.ref(g.Predef.Origin), w.ref(g.Predef.EntityB), name), g.OpA
	case sketch.GroupMirror:
		return "doc.Features.Mirror(" + w.ref(g.Predef.EntityB) + ", " + name + ")", g.OpA
	case sketch.GroupChamfer, sketch.GroupFillet:
		what := "Chamfer"
		if g.Type == sketch.GroupFillet {
			what = "Fillet"
		}
		if len(g.Edges) > 0 && len(g.SolidEdges) == 0 {
			return fmt.Sprintf("doc.Features.%s(%s, %s, %s)", what, w.refs(g.Edges), num(g.ValB), name), core.HGroup{}
		}
		edges, ok := w.solidEdges(g)
		if !ok || len(g.Edges) > 0 {
			w.notWritten("%s: the edges of %s", g.Name, strings.ToLower(what))
			return "", core.HGroup{}
		}
		return fmt.Sprintf("doc.Features.%sSolid(%s, %s, %s)", what, edges, num(g.ValB), name), core.HGroup{}
	case sketch.GroupShell:
		faces, ok := w.solidFaces(g)
		if !ok {
			w.notWritten("%s: the faces the shell takes away", g.Name)
			return "", core.HGroup{}
		}
		return fmt.Sprintf("doc.Features.ShellSolid(%s, %s, %s)", faces, num(g.ValB), name), core.HGroup{}
	case sketch.GroupLoft:
		if len(g.LoftThrough) > 0 {
			var ps []string
			for _, h := range append(append([]core.HGroup{g.OpA}, g.LoftThrough...), g.OpB) {
				ps = append(ps, w.groupVar[h])
			}
			return "doc.Features.LoftThrough([]api.Group{" + strings.Join(ps, ", ") + "}, " + name + ")", core.HGroup{}
		}
		return "doc.Features.Loft(" + w.groupVar[g.OpA] + ", " + w.groupVar[g.OpB] + ", " + name + ")", core.HGroup{}
	case sketch.GroupSweep:
		if len(g.SolidEdges) > 0 {
			edges, ok := w.solidEdges(g)
			if !ok {
				w.notWritten("%s: the solid's edges the sweep follows", g.Name)
				return "", core.HGroup{}
			}
			return fmt.Sprintf("doc.Features.SweepEdges(%s, %s, %s)", edges, w.refs(g.Edges), name), g.OpA
		}
		return "doc.Features.Sweep(" + w.refs(g.Edges) + ", " + name + ")", g.OpA
	case sketch.GroupPathPattern:
		return fmt.Sprintf("doc.Features.PatternAlongPath(%s, %d, %s)", w.refs(g.Edges), int(g.ValA), name), g.OpA
	case sketch.GroupWrap:
		faces, ok := w.solidFaceList(g, sk.Group(g.OpA))
		if !ok || len(faces) != 1 {
			w.notWritten("%s: the face it is wrapped onto", g.Name)
			return "", core.HGroup{}
		}
		what := "Emboss"
		if g.MeshCombine == sketch.CombineDifference {
			what = "Engrave"
		}
		return fmt.Sprintf("doc.Features.%s(%s, %s, %s)", what, faces[0], num(g.ValB), name), g.OpA
	case sketch.GroupLinked:
		return fmt.Sprintf("linked(doc.Assembly.Link(%q, %s))", link.Path(w.file, g.LinkFile), name), core.HGroup{}
	}
	w.notWritten("%s, a group of a kind no call makes (%s)", g.Name, kindName(g.Type))
	return "", core.HGroup{}
}

// workplaneCall makes a sketch in a workplane like g's.
func (w *scriptWriter) workplaneCall(g *sketch.Group, name string) string {
	p := &g.Predef
	switch g.Subtype {
	case sketch.GroupWorkplaneByLineSegments:
		return "doc.Features.SketchOnLines(" + w.ref(p.Origin) + ", " + w.ref(p.EntityB) + ", " +
			w.ref(p.EntityC) + ", " + name + ")"
	case sketch.GroupWorkplaneByPointNormal:
		return "doc.Features.SketchOnNormal(" + w.ref(p.Origin) + ", " + w.ref(p.EntityB) + ", " + name + ")"
	case sketch.GroupWorkplaneByFace:
		faces, ok := w.solidFaceList(g, g.PreviousGroup(w.sk))
		if ok && len(faces) == 1 {
			return "doc.Features.SketchOnFace(" + faces[0] + ", " + name + ")"
		}
		w.notWritten("%s: the face it is drawn on; drawn in its workplane as it is", g.Name)
	case sketch.GroupWorkplaneByDatum:
		if call, ok := w.datumCall(g); ok {
			return call
		}
		w.notWritten("%s: the face or edge of the solid its datum names; drawn in its workplane as it is", g.Name)
	}
	for _, wp := range []string{"XY", "YZ", "ZX"} {
		ref := referenceQ(wp)
		if p.Origin == sketch.HRequestReferenceXY.Entity(1) && p.Q == ref {
			return fmt.Sprintf("doc.Features.SketchIn(doc.Workplane(%q), %s)", wp, name)
		}
	}
	u, v := uvOf(p.Q)
	origin := w.ref(p.Origin)
	if p.Origin.V == 0 {
		origin = "doc.Origin()"
	}
	return "doc.Features.SketchAt(" + origin + ", " + vecText(snapVec(u)) + ", " + vecText(snapVec(v)) + ", " + name + ")"
}

// referenceQ is the orientation of a reference workplane.
func referenceQ(name string) geom.Quaternion {
	switch name {
	case "YZ":
		return geom.Quaternion{W: 0.5, X: 0.5, Y: 0.5, Z: 0.5}
	case "ZX":
		return geom.Quaternion{W: 0.5, X: -0.5, Y: -0.5, Z: -0.5}
	}
	return geom.Quaternion{W: 1}
}

func snapVec(v geom.Vector) geom.Vector { return geom.Vector{X: snap(v.X), Y: snap(v.Y), Z: snap(v.Z)} }

// extrudeDepth is the extrusion's depth along its workplane's normal,
// for the call; the parameters written after it set it exactly.
func (w *scriptWriter) extrudeDepth(g *sketch.Group) float64 {
	sk := w.sk
	v := geom.Vector{X: paramVal(sk, g.H.Param(0)), Y: paramVal(sk, g.H.Param(1)), Z: paramVal(sk, g.H.Param(2))}
	wp := sk.Entity(g.Predef.EntityB)
	if wp == nil || !wp.IsWorkplane() {
		return 2 * v.Magnitude()
	}
	n := wp.NormalEntity(sk).NormalN(sk)
	d := 2 * v.Dot(n)
	if math.Abs(d) < 1e-9 {
		return 10
	}
	return d
}

func paramVal(sk *sketch.Sketch, h core.HParam) float64 {
	if p := sk.Param(h); p != nil {
		return p.Val
	}
	return 0
}

// threadCall is a thread written as one call on its circle; its four
// groups are made by it.
func (w *scriptWriter) threadCall(g *sketch.Group) (string, core.HGroup) {
	sk := w.sk
	o, err := sk.ThreadOf(g.H)
	if err != nil {
		w.notWritten("%s: %v", g.Name, err)
		return "", core.HGroup{}
	}
	t := threadOptionsOf(o)
	var fields []string
	fields = append(fields, fmt.Sprintf("Designation: %q", t.Designation))
	if t.Internal {
		fields = append(fields, "Internal: true")
	}
	if t.LengthExpr != "" {
		fields = append(fields, fmt.Sprintf("LengthExpr: %q", t.LengthExpr))
	} else {
		fields = append(fields, "Length: "+num(t.Length))
	}
	if t.Through {
		fields = append(fields, "Through: true")
	}
	if t.Reverse {
		fields = append(fields, "Reverse: true")
	}
	if t.Clearance != 0 {
		fields = append(fields, "Clearance: "+num(t.Clearance))
	}
	if t.Crest != "" && t.Crest != "standard" {
		fields = append(fields, fmt.Sprintf("Crest: %q", t.Crest))
	}
	// The thread is cut on the solid of the group before its sketch.
	hs, _ := sk.ThreadGroups(g.H)
	from := core.HGroup{}
	if len(hs) > 0 {
		if first := sk.Group(hs[0]); first != nil {
			if prev := first.PreviousGroup(sk); prev != nil {
				from = prev.H
			}
		}
	}
	return "doc.Features.Thread(" + w.ref(g.Thread.Circle) + ", api.ThreadOptions{" + strings.Join(fields, ", ") + "})", from
}

// solidEdges is a fillet's, chamfer's or sweep's edges of the solid
// before it, each found by a point on it.
func (w *scriptWriter) solidEdges(g *sketch.Group) (string, bool) {
	sk := w.sk
	prev := g.PreviousGroup(sk)
	if g.Type == sketch.GroupSweep {
		prev = sk.Group(g.OpA).PreviousGroup(sk)
	}
	tp := topologyAt(sk, prev)
	if tp == nil {
		return "", false
	}
	var parts []string
	for _, n := range g.SolidEdges {
		i, err := tp.Resolve(n)
		if err != nil {
			return "", false
		}
		p, ok := edgePoint(tp, i)
		if !ok {
			return "", false
		}
		parts = append(parts, "doc.Active().Solid().EdgeAt("+vecText(p)+")")
	}
	return "[]api.SolidEdge{" + strings.Join(parts, ", ") + "}", true
}

// edgePoint is a point on edge i that no other edge passes through.
func edgePoint(tp *topo.Topology, i int) (geom.Vector, bool) {
	pts := tp.ChainPoints(tp.Chain(i))
	switch {
	case len(pts) == 0:
		return geom.Vector{}, false
	case len(pts) <= 2:
		return pts[0].Plus(pts[len(pts)-1]).ScaledBy(0.5), true
	}
	return pts[len(pts)/2], true
}

// solidFaces is a shell's faces taken away, each found by a point.
func (w *scriptWriter) solidFaces(g *sketch.Group) (string, bool) {
	faces, ok := w.solidFaceList(g, g.PreviousGroup(w.sk))
	if !ok {
		return "", false
	}
	return "[]api.SolidFace{" + strings.Join(faces, ", ") + "}", true
}

// solidFaceList is g's faces of the solid at the group before, each
// found by a point on it.
func (w *scriptWriter) solidFaceList(g, before *sketch.Group) ([]string, bool) {
	sk := w.sk
	tp := topologyAt(sk, before)
	if tp == nil {
		return nil, false
	}
	mesh := displayMesh(sk, before)
	var out []string
	for _, id := range g.SolidFaces {
		p, ok := facePoint(tp, mesh, id)
		if !ok {
			return nil, false
		}
		out = append(out, "doc.Active().Solid().FaceAt("+vecText(p)+")")
	}
	return out, true
}

// facePoint is a point inside the face with identity id: the middle of
// its largest triangle, put onto its surface, if the face found there
// is the one.
func facePoint(tp *topo.Topology, mesh *srf.SMesh, id srf.FaceID) (geom.Vector, bool) {
	faces := tp.FacesWithID(id)
	if len(faces) == 0 {
		return geom.Vector{}, false
	}
	best, area := -1, 0.0
	for i := range mesh.Triangles {
		t := &mesh.Triangles[i]
		if t.Meta.Face != id {
			continue
		}
		if a := t.B.Minus(t.A).Cross(t.C.Minus(t.A)).Magnitude(); a > area {
			best, area = i, a
		}
	}
	if best < 0 {
		return geom.Vector{}, false
	}
	t := &mesh.Triangles[best]
	p := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3)
	f, d := tp.FaceAt(p)
	if f < 0 {
		return geom.Vector{}, false
	}
	s := tp.Surface(f)
	var uv geom.Point2d
	s.ClosestPointTo(p, &uv, false)
	q := s.PointAt(uv.U, uv.V)
	if f2, d2 := tp.FaceAt(q); f2 >= 0 && d2 < d+1e-9 && tp.Sheets[tp.SheetOf(f2)].ID == id {
		return q, true
	}
	if tp.Sheets[tp.SheetOf(f)].ID == id && d < 0.005 {
		return p, true
	}
	return geom.Vector{}, false
}

// topologyAt is the topology of the solid at g, or nil.
func topologyAt(sk *sketch.Sketch, g *sketch.Group) *topo.Topology {
	if g == nil {
		return nil
	}
	src := sk.SolidAt(g.H)
	if src == nil || len(src.RunningShell.Surface) == 0 || src.IsForcedToMesh(sk) {
		return nil
	}
	tp, _ := src.Topology()
	return tp
}

// writeKindOptions sets what only groups of g's kind have.
func (w *scriptWriter) writeKindOptions(g *sketch.Group, v string) {
	switch g.Type {
	case sketch.GroupExtrude:
		if g.Subtype == sketch.GroupTwoSkewed || g.Subtype == sketch.GroupOneSkewed {
			w.line("%s.AsExtrude().SetSkewed(true)", v)
		}
		if g.Subtype == sketch.GroupTwoSided || g.Subtype == sketch.GroupTwoSkewed {
			w.line("%s.SetTwoSided(true)", v)
		}
		if g.ValC != 0 {
			w.line("%s.AsExtrude().SetDraft(%s)", v, num(g.ValC))
		}
		if g.ValB != 0 {
			w.line("%s.AsExtrude().SetDraftNeutral(%s)", v, num(g.ValB))
		}
		if len(g.DraftFaces) > 0 {
			w.line("%s.AsExtrude().SetDraftFaces(%s)", v, w.refs(g.DraftFaces))
		}
		if g.Extent != 0 && g.ExprExtent == "" {
			w.line("%s.AsExtrude().SetDepth(%s)", v, num(g.Extent))
		}
	case sketch.GroupRevolve, sketch.GroupHelix:
		if g.Thread.IsThread() {
			return
		}
		if g.Type == sketch.GroupHelix && g.ExprB == "" {
			w.line("must(%s.SetExpr(sketch.GroupPitch, %q))", v, num(g.ValB))
		}
		if g.Subtype == sketch.GroupTwoSided {
			w.line("%s.SetTwoSided(true)", v)
		}
		if g.Extent != 0 && g.ExprExtent == "" {
			w.line("%s.AsRevolve().SetAngle(%s)", v, num(g.Extent))
		}
	case sketch.GroupTranslate, sketch.GroupRotate:
		if g.Subtype == sketch.GroupTwoSided {
			w.line("%s.SetTwoSided(true)", v)
		}
		if g.SkipFirst {
			w.line("%s.AsRepeat().SetSkipFirst(true)", v)
		}
	case sketch.GroupMirror:
		if g.SkipFirst {
			w.line("%s.AsMirror().SetKeepOriginal(false)", v)
		}
		if g.MirrorSolid {
			w.line("%s.AsMirror().SetMirrorSolid(true)", v)
		}
	case sketch.GroupChamfer:
		if g.ValC != 0 {
			w.line("%s.AsChamfer().SetSecondDistance(%s)", v, num(g.ValC))
		}
	case sketch.GroupFillet:
		for _, er := range g.EdgeRadii {
			if er.Expr != "" {
				w.line("must(%s.AsFillet().SetEdgeRadiusExpr(%d, %q))", v, er.Edge, er.Expr)
			} else {
				w.line("%s.AsFillet().SetEdgeRadius(%d, %s)", v, er.Edge, num(er.Val))
			}
		}
	case sketch.GroupLoft:
		if g.ValA != 0 {
			w.line("%s.AsLoft().SetTurn(%d)", v, int(g.ValA))
		}
		if g.ValB != 0 || g.ValC != 0 {
			w.line("%s.AsLoft().SetTangency(%s, %s)", v, num(g.ValB), num(g.ValC))
		}
		if g.LoftAlongSides != [2]bool{} {
			w.line("%s.AsLoft().SetAlongSides(%v, %v)", v, g.LoftAlongSides[0], g.LoftAlongSides[1])
		}
		if len(g.Edges) > 0 {
			w.line("%s.AsLoft().SetGuides(%s)", v, w.refs(g.Edges))
		}
		for _, h := range g.LoftThrough {
			if t := g.LoftProfileTangency(h); t != 0 {
				w.line("%s.AsLoft().SetProfileTangency(%s, %s)", v, w.groupVar[h], num(t))
			}
		}
		if g.LoftClosed {
			w.line("%s.AsLoft().SetClosed(true)", v)
		}
	case sketch.GroupSweep:
		if g.ValC != 0 {
			w.line("%s.AsSweep().SetTwist(%s)", v, num(g.ValC))
		}
		if g.ValB != 0 {
			w.line("%s.AsSweep().SetCornerRadius(%s)", v, num(g.ValB))
		}
		if g.SweepUp != (geom.Vector{}) {
			w.line("%s.AsSweep().SetSweepUp(%s)", v, vecText(g.SweepUp))
		}
	case sketch.GroupLinked:
		if g.LinkAsSolid {
			w.line("%s.AsLink().SetAsSolid(true)", v)
		}
		if g.Scale != 1 && g.ExprScale == "" {
			w.line("must(%s.SetExpr(sketch.GroupScale, %q))", v, num(g.Scale))
		}
		if g.LinkConfiguration != "" {
			w.line("%s.AsLink().SetConfiguration(%q)", v, g.LinkConfiguration)
		}
		for _, lp := range g.LinkParams {
			w.line("%s.AsLink().SetParam(%q, %q)", v, lp.Name, lp.Expr)
		}
	}
}

// datumCall names the face or edge of the solid that datum workplane g
// follows, as the datum that names g's workplane, which must be called
// as g is.
func (w *scriptWriter) datumCall(g *sketch.Group) (string, bool) {
	sk := w.sk
	dt := sk.DatumOf(g.H.Entity(0))
	if dt == nil || dt.Name != g.Name {
		return "", false
	}
	if len(g.SolidEdges) > 0 {
		tp := topologyAt(sk, g.PreviousGroup(sk))
		if tp == nil {
			return "", false
		}
		i, err := tp.Resolve(g.SolidEdges[0])
		if err != nil {
			return "", false
		}
		p, ok := edgePoint(tp, i)
		if !ok {
			return "", false
		}
		return fmt.Sprintf("doc.Datums.NameEdge(%q, doc.Active().Solid().EdgeAt(%s))", dt.Name, vecText(p)), true
	}
	faces, ok := w.solidFaceList(g, g.PreviousGroup(sk))
	if !ok || len(faces) != 1 {
		return "", false
	}
	return fmt.Sprintf("doc.Datums.NameFace(%q, %s)", dt.Name, faces[0]), true
}
