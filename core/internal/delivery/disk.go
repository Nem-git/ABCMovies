package delivery

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// DiskSink writes a download session's bytes to the instance-local disk
// (PLAN.md §6.4, TECHNICAL-DECISIONS.md §1.13). Deliverables are user-requested
// downloads owned by the sink — not §2.4 caches — so they land under the
// configured root and are never evicted by the cache-store machinery.
//
// Delivery goes to a hidden partial file and Finalize renames it to the final
// output name, so an interrupted or failed run never leaves a half-written
// deliverable under its final name (Abort discards the partial). The final
// name follows the output naming contract's shape (§1.15): a sanitized base
// plus the container extension.
type DiskSink struct {
	root string
	path string // final output path (deduped at Finalize)
	tmp  string // partial file written before finalize
	file *os.File
}

var _ Sink = (*DiskSink)(nil)

// DiskFactory builds a DiskSink for every delivery session, rooted at root.
type DiskFactory struct {
	Root string
}

var _ SinkFactory = (*DiskFactory)(nil)

// NewDiskFactory validates the root directory and returns a sink factory that
// writes there. The root is created if missing.
func NewDiskFactory(root string) (*DiskFactory, error) {
	if root == "" {
		return nil, fmt.Errorf("disk sink: root path is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("disk sink: mkdir %s: %w", root, err)
	}
	return &DiskFactory{Root: root}, nil
}

// NewSink prepares a DiskSink for the session, opening a partial file under
// the factory root. The final output name derives from the session's selected
// target and requested container, sanitized per §1.15.
func (f *DiskFactory) NewSink(_ context.Context, s *Session, _ []*corev1.Track) (Sink, error) {
	finalName := outputName(s, s.Context.GetContainer())
	path := filepath.Join(f.Root, finalName)
	tmp := path + "." + s.ID + ".part"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("disk sink: create partial %s: %w", tmp, err)
	}
	return &DiskSink{root: f.Root, path: path, tmp: tmp, file: file}, nil
}

// Deliver writes one track's bytes to the partial file. A download writes
// each selected track's bytes in delivery order.
func (d *DiskSink) Deliver(_ context.Context, _ *Session, _ *corev1.Track, body io.Reader) (int64, error) {
	if d.file == nil {
		return 0, fmt.Errorf("disk sink: delivery after finalize")
	}
	n, err := io.Copy(d.file, body)
	if err != nil {
		return n, fmt.Errorf("disk sink: write: %w", err)
	}
	return n, nil
}

// Finalize flushes and renames the partial to its final output path, deduping
// a collision with a numeric suffix per §1.15.
func (d *DiskSink) Finalize(_ context.Context, _ *Session) error {
	if d.file == nil {
		return fmt.Errorf("disk sink: finalize after finalize")
	}
	if err := d.file.Sync(); err != nil {
		_ = d.file.Close()
		return fmt.Errorf("disk sink: sync: %w", err)
	}
	if err := d.file.Close(); err != nil {
		d.file = nil
		return fmt.Errorf("disk sink: close: %w", err)
	}
	d.file = nil

	final := d.path
	if _, err := os.Stat(final); err == nil {
		final = dedupePath(final)
	}
	if err := os.Rename(d.tmp, final); err != nil {
		return fmt.Errorf("disk sink: rename to %s: %w", final, err)
	}
	return nil
}

// Abort discards the partial deliverable.
func (d *DiskSink) Abort(_ context.Context, _ *Session) {
	if d.file != nil {
		_ = d.file.Close()
		d.file = nil
	}
	_ = os.Remove(d.tmp)
}

// outputName builds a sanitized base + container extension for a session's
// deliverable (§1.15). The base is the session's selected target; the
// extension is the requested container (or none, when absent).
func outputName(s *Session, container string) string {
	base := sanitizeName(s.Context.GetSelectedTarget())
	if base == "" {
		base = s.ID
	}
	if container == "" {
		return base
	}
	ext := strings.TrimPrefix(strings.ToLower(container), ".")
	return base + "." + ext
}

// dedupePath appends a ` (2)`, ` (3)`, ... suffix to path when it exists,
// matching the collision rule of §1.15.
func dedupePath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}
