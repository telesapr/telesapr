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
	default:
		s.Form = FormUnified
		err = s.parseUnified(t, text)
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
		if s.Form == FormTrapezoidal {
			s.Pitch, ok = TrapezoidalPitch(s.Diameter)
		} else {
			s.Pitch, ok = CoarsePitch(s.Diameter)
		}
		if !ok {
			prefix := map[Form]string{FormMetric: "M", FormTrapezoidal: "Tr"}[s.Form]
			return fmt.Errorf("%s%s isn't a standard size; give its pitch, as in %s%sx1",
				prefix, formatNumber(s.Diameter), prefix, formatNumber(s.Diameter))
		}
		return nil
	}

	// A lead and a pitch: Ph3P1.5 for the metric thread, 8(P4) or 8P4
	// for the trapezoidal one.
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

// parseUnified reads an inch designation: a size, then the threads per
// inch or the name of a series, or both.
func (s *Spec) parseUnified(t, text string) error {
	bad := fmt.Errorf("can't read '%s' as a thread designation", strings.TrimSpace(text))
	if before, after, ok := strings.Cut(t, "("); ok {
		n, isStarts := strings.CutSuffix(strings.TrimSpace(after), "STARTS)")
		starts, err := strconv.Atoi(strings.TrimSpace(n))
		if !isStarts || err != nil || starts < 1 {
			return bad
		}
		s.Starts, t = starts, strings.TrimSpace(before)
	}
	series := ""
	for _, name := range []string{"UNC", "UNF", "UNEF", "UNS", "UN"} {
		if rest, ok := strings.CutSuffix(t, name); ok {
			series, t = name, strings.TrimRight(rest, "- ")
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
		if tpi == 0 {
			if series == "UNC" || series == "UNF" {
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
	case FormTrapezoidal:
		b.WriteString("Tr" + formatNumber(s.Diameter) + "x" + formatNumber(s.Lead()))
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
		if s.NumStarts() > 1 {
			fmt.Fprintf(&b, " (%d starts)", s.NumStarts())
		}
	}
	if s.LeftHand {
		b.WriteString("-LH")
	}
	return b.String()
}
