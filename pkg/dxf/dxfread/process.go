package dxfread

// The sections of a DXF file, as C++ dxfRW::read and its process*
// functions walk them.

import "bytes"

// Interface receives what the file holds (the parts of C++
// DRW_Interface SolveSpace implements; the rest are read and dropped).
type Interface interface {
	AddLayer(l *Layer)
	AddBlock(b *Block)
	EndBlock()
	AddPoint(p *Point)
	AddLine(l *Line)
	AddArc(a *Arc)
	AddCircle(c *Circle)
	AddLWPolyline(p *LWPolyline)
	AddPolyline(p *Polyline)
	AddSpline(s *Spline)
	AddInsert(i *Insert)
	AddText(t *Text)
	AddMText(m *MText)
	// AddDimension gets every kind of dimension, with its Kind set (C++
	// addDimLinear, addDimAlign and so on).
	AddDimension(d *Dimension)
}

// binarySentinel starts a binary DXF file.
var binarySentinel = []byte("AutoCAD Binary DXF\r\n\x1a\x00")

// dxfRW is the reading state (C++ dxfRW).
type dxfRW struct {
	r          *reader
	iface      Interface
	applyExt   bool
	nextentity string

	// The header's current variable (C++ DRW_Header::curr and name).
	headerCurr bool
	headerName string
}

// Read reads a DXF file, telling iface what it holds, and reports
// whether it read it all (C++ dxfRW::read). applyExt applies the
// entities' extrusions, as SolveSpace asks.
func Read(data []byte, iface Interface, applyExt bool) bool {
	d := &dxfRW{iface: iface, applyExt: applyExt}
	r := &reader{data: data, good: true, decoder: newCodec()}
	if len(data) < len(binarySentinel) {
		// C++ reads the sentinel's 22 bytes first; a shorter file leaves
		// the stream failed.
		return false
	}
	// strcmp: up to the first NUL of the file's first 22 bytes.
	head := data[:len(binarySentinel)]
	if i := bytes.IndexByte(head, 0); i >= 0 {
		head = head[:i+1]
	}
	if bytes.Equal(head, binarySentinel) {
		r.binary = true
		r.pos = len(binarySentinel)
	}
	d.r = r
	return d.processDxf()
}

func (d *dxfRW) processDxf() bool {
	r := d.r
	code := -1
	inSection := false

	r.ignoreComments = false
	for {
		var ok bool
		code, ok = r.readRec()
		if !ok {
			break
		}
		switch code {
		case 999:
			continue

		case 0:
			// ignore further comments, as libdxfrw doesn't support
			// comments in sections
			r.ignoreComments = true
			if !inSection {
				switch r.getString() {
				case "SECTION":
					inSection = true
					continue
				case "EOF":
					return true
				}
			} else if r.getString() == "ENDSEC" {
				inSection = false
			}

		case 2:
			if inSection {
				var processed bool
				switch r.getString() {
				case "HEADER":
					processed = d.processHeader()
				case "TABLES":
					processed = d.processTables()
				case "BLOCKS":
					processed = d.processBlocks()
				case "ENTITIES":
					processed = d.processEntities(false)
				case "OBJECTS":
					processed = d.processObjects()
				default:
					continue
				}
				if !processed {
					return false
				}
				inSection = false
			}
			continue

		default:
			// an unknown or not supported SECTION
			inSection = false
		}
	}

	// In case the final EOF has no newline.
	return code == 0 && r.getString() == "EOF"
}

// processHeader reads the HEADER section; only the version and code
// page matter here (C++ dxfRW::processHeader and
// DRW_Header::parseCode).
func (d *dxfRW) processHeader() bool {
	r := d.r
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			return r.getString() == "ENDSEC"
		}

		if !d.headerCurr && code != 9 {
			return false
		}
		switch code {
		case 9:
			d.headerCurr = true
			d.headerName = r.getString()
		case 1:
			v := r.getUtf8String()
			if d.headerName == "$ACADVER" {
				r.decoder.setVersionString(v)
			}
		case 3:
			v := r.getUtf8String()
			if d.headerName == "$DWGCODEPAGE" {
				r.decoder.setCodePage(v)
			}
		case 2, 6, 7, 8, 390:
			r.getUtf8String()
		}
	}
}

// processTables reads the TABLES section, passing on the layers (C++
// dxfRW::processTables).
func (d *dxfRW) processTables() bool {
	r := d.r
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code != 0 {
			continue
		}
		switch r.getString() {
		case "TABLE":
			code, ok = r.readRec()
			if !ok {
				return false // wrong dxf file
			}
			if code == 2 {
				switch r.getString() {
				case "LTYPE":
					d.processTable("LTYPE", d.parseLType)
				case "LAYER":
					d.processLayer()
				case "STYLE", "VPORT", "APPID", "DIMSTYLE":
					d.processTable(r.getString(), nil)
				}
			}
		case "ENDSEC":
			return true
		}
	}
}

// parseLType is C++ DRW_LType::parseCode: only its path count can fail.
func (d *dxfRW) parseLType(code int) bool {
	if code == 73 {
		return reserveOK(d.r.getInt32())
	}
	return true
}

// processTable reads a table whose entries SolveSpace ignores (C++
// dxfRW::processLType, processTextStyle and so on); a table's return
// value is ignored, as in C++.
func (d *dxfRW) processTable(entry string, parse func(code int) bool) {
	r := d.r
	reading := false
	for {
		code, ok := r.readRec()
		if !ok {
			return
		}
		if code == 0 {
			switch r.getString() {
			case entry:
				reading = true
			case "ENDTAB":
				return
			}
		} else if reading && parse != nil {
			if !parse(code) {
				return
			}
		}
	}
}

// processLayer reads the LAYER table (C++ dxfRW::processLayer).
func (d *dxfRW) processLayer() bool {
	r := d.r
	reading := false
	var layer Layer
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			if reading {
				l := layer
				d.iface.AddLayer(&l)
			}
			switch r.getString() {
			case "LAYER":
				reading = true
				layer.reset()
			case "ENDTAB":
				return true
			}
		} else if reading {
			layer.parseLayer(code, r)
		}
	}
}

// processBlocks reads the BLOCKS section (C++ dxfRW::processBlocks).
func (d *dxfRW) processBlocks() bool {
	r := d.r
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			switch r.getString() {
			case "BLOCK":
				d.processBlock()
			case "ENDSEC":
				return true
			}
		}
	}
}

// processBlock reads a block (C++ dxfRW::processBlock).
func (d *dxfRW) processBlock() bool {
	r := d.r
	block := newBlock()
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			d.nextentity = r.getString()
			d.iface.AddBlock(&block)
			if d.nextentity != "ENDBLK" {
				d.processEntities(true)
			}
			d.iface.EndBlock()
			return true
		}

		block.parseBlock(code, r)
	}
}

// processEntities reads entities up to ENDSEC or ENDBLK (C++
// dxfRW::processEntities). In a block the entity's name was read
// already, and so C++ drops the first record after it.
func (d *dxfRW) processEntities(isblock bool) bool {
	r := d.r
	code, ok := r.readRec()
	if !ok {
		return false
	}

	if code == 0 {
		d.nextentity = r.getString()
	} else if !isblock {
		return false // first record in entities is 0
	}

	for {
		var processed bool
		switch d.nextentity {
		case "ENDSEC", "ENDBLK":
			return true
		case "POINT":
			p := newPoint()
			processed = d.processEntity(p.parsePoint, func() { d.iface.AddPoint(&p) })
		case "LINE":
			l := newLine()
			processed = d.processEntity(l.parseLine, func() { d.iface.AddLine(&l) })
		case "CIRCLE":
			c := newCircle()
			processed = d.processEntity(c.parseCircle, func() {
				if d.applyExt {
					c.applyExtrusion()
				}
				d.iface.AddCircle(&c)
			})
		case "ARC":
			a := newArc()
			processed = d.processEntity(a.parseArc, func() {
				if d.applyExt {
					a.applyExtrusion()
				}
				d.iface.AddArc(&a)
			})
		case "INSERT":
			in := newInsert()
			processed = d.processEntity(in.parseInsert, func() { d.iface.AddInsert(&in) })
		case "LWPOLYLINE":
			p := newLWPolyline()
			processed = d.processEntity(p.parseLWPolyline, func() {
				if d.applyExt {
					p.applyExtrusion()
				}
				d.iface.AddLWPolyline(&p)
			})
		case "POLYLINE":
			processed = d.processPolyline()
		case "TEXT":
			t := newText()
			processed = d.processEntity(t.parseText, func() { d.iface.AddText(&t) })
		case "MTEXT":
			m := newMText()
			processed = d.processEntity(m.parseMText, func() {
				m.updateAngle()
				d.iface.AddMText(&m)
			})
		case "SPLINE":
			s := newSpline()
			processed = d.processEntity(s.parseSpline, func() { d.iface.AddSpline(&s) })
		case "DIMENSION":
			dim := newDimension()
			processed = d.processEntity(dim.parseDimension, func() {
				kind := dim.Type & 0x0F
				if kind > int(DimOrdinate) {
					return
				}
				dim.Kind = DimensionKind(kind)
				d.iface.AddDimension(&dim)
			})
		case "ELLIPSE", "TRACE", "SOLID", "HATCH", "3DFACE", "VIEWPORT", "IMAGE", "LEADER",
			"RAY", "XLINE":
			// Their parsers never fail, and SolveSpace ignores them.
			processed = d.processEntity(func(int, *reader) bool { return true }, func() {})
		default:
			code, ok := r.readRec()
			if !ok {
				return false // end of file without ENDSEC
			}
			if code == 0 {
				d.nextentity = r.getString()
			}
			processed = true
		}
		if !processed {
			return false
		}
	}
}

// processEntity reads an entity's records into it with parse until the
// next entity, then calls done (the shape of C++ dxfRW::processLine
// and the others).
func (d *dxfRW) processEntity(parse func(code int, r *reader) bool, done func()) bool {
	r := d.r
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			d.nextentity = r.getString()
			done()
			return true
		}
		if !parse(code, r) {
			return false
		}
	}
}

// processPolyline reads a POLYLINE, its vertices and SEQEND (C++
// dxfRW::processPolyline). As in C++, SEQEND's own records are parsed
// into the polyline.
func (d *dxfRW) processPolyline() bool {
	r := d.r
	pl := newPolyline()
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			d.nextentity = r.getString()
			if d.nextentity != "VERTEX" {
				d.iface.AddPolyline(&pl)
				return true
			}
			d.processVertex(&pl)
		}

		if !pl.parsePolyline(code, r) {
			return false
		}
	}
}

// processVertex reads the vertices up to SEQEND (C++
// dxfRW::processVertex).
func (d *dxfRW) processVertex(pl *Polyline) bool {
	r := d.r
	v := newVertex()
	for {
		code, ok := r.readRec()
		if !ok {
			return false
		}
		if code == 0 {
			pl.Vertices = append(pl.Vertices, v)
			d.nextentity = r.getString()
			if d.nextentity == "SEQEND" {
				return true
			}
			if d.nextentity == "VERTEX" {
				v = newVertex()
			}
		}

		if !v.parseVertex(code, r) {
			return false
		}
	}
}

// processObjects reads the OBJECTS section (C++ dxfRW::processObjects).
func (d *dxfRW) processObjects() bool {
	r := d.r
	code, ok := r.readRec()
	if !ok || code != 0 {
		return false // first record in objects must be 0
	}

	d.nextentity = r.getString()
	for {
		switch d.nextentity {
		case "ENDSEC":
			return true
		case "IMAGEDEF", "PLOTSETTINGS":
			if !d.processEntity(func(int, *reader) bool { return true }, func() {}) {
				return false
			}
		default:
			code, ok := r.readRec()
			if !ok {
				return false // end of file without ENDSEC
			}
			if code == 0 {
				d.nextentity = r.getString()
			}
		}
	}
}
