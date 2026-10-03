package link

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/step"
)

// A STEP file's tessellation is kept on disk between sessions (roadmap
// 17.4): a large part takes seconds to triangulate, and the same file
// at the same chord tolerance always gives the same mesh. The cache is
// TELESAPR_CACHE_DIR, or the user's cache folder's telesapr/step; a
// file is named by a hash of the STEP file's path, size, modification
// time and the chord tolerance, and the format's version. What can't
// be read or written there is simply made again. Under go test, only
// TELESAPR_CACHE_DIR is used.

// stepDiskVersion changes when what is kept, or how it is made, does.
const stepDiskVersion = 1

var (
	// stepDiskHits and stepDiskWrites count, for the tests.
	stepDiskHits, stepDiskWrites atomic.Int64
)

// stepCacheDir is the folder of the kept tessellations, or "".
func stepCacheDir() string {
	if d := os.Getenv("TELESAPR_CACHE_DIR"); d != "" {
		return filepath.Join(d, "step")
	}
	// Tests keep nothing in the user's folder unless told where.
	if testing.Testing() {
		return ""
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "telesapr", "step")
}

// stepDiskFile is the file the tessellation for key is kept in.
func stepDiskFile(key stepKey) string {
	dir := stepCacheDir()
	if dir == "" {
		return ""
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%d|%d|%g", stepDiskVersion, key.path, key.size, key.mtime.UnixNano(), key.pct)))
	return filepath.Join(dir, hex.EncodeToString(h[:16])+".gob")
}

// readStepDisk is the kept tessellation for key, if there is one.
func readStepDisk(key stepKey) (*step.Result, bool) {
	file := stepDiskFile(key)
	if file == "" {
		return nil, false
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	var res step.Result
	if err := gob.NewDecoder(f).Decode(&res); err != nil {
		return nil, false
	}
	stepDiskHits.Add(1)
	return &res, true
}

// writeStepDisk keeps res for key, best effort: written aside, then
// renamed, so that a reader never sees half of one.
func writeStepDisk(key stepKey, res *step.Result) {
	file := stepDiskFile(key)
	if file == "" || os.MkdirAll(filepath.Dir(file), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), "step-*.tmp")
	if err != nil {
		return
	}
	err = gob.NewEncoder(tmp).Encode(res)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	stepDiskWrites.Add(1)
}
