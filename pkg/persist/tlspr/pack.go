package tlspr

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A packed model (roadmap 17.5, S3) is a zip: the model, model.tlspr,
// with what it links beside it, in parts/ (and what those link, all in
// parts/, their links made to name each other there), its pictures in
// images/, and the tessellations kept of its STEP files (17.4) in
// cache/step/, so that it opens elsewhere without triangulating them
// again. Opening one unpacks it into a folder of its own; its links
// then name the files there, and saving it packed again packs them
// again. The zip's dates are fixed, so one model packs to the same
// bytes.

// PackedExtension is the packed model's file extension, without its dot.
const PackedExtension = "tlsprz"

// IsPacked reports whether path names a packed model.
func IsPacked(path string) bool {
	return strings.EqualFold(filepath.Ext(path), "."+PackedExtension)
}

// packer gathers a pack's files.
type packer struct {
	files map[string][]byte
	// names are the zip names given to the linked files, by absolute
	// path, so that a file linked twice is packed once.
	names map[string]string
}

// add puts the file at abs into dir/ of the zip, once, and returns its
// zip name.
func (p *packer) add(abs, dir string, data []byte) string {
	if n, ok := p.names[abs]; ok {
		return n
	}
	base := filepath.Base(abs)
	ext := filepath.Ext(base)
	name := path.Join(dir, base)
	for k := 2; p.files[name] != nil; k++ {
		name = path.Join(dir, fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, ext), k, ext))
	}
	p.names[abs] = name
	p.files[name] = data
	return name
}

// part packs the linked file at abs, and what it links in turn, and
// returns its zip name.
func (p *packer) part(abs string, pct float64) (string, error) {
	if n, ok := p.names[abs]; ok {
		return n, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(abs), "."+Extension) {
		// A model: its own links made to name the parts beside it.
		doc, err := Decode(data)
		if err != nil {
			return "", fmt.Errorf("%s: %w", abs, err)
		}
		name := p.add(abs, "parts", []byte{}) // filled below
		if err := p.links(doc.Sketch, abs, "parts", pct); err != nil {
			return "", err
		}
		p.files[name] = doc.Encode()
		return name, nil
	}
	name := p.add(abs, "parts", data)
	if strings.EqualFold(filepath.Ext(abs), ".step") || strings.EqualFold(filepath.Ext(abs), ".stp") {
		if dir := link.StepCacheDir(); dir != "" {
			kept := link.StepCacheName(data, pct)
			if mesh, err := os.ReadFile(filepath.Join(dir, kept)); err == nil {
				p.files["cache/step/"+kept] = mesh
			}
		}
	}
	return name, nil
}

// links packs what sk, a model at from, links and shows, and makes its
// links name them as seen from the model's folder in the zip, dir.
func (p *packer) links(sk *sketch.Sketch, from, dir string, pct float64) error {
	rel := func(name string) string {
		r, _ := filepath.Rel(dir, name)
		return filepath.ToSlash(r)
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type != sketch.GroupLinked || g.LinkFile == "" {
			continue
		}
		abs, err := filepath.Abs(link.Path(from, g.LinkFile))
		if err != nil {
			return err
		}
		name, err := p.part(abs, pct)
		if err != nil {
			return fmt.Errorf("the linked file %s: %w", g.LinkFile, err)
		}
		g.LinkFile = rel(name)
	}
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Type != sketch.RequestImage || r.File == "" {
			continue
		}
		abs, err := filepath.Abs(link.Path(from, r.File))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("the picture %s: %w", r.File, err)
		}
		r.File = rel(p.add(abs, "images", data))
	}
	return nil
}

// Pack writes sk, a model saved or opened at from ("" for a new one),
// as a packed model at path, with what it links. sk's links are left
// absolute.
func Pack(sk *sketch.Sketch, from, out string, o Options) error {
	p := &packer{files: map[string][]byte{}, names: map[string]string{}}
	// The links are made to name the packed files while the model is
	// encoded, then put back, absolute: what saved the model packed
	// takes the pack for where the model is, and its links must still
	// name the files.
	base := from
	if base == "" {
		base = out
	}
	type kept struct{ links, pictures []string }
	var k kept
	for _, g := range sk.Groups {
		k.links = append(k.links, g.LinkFile)
	}
	for _, r := range sk.Requests {
		k.pictures = append(k.pictures, r.File)
	}
	defer func() {
		abs := func(l string) string {
			if l == "" {
				return ""
			}
			a, err := filepath.Abs(link.Path(base, l))
			if err != nil {
				return l
			}
			return a
		}
		for i := range sk.Groups {
			sk.Groups[i].LinkFile = abs(k.links[i])
		}
		for i := range sk.Requests {
			sk.Requests[i].File = abs(k.pictures[i])
		}
	}()
	if err := p.links(sk, base, ".", sk.ChordTolPct); err != nil {
		return err
	}
	p.files["model.tlspr"] = Encode(sk, o)

	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	names := make([]string, 0, len(p.files))
	for n := range p.files {
		if n != "model.tlspr" {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	when := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range append([]string{"model.tlspr"}, names...) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate, Modified: when})
		if err != nil {
			return err
		}
		if _, err := w.Write(p.files[n]); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(out, b.Bytes(), 0o644)
}

// The bounds on a packed model as it is read, so that a small file
// that declares or inflates to a huge one cannot exhaust memory: a
// file of the pack, the pack as a whole, and how many files it holds.
// A model with its parts, pictures and tessellations is megabytes.
const (
	MaxPackedFile  = 256 << 20
	MaxPackedTotal = 1 << 30
	MaxPackedFiles = 10000
)

// Unpack writes the files of the packed model at in into dir, and the
// tessellations it keeps into the STEP cache, and returns the path of
// its model. It refuses a pack over the bounds above, or one whose
// files would be written outside dir; the tessellations are kept only
// when the pack holds a model.
func Unpack(in, dir string) (string, error) {
	zr, err := zip.OpenReader(in)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	if len(zr.File) > MaxPackedFiles {
		return "", fmt.Errorf("%s: the packed model holds %d files, more than the %d allowed", in, len(zr.File), MaxPackedFiles)
	}
	model := ""
	var total uint64
	var cached []*zip.File
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("%s: the packed file %q would be written outside the folder", in, f.Name)
		}
		if f.UncompressedSize64 > MaxPackedFile {
			return "", fmt.Errorf("%s: the packed file %q is %d bytes, more than the %d allowed", in, f.Name, f.UncompressedSize64, MaxPackedFile)
		}
		total += f.UncompressedSize64
		if total > MaxPackedTotal {
			return "", fmt.Errorf("%s: the packed model is over %d bytes unpacked", in, MaxPackedTotal)
		}
		if strings.HasPrefix(f.Name, "cache/step/") {
			cached = append(cached, f)
			continue
		}
		data, err := readPacked(f)
		if err != nil {
			return "", fmt.Errorf("%s: %w", in, err)
		}
		dst := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return "", err
		}
		if f.Name == "model.tlspr" {
			model = dst
		}
	}
	if model == "" {
		return "", errors.New(in + ": no model.tlspr in the packed model")
	}
	cache := link.StepCacheDir()
	for _, f := range cached {
		rest := strings.TrimPrefix(f.Name, "cache/step/")
		if cache == "" || !filepath.IsLocal(rest) || strings.ContainsRune(rest, '/') {
			continue
		}
		dst := filepath.Join(cache, rest)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		data, err := readPacked(f)
		if err != nil || os.MkdirAll(cache, 0o755) != nil {
			continue
		}
		_ = os.WriteFile(dst, data, 0o644)
	}
	return model, nil
}

// readPacked reads one file of a pack, and refuses one that inflates
// to more than it declares (the zip reader does too, once read past).
func readPacked(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(io.LimitReader(r, int64(f.UncompressedSize64)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(data)) > f.UncompressedSize64 {
		return nil, fmt.Errorf("the packed file %q is larger than it declares", f.Name)
	}
	return data, nil
}

// LoadPacked opens a packed model: it is unpacked into a new folder of
// its own, and its model opened there, its links made absolute so that
// they hold wherever the model is saved.
func LoadPacked(path string) (*Document, error) {
	dir, err := os.MkdirTemp("", "telesapr-packed-")
	if err != nil {
		return nil, err
	}
	model, err := Unpack(path, dir)
	if err != nil {
		return nil, err
	}
	doc, err := LoadFile(model)
	if err != nil {
		return nil, err
	}
	sk := doc.Sketch
	for i := range sk.Groups {
		if g := &sk.Groups[i]; g.Type == sketch.GroupLinked && g.LinkFile != "" {
			g.LinkFile = link.Path(model, g.LinkFile)
		}
	}
	for i := range sk.Requests {
		if r := &sk.Requests[i]; r.Type == sketch.RequestImage && r.File != "" {
			r.File = link.Path(model, r.File)
		}
	}
	return doc, nil
}
