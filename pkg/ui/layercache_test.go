package ui

import "testing"

// TestLayerCacheSameFrames runs the interaction scripts with the
// persistent layer cache and without it, and checks that every frame
// the graphics window presents is the same: the cache must never show
// a stale picture.
func TestLayerCacheSameFrames(t *testing.T) {
	if testing.Short() {
		t.Skip("slow; skipped with -short")
	}
	for _, name := range uiScripts(t, "act_") {
		t.Run(name, func(t *testing.T) {
			var cached, drawn []uint64
			hits := 0
			outCached := runUIScriptWith(t, name, func(r *uiScriptRunner) {
				r.frames = &cached
				r.ss.GW.persistent.onHit = func() { hits++ }
			})
			outDrawn := runUIScriptWith(t, name, func(r *uiScriptRunner) {
				r.frames = &drawn
				r.ss.GW.persistent.disabled = true
			})
			if outCached != outDrawn {
				t.Fatal("the script's output depends on the cache")
			}
			if len(cached) != len(drawn) {
				t.Fatalf("%d frames with the cache, %d without", len(cached), len(drawn))
			}
			for i := range cached {
				if cached[i] != drawn[i] {
					t.Fatalf("frame %d of %d differs with the cache", i+1, len(cached))
				}
			}
			t.Logf("%d frames, %d from the cache", len(cached), hits)
		})
	}
}
