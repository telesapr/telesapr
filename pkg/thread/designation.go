package thread

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Parse reads a thread designation:
//
//	M8             ISO metric, coarse pitch
//	M8x1           ISO metric, 1 mm pitch
//	M16xPh3P1.5    ISO metric, 1.5 mm pitch, 3 mm lead (two starts)
//	Tr20x4         ISO metric trapezoidal
//	Tr20x8(P4)     trapezoidal, 4 mm pitch, 8 mm lead
//	1/4-20 UNC     unified inch, 20 threads per inch
//	#10 UNF        unified inch, the fine thread of the size
//	1/2-13 UNC (2 starts)
//	1/4 BSW        Whitworth, the coarse (BSW) pitch of the size
//	3/8-20 BSF     Whitworth, 20 threads per inch
//	G1/2           parallel pipe thread (BSP), of ISO 228-1
//	1/2-10 ACME    general purpose ACME, 10 threads per inch
//	S20x4          DIN 513 metric buttress
//	S40x14(P7)     buttress, 7 mm pitch, 14 mm lead
//	1-12 BUTT      ASME B1.9 buttress inch, 12 threads per inch, pulling
//	1-12 PUSH-BUTT the same, pushing (PULL-BUTT is read as BUTT)
//	2-4 BUTT-2A    with its class, which the thread's options keep
//
// A designation ending in -LH is a left-hand thread. Letter case and
// spaces don't matter, and the multiplication sign may stand for the x.
func Parse(text string) (Spec, error) {
	t := strings.ToUpper(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, "×", "X")
	if t == "" {
		return Spec{}, fmt.Errorf("no thread designation")
	}

	var s Spec
	for _, hand := range []string{"LH", "RH"} {
		if rest, ok := strings.CutSuffix(t, hand); ok && rest != "" && strings.ContainsAny(rest[len(rest)-1:], "- ") {
			t = strings.TrimRight(rest, "- ")
			s.LeftHand = hand == "LH"
			break
		}
	}

	var err error
	switch {
	case strings.HasPrefix(t, "TR"):
		s.Form = FormTrapezoidal
		err = s.parseMetric(t[2:], text)
	case strings.HasPrefix(t, "M"):
		s.Form = FormMetric
		err = s.parseMetric(t[1:], text)
	case strings.HasPrefix(t, "S") && len(t) > 1 && strings.ContainsAny(t[1:2], "0123456789 "):
		s.Form = FormButtress
		err = s.parseMetric(t[1:], text)
	case strings.HasPrefix(t, "G") && len(t) > 1 && strings.ContainsAny(t[1:2], "0123456789 "):
		s.Form = FormWhitworth
		err = s.parsePipe(t[1:], text)
	default:
		s.Form = FormUnified
		err = s.parseInch(t, text)
	}
	if err != nil {
		return Spec{}, err
	}
	if err := s.Validate(); err != nil {
		return Spec{}, err
	}
	return s, nil
}

// parseMetric reads what follows the M or Tr of a designation; text is
// the whole of it, for messages.
func (s *Spec) parseMetric(t, text string) error {
	bad := fmt.Errorf("can't read '%s' as a thread designation", strings.TrimSpace(text))
	size, rest, hasPitch := strings.Cut(t, "X")
	var ok bool
	if s.Diameter, ok = parseNumber(size); !ok {
		return bad
	}
	if !hasPitch {
		switch s.Form {
		case FormTrapezoidal:
			s.Pitch, ok = TrapezoidalPitch(s.Diameter)
		case FormButtress:
			s.Pitch, ok = ButtressPitch(s.Diameter)
		default:
			s.Pitch, ok = CoarsePitch(s.Diameter)
		}
		if !ok {
			return s.unknownSize()
		}
		return nil
	}

	// A lead and a pitch: Ph3P1.5 for the metric thread, 8(P4) or 8P4
	// for the trapezoidal and buttress ones.
	rest = strings.TrimSpace(rest)
	lead := ""
	if s.Form == FormMetric {
		if after, isLead := strings.CutPrefix(rest, "PH"); isLead {
			if lead, rest, ok = strings.Cut(after, "P"); !ok {
				return bad
			}
		}
	} else if before, after, isLead := strings.Cut(rest, "P"); isLead {
		lead = strings.TrimSuffix(strings.TrimSpace(before), "(")
		rest = strings.TrimSuffix(strings.TrimSpace(after), ")")
	}
	if s.Pitch, ok = parseNumber(rest); !ok {
		return bad
	}
	if lead != "" {
		l, ok := parseNumber(lead)
		if !ok || !(s.Pitch > 0) {
			return bad
		}
		starts := math.Round(l / s.Pitch)
		if starts < 1 || math.Abs(starts*s.Pitch-l) > 1e-6 {
			return fmt.Errorf("the lead of a thread must be a multiple of its pitch")
		}
		s.Starts = int(starts)
	}
	return nil
}

// unknownSize is the error for a metric, trapezoidal or buttress size that the
// series lacks: a pitch to give it, which makes a thread that can be
// made, and the nearest standard size.
func (s *Spec) unknownSize() error {
	near := nearestSize(s.Form, s.Diameter)
	try := Spec{Form: s.Form, Diameter: s.Diameter, Pitch: near.Pitch}
	for range 30 {
		if try.Validate() == nil {
			break
		}
		try.Pitch /= 2
	}
	prefix := map[Form]string{FormMetric: "M", FormTrapezoidal: "Tr", FormButtress: "S"}[s.Form]
	return fmt.Errorf("%s%s isn't a standard size; give its pitch, as in %s, or take the nearest, %s",
		prefix, formatNumber(s.Diameter), try, near)
}

// parseStarts takes a trailing "(n starts)" off t.
func (s *Spec) parseStarts(t string, bad error) (string, error) {
	before, after, ok := strings.Cut(t, "(")
	if !ok {
		return t, nil
	}
	n, isStarts := strings.CutSuffix(strings.TrimSpace(after), "STARTS)")
	starts, err := strconv.Atoi(strings.TrimSpace(n))
	if !isStarts || err != nil || starts < 1 {
		return "", bad
	}
	s.Starts = starts
	return strings.TrimSpace(before), nil
}

// parsePipe reads what follows the G of a pipe thread's designation:
// the name of a size of ISO 228-1.
func (s *Spec) parsePipe(t, text string) error {
	bad := fmt.Errorf("can't read '%s' as a thread designation", strings.TrimSpace(text))
	t, err := s.parseStarts(t, bad)
	if err != nil {
		return err
	}
	// The tolerance class of an external pipe thread, A or B.
	t = strings.TrimSpace(t)
	for _, class := range []string{" A", " B"} {
		t = strings.TrimSuffix(t, class)
	}
	size, ok := parseInches(t)
	if !ok {
		return bad
	}
	for _, p := range pipe {
		if v, _ := parseInches(p.name); same(v, size) {
			s.Diameter, s.Pitch = p.d, mmPerInch/p.tpi
			return nil
		}
	}
	return fmt.Errorf("no pipe thread size %s in ISO 228-1", strings.TrimSpace(t))
}

// inchSeries are the series an inch designation may name, with the
// form each gives and the table of its sizes' threads per inch.
var inchSeries = []struct {
	name  string
	form  Form
	table []inchSize
}{
	{"UNC", FormUnified, nil}, {"UNF", FormUnified, nil}, {"UNEF", FormUnified, nil},
	{"UNS", FormUnified, nil}, {"UN", FormUnified, nil},
	{"BSW", FormWhitworth, bsw}, {"BSF", FormWhitworth, bsf}, {"ACME", FormACME, acme},
	{"BUTT", FormButtressInch, buttressInch},
}

// parseInch reads an inch designation: a size, then the threads per
// inch or the name of a series, or both.
func (s *Spec) parseInch(t, text string) error {
	bad := fmt.Errorf("can't read '%s' as a thread designation", strings.TrimSpace(text))
	t, err := s.parseStarts(t, bad)
	if err != nil {
		return err
	}
	// The class of an ACME thread, as in 1/2-10 ACME-2G, or of a B1.9
	// buttress one, as in 2-4 BUTT-2A, which the options keep apart.
	if i := strings.LastIndex(t, "ACME"); i >= 0 {
		if class := strings.Trim(t[i+4:], "- "); len(class) == 2 && class[1] == 'G' && strings.ContainsAny(class[:1], "2345") {
			t = t[:i+4]
		}
	}
	if i := strings.LastIndex(t, "BUTT"); i >= 0 {
		if class := strings.Trim(t[i+4:], "- "); len(class) == 2 && strings.ContainsAny(class[:1], "23") && strings.ContainsAny(class[1:], "AB") {
			t = t[:i+4]
		}
	}
	series := ""
	var table []inchSize
	for _, sr := range inchSeries {
		if rest, ok := strings.CutSuffix(t, sr.name); ok {
			series, t, table, s.Form = sr.name, strings.TrimRight(rest, "- "), sr.table, sr.form
			if s.Form == FormButtressInch {
				t = s.parseLoad(t)
			}
			break
		}
	}

	size, count := t, ""
	if i := strings.LastIndex(t, "-"); i > 0 {
		// The dash may also join the parts of a size, as in 1-1/8.
		if _, ok := parseInches(t); !ok || !strings.Contains(t[i:], "/") {
			size, count = t[:i], t[i+1:]
		}
	}
	inches, ok := parseInches(size)
	if !ok {
		return bad
	}
	var tpi float64
	if count != "" {
		if tpi, ok = parseNumber(count); !ok {
			return bad
		}
	} else {
		for _, u := range unified {
			if same(u.d, inches) {
				switch series {
				case "UNC":
					tpi = u.unc
				case "UNF":
					tpi = u.unf
				}
			}
		}
		for _, row := range table {
			if same(row.d, inches) {
				tpi = row.tpi
			}
		}
		if tpi == 0 {
			if series == "UNC" || series == "UNF" || table != nil {
				return fmt.Errorf("the %s series has no size '%s'", series, strings.TrimSpace(size))
			}
			return fmt.Errorf("'%s' lacks its threads per inch, as in 1/4-20, or its series, as in 1/4 UNC",
				strings.TrimSpace(text))
		}
	}
	if !(tpi > 0) {
		return bad
	}
	s.Diameter = inches * mmPerInch
	s.Pitch = mmPerInch / tpi
	return nil
}

// parseNumber reads a decimal number, with a point or a comma.
func parseNumber(t string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(t), ",", ".", 1), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// parseInches reads the size of an inch thread: a gauge number (#10),
// a fraction (1/4), a whole number and a fraction (1 1/8, 1-1/8), or a
// decimal.
func parseInches(t string) (float64, bool) {
	t = strings.TrimSpace(t)
	if gauge, ok := strings.CutPrefix(t, "#"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(gauge))
		if err != nil || n < 0 || n > 14 {
			return 0, false
		}
		return 0.060 + 0.013*float64(n), true
	}
	whole := 0.0
	if i := strings.IndexAny(t, " -"); i > 0 {
		n, err := strconv.Atoi(t[:i])
		if err != nil || n < 0 {
			return 0, false
		}
		whole, t = float64(n), strings.TrimLeft(t[i:], " -")
		if !strings.Contains(t, "/") {
			return 0, false
		}
	}
	if num, den, ok := strings.Cut(t, "/"); ok {
		n, err1 := strconv.Atoi(strings.TrimSpace(num))
		d, err2 := strconv.Atoi(strings.TrimSpace(den))
		if err1 != nil || err2 != nil || n <= 0 || d <= 0 {
			return 0, false
		}
		return whole + float64(n)/float64(d), true
	}
	return parseNumber(t)
}

// formatNumber writes a size without the noise of its last bits.
func formatNumber(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64)
}

// String is the thread's designation, which Parse reads back.
func (s Spec) String() string {
	var b strings.Builder
	switch s.Form {
	case FormMetric:
		b.WriteString("M" + formatNumber(s.Diameter))
		coarse, ok := CoarsePitch(s.Diameter)
		switch {
		case s.NumStarts() > 1:
			b.WriteString("xPh" + formatNumber(s.Lead()) + "P" + formatNumber(s.Pitch))
		case !ok || !same(coarse, s.Pitch):
			b.WriteString("x" + formatNumber(s.Pitch))
		}
	case FormTrapezoidal, FormButtress:
		b.WriteString(map[Form]string{FormTrapezoidal: "Tr", FormButtress: "S"}[s.Form] +
			formatNumber(s.Diameter) + "x" + formatNumber(s.Lead()))
		if s.NumStarts() > 1 {
			b.WriteString("(P" + formatNumber(s.Pitch) + ")")
		}
	case FormUnified:
		inches, tpi := s.Diameter/mmPerInch, mmPerInch/s.Pitch
		size, series := formatNumber(inches), "UN"
		for _, u := range unified {
			if same(u.d, inches) {
				size = u.name
				switch {
				case same(u.unc, tpi):
					series = "UNC"
				case same(u.unf, tpi):
					series = "UNF"
				}
			}
		}
		b.WriteString(size + "-" + formatNumber(tpi) + " " + series)
		s.writeStarts(&b)
	case FormWhitworth:
		s.writeWhitworth(&b)
		s.writeStarts(&b)
	case FormACME:
		b.WriteString(inchName(s.Diameter/mmPerInch, acme) + "-" + formatNumber(mmPerInch/s.Pitch) + " ACME")
		s.writeStarts(&b)
	case FormButtressInch:
		b.WriteString(inchName(s.Diameter/mmPerInch, buttressInch) + "-" + formatNumber(mmPerInch/s.Pitch) + " " +
			loadNames[s.Load] + "BUTT")
		s.writeStarts(&b)
	}
	if s.LeftHand {
		b.WriteString("-LH")
	}
	return b.String()
}

// writeStarts writes the starts of an inch thread of several.
func (s Spec) writeStarts(b *strings.Builder) {
	if s.NumStarts() > 1 {
		fmt.Fprintf(b, " (%d starts)", s.NumStarts())
	}
}

// writeWhitworth writes a Whitworth thread: as a pipe thread for a
// pipe size at its pitch, as BSW or BSF without the pitch for a size
// at that series' pitch, and with the threads per inch otherwise.
func (s Spec) writeWhitworth(b *strings.Builder) {
	inches, tpi := s.Diameter/mmPerInch, mmPerInch/s.Pitch
	if p, ok := pipeSize(s.Diameter); ok && same(p.tpi, tpi) {
		b.WriteString("G" + p.name)
		return
	}
	for _, sr := range []struct {
		name  string
		table []inchSize
	}{{"BSW", bsw}, {"BSF", bsf}} {
		for _, row := range sr.table {
			if same(row.d, inches) && same(row.tpi, tpi) {
				b.WriteString(row.name + " " + sr.name)
				return
			}
		}
	}
	b.WriteString(inchName(inches, bsw, bsf) + "-" + formatNumber(tpi) + " BSW")
}

// inchName is the name of a size of the tables, or its decimal inches.
func inchName(inches float64, tables ...[]inchSize) string {
	for _, table := range tables {
		for _, row := range table {
			if same(row.d, inches) {
				return row.name
			}
		}
	}
	return formatNumber(inches)
}

// Note says how a designation was read when it is written otherwise,
// as M8x1.25 is written M8, so that the change isn't made quietly; ""
// when text is s's designation but for letter case, spaces and dashes.
func Note(text string, s Spec) string {
	squeeze := func(t string) string {
		t = strings.ToUpper(strings.ReplaceAll(t, "×", "X"))
		return strings.NewReplacer(" ", "", "-", "").Replace(t)
	}
	if squeeze(text) == squeeze(s.String()) {
		return ""
	}
	return fmt.Sprintf("'%s' is written %s", strings.TrimSpace(text), s)
}

// parseLoad takes the way of the load, PUSH or PULL, off the end of t,
// what comes before BUTT in a designation, and returns the rest. A
// designation without either pulls, as B1.9 has it.
func (s *Spec) parseLoad(t string) string {
	s.Load = LoadPull
	for _, w := range []struct {
		word string
		load Load
	}{{"PUSH", LoadPush}, {"PULL", LoadPull}} {
		if rest, ok := strings.CutSuffix(t, w.word); ok {
			s.Load = w.load
			return strings.TrimRight(rest, "- ")
		}
	}
	return t
}
