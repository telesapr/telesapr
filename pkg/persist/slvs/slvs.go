package slvs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

type Block struct {
	Kind   string
	Lines  []string
	Fields map[string]string
}

type RemapEntry struct {
	Index      int
	Entity     core.HEntity
	CopyNumber int
}

type Document struct {
	Header          string
	HeaderSep       string // separator line(s) between Header and first block (preserved across round-trip)
	Blocks          []Block
	Sketch          *sketch.Sketch
	groupOrder      []core.HGroup
	paramOrder      []core.HParam
	requestOrder    []core.HRequest
	entityOrder     []core.HEntity
	constraintOrder []core.HConstraint
	groupRemap      map[core.HGroup][]RemapEntry
}

func LoadFile(path string) (*Document, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Sketch:     sketch.NewSketch(),
		groupRemap: make(map[core.HGroup][]RemapEntry),
	}

	if err := doc.parse(data); err != nil {
		return nil, err
	}
	doc.bindRequestEntities()
	return doc, nil
}

func (d *Document) Save() []byte {
	var b strings.Builder
	b.WriteString(d.Header)
	if d.HeaderSep != "" {
		b.WriteString(d.HeaderSep)
	} else {
		b.WriteString("\n\n\n")
	}

	for i := range d.Sketch.Groups {
		writeGroup(&b, &d.Sketch.Groups[i])
		b.WriteString("AddGroup\n\n")
	}

	for i := range d.Sketch.Params {
		writeParam(&b, &d.Sketch.Params[i])
		b.WriteString("AddParam\n\n")
	}

	for i := range d.Sketch.Requests {
		writeRequest(&b, &d.Sketch.Requests[i])
		b.WriteString("AddRequest\n\n")
	}

	for i := range d.Sketch.Entities {
		writeEntity(&b, &d.Sketch.Entities[i])
		b.WriteString("AddEntity\n\n")
	}

	for i := range d.Sketch.Constraints {
		writeConstraint(&b, &d.Sketch.Constraints[i])
		b.WriteString("AddConstraint\n\n")
	}

	for i := range d.Sketch.Styles {
		writeStyle(&b, &d.Sketch.Styles[i])
		b.WriteString("AddStyle\n\n")
	}

	return []byte(b.String())
}

func (d *Document) parse(data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lines := make([]string, 0, 256)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return fmt.Errorf("slvs document is empty")
	}
	d.Header = lines[0]

	i := 1
	// Capture the separator lines (blank lines) between Header
	// and the first block so Save() round-trips with identical
	// whitespace. The file has "Header\n[\n]*Group..."; the number
	// of '\n' chars after Header is (1 + blankLines).
	var sep strings.Builder
	sep.WriteString("\n") // header terminator
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		sep.WriteString("\n")
		i++
	}
	d.HeaderSep = sep.String()

	for ; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}

		block := Block{
			Lines:  make([]string, 0, 16),
			Fields: make(map[string]string),
		}

		for i < len(lines) {
			line := lines[i]
			block.Lines = append(block.Lines, line)
			trim := strings.TrimSpace(line)

			if idx := strings.Index(trim, "="); idx >= 0 {
				key := strings.TrimSpace(trim[:idx])
				val := strings.TrimSpace(trim[idx+1:])
				block.Fields[key] = val

				if strings.HasSuffix(val, "{") {
					i++
					for i < len(lines) {
						inner := lines[i]
						block.Lines = append(block.Lines, inner)
						if strings.TrimSpace(inner) == "}" {
							break
						}
						i++
					}
				}
			}

			i++
			if strings.HasPrefix(trim, "Add") {
				block.Kind = strings.TrimPrefix(trim, "Add")
				break
			}
		}

		if block.Kind == "" && len(block.Lines) > 0 {
			return fmt.Errorf("unknown block kind near line: %s", block.Lines[0])
		}

		d.Blocks = append(d.Blocks, block)
	}

	for _, block := range d.Blocks {
		switch block.Kind {
		case "Group":
			if err := d.processGroup(block); err != nil {
				return err
			}
		case "Param":
			d.processParam(block)
		case "Request":
			d.processRequest(block)
		case "Entity":
			d.processEntity(block)
		case "Constraint":
			d.processConstraint(block)
		case "Style":
			d.processStyle(block)
		}
	}

	return nil
}

func (d *Document) processGroup(block Block) error {
	fields := block.Fields

	hStr, ok := fields["Group.h.v"]
	if !ok {
		return fmt.Errorf("group missing handle")
	}
	group := sketch.Group{}
	group.H = core.HGroup{V: parseHexUint32(hStr)}

	if v, ok := fields["Group.type"]; ok {
		group.Type = sketch.GroupType(parseInt(v))
	}
	if v, ok := fields["Group.order"]; ok {
		group.Order = parseInt(v)
	}
	if v, ok := fields["Group.name"]; ok {
		group.Name = v
	}
	color, ok := fields["Group.color"]
	if !ok || color == "" {
		return fmt.Errorf("group %v missing color; legacy formats are unsupported", group.H)
	}
	group.Color = core.RgbaColorFromPackedInt(parseHexUint32(color))
	if v, ok := fields["Group.activeWorkplane.v"]; ok {
		group.ActiveWorkplane = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.opA.v"]; ok {
		group.OpA = core.HGroup{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.opB.v"]; ok {
		group.OpB = core.HGroup{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.skipFirst"]; ok {
		group.SkipFirst = parseBool(v)
	}
	if v, ok := fields["Group.visible"]; ok {
		group.Visible = parseBool(v)
	}
	if v, ok := fields["Group.suppress"]; ok {
		group.Suppress = parseBool(v)
	}
	if v, ok := fields["Group.relaxConstraints"]; ok {
		group.RelaxConstraints = parseBool(v)
	}
	if v, ok := fields["Group.allowRedundant"]; ok {
		group.AllowRedundant = parseBool(v)
	}
	if v, ok := fields["Group.suppressDofCalculation"]; ok {
		group.SuppressDofCalculation = parseBool(v)
	}
	if v, ok := fields["Group.allDimsReference"]; ok {
		group.AllDimsReference = parseBool(v)
	}
	if v, ok := fields["Group.scale"]; ok {
		group.Scale = parseFloat(v)
	} else {
		group.Scale = 1.0
	}
	if v, ok := fields["Group.meshCombine"]; ok {
		group.MeshCombine = sketch.CombineAs(parseInt(v))
	}
	if v, ok := fields["Group.forceToMesh"]; ok {
		group.ForceToMesh = parseBool(v)
	}
	if v, ok := fields["Group.linkFile"]; ok {
		group.LinkFile = v
	}
	if v, ok := fields["Group.impFileRel"]; ok && v != "" {
		group.LinkFile = v
	}
	if v, ok := fields["Group.subtype"]; ok {
		group.Subtype = sketch.GroupSubtype(parseInt(v))
	}
	if v, ok := fields["Group.valA"]; ok {
		group.ValA = parseFloat(v)
	}
	if v, ok := fields["Group.valB"]; ok {
		group.ValB = parseFloat(v)
	}
	if v, ok := fields["Group.valC"]; ok {
		group.ValC = parseFloat(v)
	}
	if v, ok := fields["Group.predef.q.w"]; ok {
		group.Predef.Q.W = parseFloat(v)
	}
	if v, ok := fields["Group.predef.q.vx"]; ok {
		group.Predef.Q.X = parseFloat(v)
	}
	if v, ok := fields["Group.predef.q.vy"]; ok {
		group.Predef.Q.Y = parseFloat(v)
	}
	if v, ok := fields["Group.predef.q.vz"]; ok {
		group.Predef.Q.Z = parseFloat(v)
	}
	if v, ok := fields["Group.predef.origin.v"]; ok {
		group.Predef.Origin = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.predef.entityB.v"]; ok {
		group.Predef.EntityB = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.predef.entityC.v"]; ok {
		group.Predef.EntityC = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Group.predef.swapUV"]; ok {
		group.Predef.SwapUV = parseBool(v)
	}
	if v, ok := fields["Group.predef.negateU"]; ok {
		group.Predef.NegateU = parseBool(v)
	}
	if v, ok := fields["Group.predef.negateV"]; ok {
		group.Predef.NegateV = parseBool(v)
	}

	group.Remap = make(core.EntityMap)
	for idx, line := range block.Lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "Group.remap") && strings.HasSuffix(trim, "{") {
			for j := idx + 1; j < len(block.Lines); j++ {
				entryLine := strings.TrimSpace(block.Lines[j])
				if entryLine == "}" {
					break
				}
				if entryLine == "" {
					continue
				}
				parts := strings.Fields(entryLine)
				if len(parts) < 3 {
					continue
				}
				index := parseInt(parts[0])
				entity := core.HEntity{V: parseHexUint32(parts[1])}
				copyNum := parseInt(parts[2])
				key := core.EntityKey{Input: entity, CopyNumber: copyNum}
				group.Remap[key] = core.EntityID{V: uint32(index)}
				d.groupRemap[group.H] = append(d.groupRemap[group.H], RemapEntry{
					Index:      index,
					Entity:     entity,
					CopyNumber: copyNum,
				})
			}
			break
		}
	}

	d.Sketch.Groups = append(d.Sketch.Groups, group)
	d.Sketch.GroupOrder = append(d.Sketch.GroupOrder, group.H)
	d.groupOrder = append(d.groupOrder, group.H)
	return nil
}

func (d *Document) processParam(block Block) {
	fields := block.Fields
	hStr, ok := fields["Param.h.v."]
	if !ok {
		return
	}
	param := core.Param{
		H: core.HParam{V: parseHexUint32(hStr)},
	}
	if v, ok := fields["Param.val"]; ok && v != "" {
		param.Val = parseFloat(v)
	}
	if v, ok := fields["Param.known"]; ok {
		param.Known = parseBool(v)
	}
	if v, ok := fields["Param.free"]; ok {
		param.Free = parseBool(v)
	}
	if !param.Known && !param.Free {
		switch {
		case param.H.V&0x40000000 != 0:
			param.Free = true
		default:
			param.Known = true
		}
	}
	d.Sketch.Params = append(d.Sketch.Params, param)
	d.paramOrder = append(d.paramOrder, param.H)
}

func (d *Document) processRequest(block Block) {
	fields := block.Fields
	hStr, ok := fields["Request.h.v"]
	if !ok {
		return
	}
	req := sketch.Request{
		H: core.HRequest{V: parseHexUint32(hStr)},
	}

	if v, ok := fields["Request.type"]; ok {
		req.Type = sketch.RequestType(parseInt(v))
	}
	if v, ok := fields["Request.extraPoints"]; ok {
		req.ExtraPoints = parseInt(v)
	}
	if v, ok := fields["Request.workplane.v"]; ok {
		req.Workplane = core.HEntity{V: parseHexUint32(v)}
	} else {
		req.Workplane = core.FreeIn3D
	}
	if v, ok := fields["Request.group.v"]; ok {
		req.Group = core.HGroup{V: parseHexUint32(v)}
	}
	if v, ok := fields["Request.style.v"]; ok {
		req.Style = core.HStyle{V: parseHexUint32(v)}
	}
	if req.Style.V == 0 {
		if v, ok := fields["Request.style"]; ok {
			req.Style = core.HStyle{V: parseHexUint32(v)}
		}
	}
	if v, ok := fields["Request.construction"]; ok {
		req.Construction = parseBool(v)
	}
	if v, ok := fields["Request.str"]; ok {
		req.Str = v
	}
	if v, ok := fields["Request.font"]; ok {
		req.Font = v
	}
	if v, ok := fields["Request.file"]; ok {
		req.File = v
	}
	if v, ok := fields["Request.aspectRatio"]; ok && v != "" {
		req.AspectRatio = parseFloat(v)
	}

	d.Sketch.Requests = append(d.Sketch.Requests, req)
	d.requestOrder = append(d.requestOrder, req.H)
}

func (d *Document) processEntity(block Block) {
	fields := block.Fields
	hStr, ok := fields["Entity.h.v"]
	if !ok {
		return
	}

	entity := core.Entity{
		EntityBase: core.EntityBase{
			H: core.HEntity{V: parseHexUint32(hStr)},
		},
	}

	if v, ok := fields["Entity.type"]; ok {
		entity.Type = core.EntityType(parseInt(v))
	}
	if v, ok := fields["Entity.group.v"]; ok {
		entity.Group = core.HGroup{V: parseHexUint32(v)}
	}
	if v, ok := fields["Entity.workplane.v"]; ok {
		entity.Workplane = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Entity.construction"]; ok {
		entity.Construction = parseBool(v)
	}
	if v, ok := fields["Entity.extraPoints"]; ok {
		entity.ExtraPoints = parseInt(v)
	}
	if v, ok := fields["Entity.normal.v"]; ok {
		entity.Normal = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Entity.distance.v"]; ok {
		entity.Distance = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Entity.style.v"]; ok {
		entity.Style = core.HStyle{V: parseHexUint32(v)}
	}
	if entity.Style.V == 0 {
		if v, ok := fields["Entity.style"]; ok {
			entity.Style = core.HStyle{V: parseHexUint32(v)}
		}
	}
	if v, ok := fields["Entity.str"]; ok {
		entity.Str = v
	}
	if v, ok := fields["Entity.font"]; ok {
		entity.Font = v
	}
	if v, ok := fields["Entity.file"]; ok {
		entity.File = v
	}

	for key, val := range fields {
		if strings.HasPrefix(key, "Entity.point[") && strings.HasSuffix(key, "].v") {
			idxStart := strings.Index(key, "[") + 1
			idxEnd := strings.Index(key, "]")
			if idxStart > 0 && idxEnd > idxStart {
				index := parseInt(key[idxStart:idxEnd])
				if index >= 0 && index < len(entity.Point) {
					entity.Point[index] = core.HEntity{V: parseHexUint32(val)}
				}
			}
		}
		if strings.HasPrefix(key, "Entity.param[") && strings.HasSuffix(key, "].v") {
			idxStart := strings.Index(key, "[") + 1
			idxEnd := strings.Index(key, "]")
			if idxStart > 0 && idxEnd > idxStart {
				index := parseInt(key[idxStart:idxEnd])
				if index >= 0 && index < len(entity.Param) {
					entity.Param[index] = core.HParam{V: parseHexUint32(val)}
				}
			}
		}
	}

	if x, ok := fields["Entity.actPoint.x"]; ok {
		entity.ActPoint.X = parseFloat(x)
	}
	if y, ok := fields["Entity.actPoint.y"]; ok {
		entity.ActPoint.Y = parseFloat(y)
	}
	if z, ok := fields["Entity.actPoint.z"]; ok {
		entity.ActPoint.Z = parseFloat(z)
	}
	if w, ok := fields["Entity.actNormal.w"]; ok {
		entity.ActNormal.W = parseFloat(w)
	}
	if vx, ok := fields["Entity.actNormal.vx"]; ok {
		entity.ActNormal.X = parseFloat(vx)
	}
	if vy, ok := fields["Entity.actNormal.vy"]; ok {
		entity.ActNormal.Y = parseFloat(vy)
	}
	if vz, ok := fields["Entity.actNormal.vz"]; ok {
		entity.ActNormal.Z = parseFloat(vz)
	}
	if dval, ok := fields["Entity.actDistance"]; ok {
		entity.ActDistance = parseFloat(dval)
	}
	if vis, ok := fields["Entity.actVisible"]; ok {
		entity.ActVisible = parseBool(vis)
	}

	d.Sketch.Entities = append(d.Sketch.Entities, entity)
	d.entityOrder = append(d.entityOrder, entity.H)
}

func (d *Document) bindRequestEntities() {
	if d.Sketch == nil {
		return
	}
	for i := range d.Sketch.Requests {
		req := &d.Sketch.Requests[i]
		req.BindEntities(d.Sketch)
	}
}

func (d *Document) processConstraint(block Block) {
	fields := block.Fields
	hStr, ok := fields["Constraint.h.v"]
	if !ok {
		return
	}
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H: core.HConstraint{V: parseHexUint32(hStr)},
		},
	}
	if v, ok := fields["Constraint.type"]; ok {
		constraint.Type = core.ConstraintType(parseInt(v))
	}
	if v, ok := fields["Constraint.group.v"]; ok {
		constraint.Group = core.HGroup{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.workplane.v"]; ok {
		constraint.Workplane = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.valA"]; ok {
		constraint.ValA = parseFloat(v)
	}
	if v, ok := fields["Constraint.valP.v"]; ok {
		constraint.ValP = core.HParam{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.ptA.v"]; ok {
		constraint.PtA = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.ptB.v"]; ok {
		constraint.PtB = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.entityA.v"]; ok {
		constraint.EntityA = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.entityB.v"]; ok {
		constraint.EntityB = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.entityC.v"]; ok {
		constraint.EntityC = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.entityD.v"]; ok {
		constraint.EntityD = core.HEntity{V: parseHexUint32(v)}
	}
	if v, ok := fields["Constraint.other"]; ok {
		constraint.Other = parseBool(v)
	}
	if v, ok := fields["Constraint.other2"]; ok {
		constraint.Other2 = parseBool(v)
	}
	if v, ok := fields["Constraint.reference"]; ok {
		constraint.Reference = parseBool(v)
	}
	if v, ok := fields["Constraint.comment"]; ok {
		constraint.Comment = v
	}
	if v, ok := fields["Constraint.disp.offset.x"]; ok {
		constraint.Disp.Offset.X = parseFloat(v)
	}
	if v, ok := fields["Constraint.disp.offset.y"]; ok {
		constraint.Disp.Offset.Y = parseFloat(v)
	}
	if v, ok := fields["Constraint.disp.offset.z"]; ok {
		constraint.Disp.Offset.Z = parseFloat(v)
	}
	if v, ok := fields["Constraint.disp.style"]; ok {
		constraint.Disp.Style = core.HStyle{V: parseHexUint32(v)}
	}

	d.Sketch.Constraints = append(d.Sketch.Constraints, constraint)
	d.constraintOrder = append(d.constraintOrder, constraint.H)
}

func (d *Document) processStyle(block Block) {
	fields := block.Fields
	hStr, ok := fields["Style.h.v"]
	if !ok {
		return
	}
	style := core.Style{
		H: core.HStyle{V: parseHexUint32(hStr)},
	}
	if v, ok := fields["Style.name"]; ok {
		style.Name = v
	}
	if v, ok := fields["Style.width"]; ok && v != "" {
		style.Width = parseFloat(v)
	}
	if v, ok := fields["Style.widthAs"]; ok {
		style.WidthAs = core.UnitsAs(parseInt(v))
	}
	if v, ok := fields["Style.textHeight"]; ok && v != "" {
		style.TextHeight = parseFloat(v)
	}
	if v, ok := fields["Style.textHeightAs"]; ok {
		style.TextHeightAs = core.UnitsAs(parseInt(v))
	}
	if v, ok := fields["Style.textAngle"]; ok && v != "" {
		style.TextAngle = parseFloat(v)
	}
	if v, ok := fields["Style.textOrigin"]; ok {
		style.TextOrigin = core.TextOrigin(parseHexUint32(v))
	}
	if v, ok := fields["Style.color"]; ok && v != "" {
		style.Color = core.RgbaColorFromPackedInt(parseHexUint32(v))
	}
	if v, ok := fields["Style.fillColor"]; ok && v != "" {
		style.FillColor = core.RgbaColorFromPackedInt(parseHexUint32(v))
	}
	if v, ok := fields["Style.filled"]; ok {
		style.Filled = parseBool(v)
	}
	if v, ok := fields["Style.visible"]; ok {
		style.Visible = parseBool(v)
	}
	if v, ok := fields["Style.exportable"]; ok {
		style.Exportable = parseBool(v)
	}
	if v, ok := fields["Style.stippleType"]; ok {
		style.StippleType = core.StipplePattern(parseInt(v))
	}
	if v, ok := fields["Style.stippleScale"]; ok && v != "" {
		style.StippleScale = parseFloat(v)
	}
	d.Sketch.Styles = append(d.Sketch.Styles, style)
}

func parseHexUint32(s string) uint32 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 16, 32)
	return uint32(v)
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 32)
	return int(v)
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseBool(s string) bool {
	s = strings.TrimSpace(s)
	return s == "1" || strings.EqualFold(s, "true")
}

const versionString = "\xB1\xB2\xB3SolveSpaceREVa"

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
	if r.Workplane != core.FreeIn3D {
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
	writeHandle(b, "Entity.workplane.v", e.Workplane.V)
	writeHandle(b, "Entity.group.v", e.Group.V)
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
