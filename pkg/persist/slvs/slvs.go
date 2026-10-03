package slvs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
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
	Header string
	// Empty is set for a file with nothing in it, and LoadError for one
	// with lines the loader didn't recognize; it loaded what it could.
	Empty, LoadError bool

	Blocks []Block
	// shellLines are the mesh and shell a file saves for linking
	// (Triangle, Surface, ... lines), in file order.
	shellLines      []string
	Sketch          *sketch.Sketch
	groupOrder      []core.HGroup
	paramOrder      []core.HParam
	requestOrder    []core.HRequest
	entityOrder     []core.HEntity
	constraintOrder []core.HConstraint
	groupRemap      map[core.HGroup][]RemapEntry
}

// Parse reads a .slvs file's records into a sketch, without loading
// what it links or regenerating anything. The saved entities are in
// the sketch's Entities.
func Parse(data []byte) (*Document, error) {
	doc := &Document{
		Sketch:     sketch.NewSketch(),
		groupRemap: make(map[core.HGroup][]RemapEntry),
	}
	if err := doc.parse(data); err != nil {
		return nil, err
	}
	return doc, nil
}

// LoadFile opens a .slvs file as C++ does: it loads what the file links,
// .slvs files included, and regenerates the sketch without solving.
func LoadFile(path string) (doc *Document, err error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	doc, err = Parse(data)
	if err != nil {
		return nil, err
	}
	// Like C++ SolveSpaceUI::LoadFromFile: the saved entities are
	// regenerated, not loaded, and the sketch is then upgraded and
	// regenerated without solving.
	doc.Sketch.Entities = doc.Sketch.Entities[:0]
	if !doc.reloadAllLinked(path, true) {
		return nil, ErrCancelled
	}
	doc.Sketch.ReloadAllLinked = func(*sketch.Sketch) { doc.reloadAllLinked(path, false) }
	defer func() {
		// A sketch C++ would assert on (e.g. one missing the reference
		// workplanes) is an error here.
		if rec := recover(); rec != nil {
			doc, err = nil, fmt.Errorf("%s: can't regenerate: %v", path, rec)
		}
	}()
	doc.Sketch.UpgradeLegacyData()
	return doc, nil
}

// fgetsLine reads one C `fgets(line, 1024, fh)` from data at pos: at most
// 1023 bytes, through a newline. It returns the chunk as C sees it as a
// string, which ends at the first NUL.
func fgetsLine(data []byte, pos int) (chunk string, next int, ok bool) {
	if pos >= len(data) {
		return "", pos, false
	}
	end := pos
	for end < len(data) && end-pos < 1023 {
		end++
		if data[end-1] == '\n' {
			break
		}
	}
	chunk = string(data[pos:end])
	if i := strings.IndexByte(chunk, 0); i >= 0 {
		chunk = chunk[:i]
	}
	return chunk, end, true
}

// scanLines reads the lines of a saved file as C++ LoadFromFile does: a
// line ends at a newline, a carriage return or a NUL and is at most 1023
// bytes; empty lines are skipped; a line the loader doesn't recognize is
// dropped and counts as a load error; a file with no line left is empty.
// The entries of a group's remap, which C++ reads with their own fgets,
// are kept as they are.
func scanLines(data []byte) (lines []string, empty, loadError bool) {
	empty = true
	pos := 0
	for {
		line, next, ok := fgetsLine(data, pos)
		if !ok {
			break
		}
		pos = next
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		if i := strings.IndexByte(line, '\r'); i >= 0 {
			line = line[:i]
		}
		if line == "" {
			continue
		}
		empty = false

		if e := strings.IndexByte(line, '='); e >= 0 {
			lines = append(lines, line)
			if line[:e] == "Group.remap" {
				for {
					entry, next, ok := fgetsLine(data, pos)
					if !ok {
						break
					}
					pos = next
					lines = append(lines, strings.TrimRight(entry, "\r\n"))
					var idx, ent, copyN int
					if n, _ := fmt.Sscanf(entry, "%d %x %d", &idx, &ent, &copyN); n != 3 {
						break
					}
				}
			}
			continue
		}
		switch line {
		case "AddGroup", "AddParam", "AddEntity", "AddRequest", "AddConstraint",
			"AddStyle", versionString:
		default:
			if !isShellLine(line) {
				loadError = true
				continue
			}
		}
		lines = append(lines, line)
	}
	return lines, empty, loadError
}

func (d *Document) parse(data []byte) error {
	lines, empty, loadError := scanLines(data)
	d.Empty, d.LoadError = empty, loadError
	if len(lines) == 0 {
		return nil
	}
	d.Header = lines[0]

	i := 1
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		if isShellLine(lines[i]) {
			// The mesh or shell, which a sketch regenerates; only a
			// file linking this one reads them.
			d.shellLines = append(d.shellLines, lines[i])
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
	// Files older than group colors leave C++'s zero color.
	if v, ok := fields["Group.color"]; ok {
		group.Color = core.RgbaColorFromPackedInt(parseHexUint32(v))
	}
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

	// legacy files have a spurious dependency between linked groups
	// and their parent groups, remove
	if group.Type == sketch.GroupLinked {
		group.OpA = core.HGroup{}
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
	// Like C++, fields the file leaves out take default values.
	style := core.Style{}
	core.FillDefaultStyle(&style, nil, false)
	style.H = core.HStyle{V: parseHexUint32(hStr)}
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
