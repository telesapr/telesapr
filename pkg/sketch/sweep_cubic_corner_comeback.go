package sketch

import (
	"math"
	"slices"
)

// Where the path beside a cubic corner comes back near where it has
// been (a cubic bent back across the line before its corner, a loop's
// last lines passing the cubic), the tube of one stretch passes through
// the tube of another. The Booleans join two such tubes where both are
// swept untwisted, each from a run of its own; they fail where one of
// them is twisted (the union of the twisted fitted skin with the other
// tube leaves edges open along their crossing), and where both
// stretches are one run, whose own fitted skin then passes through
// itself and is no solid to join (a loop's run from the profile down,
// across and up the cubic, back past where it started). So, where the
// runs as cut at the corners don't join, they are cut again
// (cutComeBack): a run that comes back near itself is cut smoothly in
// two between (selfSplits), and, twisted, the twist is held still
// beside the run's own corners between lines (a line between two of
// them, held untwisted, takes their mitres where the kernel's holds
// beside them would not fit) and then also where the path comes back
// (comeBackCuts), the twist spread over the rest. A run lying partly
// where it is held and partly not, with no tangent place between to
// cut, is held whole.

// comeBackSpacing is the spacing, as a share of the profile's reach,
// the path is sampled at to find where it comes back.
const comeBackSpacing = 0.25

// comeBackSamples is the most points the path is sampled at; a longer
// path is sampled more coarsely, the reach widened by the spacing.
const comeBackSamples = 2000

// pathSample is a point of the path: the piece, how far along it, how
// far along the path, and where.
type pathSample struct {
	piece int
	s, at float64
	p     [3]float64
}

// samplePath samples chain at about spacing, every piece's ends
// included, its curves at even steps of their parameters, how far along
// summed over the chords between.
func samplePath(chain []pathPiece, spacing float64) []pathSample {
	var out []pathSample
	at := 0.0
	for i, p := range chain {
		s := 0.0
		for ci := range p.curves {
			sb := &p.curves[ci]
			n := max(2, int(math.Ceil(curveLength(sb, 1)/spacing)))
			prev := sb.PointAt(0)
			for j := range n + 1 {
				x := sb.PointAt(float64(j) / float64(n))
				s += x.Minus(prev).Magnitude()
				prev = x
				if j == 0 && (ci > 0 || i > 0) {
					continue
				}
				out = append(out, pathSample{piece: i, s: s, at: at + s, p: [3]float64{x.X, x.Y, x.Z}})
			}
		}
		at += s
	}
	return out
}

func (a pathSample) dist(b pathSample) float64 {
	dx, dy, dz := a.p[0]-b.p[0], a.p[1]-b.p[1], a.p[2]-b.p[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// comeBackPairs are the pairs of samples, i before j, where the path
// comes back within near of itself: walking on along the path from i,
// it goes further than near from it and then comes within near again
// at j. Round a closed path the walk goes on past its end, as far as
// the sample where the walk back from i leaves it, j then taken round.
func comeBackPairs(ss []pathSample, closed bool, near float64) [][2]int {
	n := len(ss)
	if closed {
		// The last sample is the first's point again.
		n--
	}
	var out [][2]int
	for i := range n {
		stop := n
		if closed {
			// Walking back from i, the first sample further than near.
			back := 1
			for back < n && ss[i].dist(ss[(i-back+n)%n]) <= near {
				back++
			}
			stop = i + n - back + 1
		}
		left := false
		for j := i + 1; j < stop; j++ {
			d := ss[i].dist(ss[j%n])
			if d > near {
				left = true
			} else if left {
				out = append(out, [2]int{i, j % n})
			}
		}
	}
	return out
}

// comeBackNear marks the samples of comeBackPairs.
func comeBackNear(ss []pathSample, closed bool, near float64) []bool {
	marked := make([]bool, len(ss))
	for _, p := range comeBackPairs(ss, closed, near) {
		marked[p[0]], marked[p[1]] = true, true
	}
	if closed {
		marked[len(ss)-1] = marked[0]
	}
	return marked
}

// selfSplits are the chain cut smoothly where a run of it (pieces
// between cuts other than cutNone) comes back within twice the
// profile's reach of itself, until none does: its own fitted skin would
// pass through itself, where two runs' tubes are joined by the
// Booleans. Each is cut in the middle of the longest piece between the
// two samples, or between them in the piece they share, so as to leave
// lines beside a corner their length for its mitre. near, where not
// nil, is carried to the halves of a piece cut. changed is false where
// no run comes back.
func (sr *sweepRuns) selfSplits(chain []pathPiece, kinds []int, near []bool) ([]pathPiece, []int, []bool, bool) {
	changed := false
	for range 8 {
		total := 0.0
		for _, p := range chain {
			total += pathLength(p)
		}
		spacing := math.Max(comeBackSpacing*sr.reach, total/comeBackSamples)
		ss := samplePath(chain, spacing)
		run := make([]int, len(chain))
		for i := 1; i < len(chain); i++ {
			run[i] = run[i-1]
			if kinds[i] != cutNone {
				run[i]++
			}
		}
		var pair [2]int
		found := false
		for _, p := range comeBackPairs(ss, sr.closed, 2*sr.reach) {
			a, b := ss[p[0]], ss[p[1]]
			if b.at > a.at && run[a.piece] == run[b.piece] {
				pair, found = p, true
				break
			}
		}
		if !found {
			break
		}
		a, b := ss[pair[0]], ss[pair[1]]
		at, d := a.piece, (a.s+b.s)/2
		if b.piece != a.piece {
			best := -1.0
			for k := a.piece + 1; k < b.piece; k++ {
				if l := pathLength(chain[k]); l > best {
					at, d, best = k, l/2, l
				}
			}
			if best < 0 {
				// Neighbours: the longer of what lies between them.
				la, lb := pathLength(chain[a.piece])-a.s, b.s
				if la >= lb {
					at, d = a.piece, a.s+la/2
				} else {
					at, d = b.piece, lb/2
				}
			}
		}
		p, q := splitAlong(chain[at], d)
		chain = slices.Concat(chain[:at:at], []pathPiece{p, q}, chain[at+1:])
		kinds = slices.Concat(kinds[:at+1:at+1], []int{cutSmooth}, kinds[at+1:])
		if near != nil {
			near = slices.Concat(near[:at+1:at+1], near[at:])
		}
		changed = true
	}
	return chain, kinds, near, changed
}

// comeBackCuts are the chain cut again smoothly beside the corners
// within runs and, with back, about where the path comes back within
// twice the profile's reach of itself, the kinds of the cuts (kinds[i]
// the cut before piece i), and which pieces lie there, to be held. A
// stretch where the path comes back is held from a spacing before the
// first such sample to a spacing past the last; one left between two
// held stretches, or between one and a piece's end, shorter than the
// reach is held too.
func (sr *sweepRuns) comeBackCuts(chain []pathPiece, kinds []int, back bool) ([]pathPiece, []int, []bool) {
	total := 0.0
	for _, p := range chain {
		total += pathLength(p)
	}
	spacing := math.Max(comeBackSpacing*sr.reach, total/comeBackSamples)
	ss := samplePath(chain, spacing)
	marked := make([]bool, len(ss))
	if back {
		marked = comeBackNear(ss, sr.closed, 2*sr.reach)
	}
	spans := make([][][2]float64, len(chain))
	for i, smp := range ss {
		if !marked[i] {
			continue
		}
		l := pathLength(chain[smp.piece])
		spans[smp.piece] = append(spans[smp.piece], [2]float64{math.Max(0, smp.s-spacing), math.Min(l, smp.s+spacing)})
	}
	// The run's own corners, between lines and arcs, are held still as
	// far as a mitre reaches too: twisted, the kernel holds the twist
	// still beside them, and a line beside two corners is then too short
	// for both holds where, untwisted, it takes both mitres.
	for i := 1; i < len(chain); i++ {
		d1, d2 := chain[i-1].tangentAt(true), chain[i].tangentAt(false)
		if kinds[i] != cutNone || d1.Dot(d2) >= sweepTangentCos {
			continue
		}
		h := sr.holdLength(math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2)))))
		l := pathLength(chain[i-1])
		spans[i-1] = append(spans[i-1], [2]float64{math.Max(0, l-h), l})
		spans[i] = append(spans[i], [2]float64{0, math.Min(pathLength(chain[i]), h)})
	}
	var out []pathPiece
	var near []bool
	outKinds := []int{cutNone}
	for i, p := range chain {
		outKinds[len(outKinds)-1] = kinds[i]
		held := mergeSpans(spans[i], pathLength(p), sr.reach)
		if len(held) == 0 {
			out, near = append(out, p), append(near, false)
			outKinds = append(outKinds, cutNone)
			continue
		}
		// The piece's stretches, held or not, from its start.
		var cuts []float64
		var heldFrom []bool
		at := 0.0
		for _, h := range held {
			if h[0] > at {
				cuts, heldFrom = append(cuts, h[0]), append(heldFrom, false)
			}
			cuts, heldFrom = append(cuts, h[1]), append(heldFrom, true)
			at = h[1]
		}
		l := pathLength(p)
		if at < l {
			cuts, heldFrom = append(cuts, l), append(heldFrom, false)
		}
		rest, done := p, 0.0
		for j, c := range cuts {
			q := rest
			if j+1 < len(cuts) {
				q, rest = splitAlong(rest, c-done)
				done = c
			}
			out, near = append(out, q), append(near, heldFrom[j])
			if j+1 < len(cuts) {
				outKinds = append(outKinds, cutSmooth)
			} else {
				outKinds = append(outKinds, cutNone)
			}
		}
	}
	outKinds[len(outKinds)-1] = cutNone
	// Pieces held and not that meet tangent with no cut between are
	// cut there, so that each run is held or not throughout.
	for i := 1; i < len(out); i++ {
		if outKinds[i] == cutNone && near[i] != near[i-1] &&
			out[i-1].tangentAt(true).Dot(out[i].tangentAt(false)) >= sweepTangentCos {
			outKinds[i] = cutSmooth
		}
	}
	return out, outKinds, near
}

// mergeSpans are the spans, along a piece of length l, merged where
// they overlap or leave less than minGap between them or to an end.
func mergeSpans(spans [][2]float64, l, minGap float64) [][2]float64 {
	if len(spans) == 0 {
		return nil
	}
	slices.SortFunc(spans, func(a, b [2]float64) int {
		switch {
		case a[0] < b[0]:
			return -1
		case a[0] > b[0]:
			return 1
		}
		return 0
	})
	out := [][2]float64{spans[0]}
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s[0] <= last[1]+minGap {
			last[1] = math.Max(last[1], s[1])
			continue
		}
		out = append(out, s)
	}
	if out[0][0] < minGap {
		out[0][0] = 0
	}
	if last := &out[len(out)-1]; l-last[1] < minGap {
		last[1] = l
	}
	return out
}

// comeBackHeld marks the runs whose pieces lie where the path comes
// back (comeBackCuts' near), held untwisted.
func (sr *sweepRuns) comeBackHeld(near []bool) {
	sr.nearHeld = make([]bool, len(sr.chains))
	i := 0
	for k, run := range sr.chains {
		for range run {
			sr.nearHeld[k] = sr.nearHeld[k] || near[i]
			i++
		}
	}
}

// cutComeBack is the runs cut again: a run coming back near itself cut
// in two (selfSplits) and, twisted, the stretches beside the runs' own
// corners and, with back, where the path comes back near another part
// of it cut out (comeBackCuts) and held untwisted, with the twist held
// still beside each corner beside a cubic over hold times holdLength
// too and spread over the rest; fitted. A pathError where that leaves nowhere to twist, and nil where
// it cuts nothing.
func (sr *sweepRuns) cutComeBack(hold float64, back bool) (*sweepRuns, error) {
	c := &sweepRuns{up: sr.up, reach: sr.reach, closed: sr.closed, whole: sr.whole, kinds: sr.kinds,
		twist: sr.twist, hold: hold}
	c.cut(sr.whole, sr.kinds)
	chain, kinds := sr.whole, sr.kinds
	var near []bool
	held := false
	if sr.twist != 0 {
		chain, kinds = c.heldCuts(chain, kinds)
		chain, kinds, near = c.comeBackCuts(chain, kinds, back)
		held = slices.Contains(near, true)
	}
	chain, kinds, near, split := c.selfSplits(chain, kinds, near)
	if !held && !split {
		return nil, nil
	}
	c.cut(chain, kinds)
	if near != nil {
		c.comeBackHeld(near)
	}
	if sr.twist != 0 {
		if err := c.spreadTwist(sr.twist); err != nil {
			return nil, err
		}
	}
	if err := c.fit(); err != nil {
		return nil, err
	}
	return c, nil
}
