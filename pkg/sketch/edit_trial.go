package sketch

import (
	"hash/maphash"
	"reflect"
	"slices"
	"sync"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A fillet or a chamfer is tried on a copy of the solid before it is
// added or its edges changed (NewFillet, SetFilletEdges, NewChamfer,
// SetChamferEdges), so that an edge it can't round is an error then; the
// regeneration that follows made the same solid again, which for the
// fillet of a wrapped arch's foot is seconds. The trial's solid is kept
// here under a key of all it was made from, and the group's regeneration
// takes it when its own inputs come to the same key: the same copy of
// the solid so far to the bit, the same picks, size and origin. The
// trial is made with the origin the group will have (nextGroupHandle),
// so the faces are named as the group's own.

// maxEditTrials is how many trials' solids are kept: one is taken by
// the regeneration that comes next, and one the interface tried and
// didn't add is forgotten as the next comes.
const maxEditTrials = 2

// editTrials keeps the solids trials made, newest last. A ShellJob's
// copy of the sketch shares it from another goroutine, so it locks; a
// solid is taken out, never shared.
type editTrials struct {
	mu           sync.Mutex
	seedA, seedB maphash.Seed
	kept         []editTrial
	hits         int
}

// editTrial is a solid a trial made, the size it made it at, and the
// key of what it was made from.
type editTrial struct {
	key   solidKey
	shell srf.SShell
	made  float64
}

// editTrials is the sketch's, made on first use; nil when NoSolidCache
// is set, as every solid is then made again.
func (sk *Sketch) editTrials() *editTrials {
	if sk.NoSolidCache {
		return nil
	}
	if sk.trials == nil {
		sk.trials = &editTrials{seedA: maphash.MakeSeed(), seedB: maphash.MakeSeed()}
	}
	return sk.trials
}

// EditTrialsTaken is how many fillets' and chamfers' solids a
// regeneration took from the trials that checked them, rather than
// making them again.
func (sk *Sketch) EditTrialsTaken() int {
	t := sk.trials
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hits
}

// empty reports whether no solid is kept, so a regeneration need not
// compute a key to look one up.
func (t *editTrials) empty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.kept) == 0
}

// keyOf is the key of the edit of sh, a fresh copy of the solid so far,
// at picks, r, origin and flat: every field of the copy, but how its
// work is paced (srf.SShell.PaceBy), which changes nothing it makes.
func (t *editTrials) keyOf(sh *srf.SShell, picks []topo.FilletPick, r float64, origin uint32, flat bool) solidKey {
	h := &keyHash{seedA: t.seedA, seedB: t.seedB}
	for _, f := range reflect.ValueOf(sh).Elem().Fields() {
		if f.Kind() != reflect.Func {
			h.value(f)
		}
	}
	h.value(reflect.ValueOf(picks))
	h.f64(r)
	h.u64(uint64(origin))
	h.boolean(flat)
	return h.sum()
}

// put keeps sh, made at made, under k, forgetting the oldest past
// maxEditTrials.
func (t *editTrials) put(k solidKey, sh srf.SShell, made float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kept = slices.DeleteFunc(t.kept, func(e editTrial) bool { return e.key == k })
	t.kept = append(t.kept, editTrial{key: k, shell: sh, made: made})
	if n := len(t.kept) - maxEditTrials; n > 0 {
		t.kept = slices.Delete(t.kept, 0, n)
	}
}

// take is the solid kept under k, which is then kept no more.
func (t *editTrials) take(k solidKey) (editTrial, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	i := slices.IndexFunc(t.kept, func(e editTrial) bool { return e.key == k })
	if i < 0 {
		return editTrial{}, false
	}
	e := t.kept[i]
	t.kept = slices.Delete(t.kept, i, i+1)
	t.hits++
	return e, true
}

// trialUse is what editSolid does with the trials' solids.
type trialUse int

const (
	// trialKeep is a trial's: the solid made is kept.
	trialKeep trialUse = iota
	// trialTake is a regeneration's: a solid kept is taken.
	trialTake
)

// editSolid is a copy of solid with picks rounded at r, or cut where
// flat (topo.FilletPicksMade), the faces it puts in from origin, paced
// by pace, and the size it made them at. A trial (trialKeep) keeps what
// it made for the regeneration after, which takes it (trialTake) in
// place of making it again. A trial's shell is empty: it is the
// regeneration's.
func (sk *Sketch) editSolid(solid *srf.SShell, picks []topo.FilletPick, r float64, origin uint32, flat bool, use trialUse, pace func()) (srf.SShell, float64, error) {
	var sh srf.SShell
	sh.PaceBy(pace)
	sh.MakeFromCopyOf(solid)
	var key solidKey
	t := sk.editTrials()
	if t != nil && (use == trialKeep || !t.empty()) {
		key = t.keyOf(&sh, picks, r, origin, flat)
	}
	if use == trialTake && key.known() {
		if e, ok := t.take(key); ok {
			e.shell.PaceBy(sh.Pace())
			return e.shell, e.made, nil
		}
	}
	made, err := topo.FilletPicksMade(&sh, picks, r, origin, flat)
	if err == nil && use == trialKeep && key.known() {
		// What the trial made is the regeneration's now: the trial,
		// which only asks whether it can be made, gets no shell.
		t.put(key, sh, made)
		return srf.SShell{}, made, nil
	}
	return sh, made, err
}
