package snapshot

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

const (
	appDir = "hyprresurrect"

	fileTimeLayout = "2006-01-02T15-04-05Z"
	fileExt        = ".json"

	dirPerm  = 0o700
	filePerm = 0o600
)

const DefaultKeep = 5

type Store struct {
	dir string
}

func New() (Store, error) {
	if xdg.StateHome == "" {
		return Store{}, errors.New("no XDG state directory; cannot locate snapshots")
	}

	return NewAt(filepath.Join(xdg.StateHome, appDir)), nil
}

func NewAt(dir string) Store {
	return Store{dir: dir}
}

type Entry struct {
	Path       string
	CapturedAt time.Time
	Windows    int
}

// Save writes snap to a new file and returns its path
func (s Store) Save(snap Snapshot) (string, error) {
	if err := os.MkdirAll(s.dir, dirPerm); err != nil {
		return "", err
	}

	t, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	tmp := t.Name()

	defer func() {
		_ = t.Close()
		_ = os.Remove(tmp)
	}()

	if err := t.Chmod(filePerm); err != nil {
		return "", err
	}

	out, err := json.Marshal(snap, jsontext.WithIndent("  "))
	if err != nil {
		return "", err
	}

	if _, err := t.Write(out); err != nil {
		return "", err
	}
	if err := t.Sync(); err != nil {
		return "", err
	}
	if err := t.Close(); err != nil {
		return "", err
	}

	name := snap.CapturedAt.UTC().Format(fileTimeLayout) + fileExt
	path := filepath.Join(s.dir, name)

	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}

	return path, nil
}

// List returns the snapshots in the store, newest first.
func (s Store) List() ([]Entry, error) {
	dir, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var entries []Entry
	for _, de := range dir {
		if !de.Type().IsRegular() {
			continue
		}

		base, ok := strings.CutSuffix(de.Name(), fileExt)
		if !ok {
			continue
		}
		if _, err := time.Parse(fileTimeLayout, base); err != nil {
			continue
		}

		path := filepath.Join(s.dir, de.Name())

		// One unreadable file must not hide the snapshots that are still good.
		snap, err := s.Load(path)
		if err != nil {
			continue
		}

		entries = append(entries, Entry{
			Path:       path,
			CapturedAt: snap.CapturedAt,
			Windows:    len(snap.Windows),
		})
	}

	slices.SortFunc(entries, func(a, b Entry) int {
		return b.CapturedAt.Compare(a.CapturedAt)
	})

	return entries, nil
}

func (s Store) Load(path string) (Snapshot, error) {
	in, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}

	var snap Snapshot
	if err := json.Unmarshal(in, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("%s: %w", path, err)
	}

	if snap.Version != Version {
		return Snapshot{}, fmt.Errorf("%s: snapshot version %d, want %d", path, snap.Version, Version)
	}

	return snap, nil
}

// Prune deletes all but the keep newest snapshots.
func (s Store) Prune(keep int) error {
	if keep < 1 {
		return errors.New("must prune one or more")
	}

	entries, err := s.List()
	if err != nil {
		return err
	}
	if len(entries) <= keep {
		return nil
	}

	var errs []error
	for _, e := range entries[keep:] {
		if err := os.Remove(e.Path); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
