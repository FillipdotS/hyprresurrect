package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var baseTime = time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)

func testSnapshot(at time.Time, windows int) Snapshot {
	snap := Snapshot{
		Version:    Version,
		CapturedAt: at,
		Monitors:   []Monitor{{Name: "DP-1", ActiveWorkspace: 1}},
	}

	for i := range windows {
		snap.Windows = append(snap.Windows, Window{
			Class:     "foot",
			Workspace: i + 1,
			Monitor:   "DP-1",
			At:        [2]int{i, i},
			Size:      [2]int{800, 600},
			Command:   []string{"foot"},
		})
	}

	return snap
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func saveAll(t *testing.T, s Store, times ...time.Time) {
	t.Helper()

	for _, at := range times {
		if _, err := s.Save(testSnapshot(at, 1)); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := NewAt(t.TempDir())
	want := testSnapshot(baseTime, 2)

	path, err := s.Save(want)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := s.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestStoreSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)

	path, err := s.Save(testSnapshot(baseTime, 1))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	if len(names) != 1 {
		t.Fatalf("store holds %v after one Save, want only %s", names, filepath.Base(path))
	}
	if got := filepath.Join(dir, names[0]); got != path {
		t.Errorf("Save() = %q, but the file on disk is %q", path, got)
	}
}

func TestStoreSavePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), appDir)
	s := NewAt(dir)

	path, err := s.Save(testSnapshot(baseTime, 1))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got := di.Mode().Perm(); got != dirPerm {
		t.Errorf("directory mode = %#o, want %#o", got, dirPerm)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if got := fi.Mode().Perm(); got != filePerm {
		t.Errorf("file mode = %#o, want %#o", got, filePerm)
	}
}

func TestStoreListNewestFirst(t *testing.T) {
	s := NewAt(t.TempDir())

	// Saved out of order, so passing this needs a sort rather than save order.
	for _, tc := range []struct {
		at      time.Time
		windows int
	}{
		{baseTime.Add(time.Minute), 2},
		{baseTime, 1},
		{baseTime.Add(2 * time.Minute), 3},
	} {
		if _, err := s.Save(testSnapshot(tc.at, tc.windows)); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	want := []Entry{
		{CapturedAt: baseTime.Add(2 * time.Minute), Windows: 3},
		{CapturedAt: baseTime.Add(time.Minute), Windows: 2},
		{CapturedAt: baseTime, Windows: 1},
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(Entry{}, "Path")); diff != "" {
		t.Errorf("List() mismatch (-want +got):\n%s", diff)
	}

	for i, e := range got {
		if _, err := os.Stat(e.Path); err != nil {
			t.Errorf("entry %d: Stat(%q) error = %v", i, e.Path, err)
		}
	}
}

func TestStoreListIgnoresStrayFiles(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)

	saveAll(t, s, baseTime)

	writeFile(t, filepath.Join(dir, "README"), "not a snapshot")
	writeFile(t, filepath.Join(dir, ".2026-08-31T10-00-00Z.json.tmp"), "{")
	if err := os.Mkdir(filepath.Join(dir, "nested"+fileExt), dirPerm); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(got))
	}
	if !got[0].CapturedAt.Equal(baseTime) {
		t.Errorf("CapturedAt = %v, want %v", got[0].CapturedAt, baseTime)
	}
}

func TestStoreListMissingDirNoError(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir + "dontexist")

	got, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List() returned %d entries, want 0", len(got))
	}
}

func TestStoreLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	s := NewAt(dir)

	path := filepath.Join(dir, baseTime.Format(fileTimeLayout)+fileExt)
	writeFile(t, path, `{"version":1,"windows":[{"class":`)

	if _, err := s.Load(path); err == nil {
		t.Error("Load() error = nil, want an error for truncated json")
	}
}

func TestStorePruneKeepsNewest(t *testing.T) {
	s := NewAt(t.TempDir())

	var times []time.Time
	for i := range 6 {
		times = append(times, baseTime.Add(time.Duration(i)*time.Minute))
	}
	saveAll(t, s, times...)

	if err := s.Prune(5); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("List() returned %d entries after Prune(5), want 5", len(got))
	}
	if oldest := got[len(got)-1].CapturedAt; !oldest.Equal(times[1]) {
		t.Errorf("oldest surviving snapshot = %v, want %v", oldest, times[1])
	}
}

func TestStorePruneKeepsEverything(t *testing.T) {
	times := []time.Time{
		baseTime,
		baseTime.Add(time.Minute),
		baseTime.Add(2 * time.Minute),
	}

	tests := []struct {
		name    string
		keep    int
		wantErr bool
	}{
		{name: "fewer snapshots than keep", keep: 5},
		{name: "exactly keep", keep: 3},
		{name: "zero is refused", keep: 0, wantErr: true},
		{name: "negative is refused", keep: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewAt(t.TempDir())
			saveAll(t, s, times...)

			err := s.Prune(tt.keep)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Prune(%d) error = %v, wantErr %v", tt.keep, err, tt.wantErr)
			}

			got, err := s.List()
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(got) != len(times) {
				t.Errorf("List() returned %d entries, want %d", len(got), len(times))
			}
		})
	}
}
