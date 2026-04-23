package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSourceMeta_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "task.md")
	writeFile(t, md, "---\nname: task\n---\nbody\n")

	src := filepath.Join(dir, "task.source.md")
	writeFile(t, src, "---\nname: task\n---\nbody\n")

	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read src: %v", err)
	}

	meta, err := NewSourceMeta(src, srcBytes)
	if err != nil {
		t.Fatalf("NewSourceMeta: %v", err)
	}
	if meta.SourcePath == "" || !filepath.IsAbs(meta.SourcePath) {
		t.Fatalf("expected absolute source path, got %q", meta.SourcePath)
	}
	if meta.SourceHashSHA256 == "" {
		t.Fatal("expected hash")
	}
	if meta.LastLoadedAt.IsZero() {
		t.Fatal("expected LastLoadedAt set")
	}
	if meta.LastLoadStatus != loadStatusOK {
		t.Fatalf("expected status ok, got %q", meta.LastLoadStatus)
	}

	if err := WriteSourceMeta(md, meta); err != nil {
		t.Fatalf("WriteSourceMeta: %v", err)
	}

	sidecar := SidecarPath(md)
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected sidecar at %s: %v", sidecar, err)
	}

	got, err := ReadSourceMeta(md)
	if err != nil {
		t.Fatalf("ReadSourceMeta: %v", err)
	}
	if got.SourcePath != meta.SourcePath {
		t.Errorf("SourcePath mismatch: got %q want %q", got.SourcePath, meta.SourcePath)
	}
	if got.SourceHashSHA256 != meta.SourceHashSHA256 {
		t.Errorf("hash mismatch")
	}
}

func TestReadSourceMeta_Absent(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "task.md")

	meta, err := ReadSourceMeta(md)
	if err != nil {
		t.Fatalf("expected nil error for absent sidecar, got %v", err)
	}
	if meta != nil {
		t.Fatalf("expected nil meta, got %+v", meta)
	}
}

func TestReadSourceMeta_Malformed(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "task.md")
	writeFile(t, SidecarPath(md), "{not json")

	_, err := ReadSourceMeta(md)
	if err == nil {
		t.Fatal("expected error for malformed sidecar")
	}
}

func TestRemoveSourceMeta_Idempotent(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "task.md")
	if err := RemoveSourceMeta(md); err != nil {
		t.Errorf("absent: expected nil, got %v", err)
	}
	writeFile(t, SidecarPath(md), "{}")
	if err := RemoveSourceMeta(md); err != nil {
		t.Errorf("present: expected nil, got %v", err)
	}
	if _, err := os.Stat(SidecarPath(md)); !os.IsNotExist(err) {
		t.Errorf("expected sidecar removed")
	}
	if err := RemoveSourceMeta(md); err != nil {
		t.Errorf("repeat: expected nil, got %v", err)
	}
}

func TestComputeSyncStatus_Branches(t *testing.T) {
	dir := t.TempDir()

	// no-source
	if got := ComputeSyncStatus(nil); got != SyncStatusNoSource {
		t.Errorf("nil meta: want %q got %q", SyncStatusNoSource, got)
	}
	if got := ComputeSyncStatus(&SourceMeta{}); got != SyncStatusNoSource {
		t.Errorf("empty meta: want %q got %q", SyncStatusNoSource, got)
	}

	src := filepath.Join(dir, "s.md")
	writeFile(t, src, "hello")
	hash, err := HashSource(src)
	if err != nil {
		t.Fatalf("HashSource: %v", err)
	}

	// ok
	meta := &SourceMeta{SourcePath: src, SourceHashSHA256: hash, LastLoadStatus: loadStatusOK}
	if got := ComputeSyncStatus(meta); got != SyncStatusInSync {
		t.Errorf("ok: want %q got %q", SyncStatusInSync, got)
	}

	// drift after edit
	writeFile(t, src, "hello world")
	if got := ComputeSyncStatus(meta); got != SyncStatusDrift {
		t.Errorf("drift: want %q got %q", SyncStatusDrift, got)
	}

	// missing
	if err := os.Remove(src); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := ComputeSyncStatus(meta); got != SyncStatusMissing {
		t.Errorf("missing: want %q got %q", SyncStatusMissing, got)
	}

	// invalid: hash matches stored hash AND status is invalid
	writeFile(t, src, "hello")
	meta2 := &SourceMeta{SourcePath: src, SourceHashSHA256: hash, LastLoadStatus: loadStatusInvalid}
	if got := ComputeSyncStatus(meta2); got != SyncStatusInvalid {
		t.Errorf("invalid: want %q got %q", SyncStatusInvalid, got)
	}
}

func TestNormalizeFrontmatterAliases(t *testing.T) {
	in := []byte("allowed-tools: [a]\nmax-concurrent: 4\nname: task\nenv:\n  allowed-tools: keep-me\n")
	out := string(normalizeFrontmatterAliases(in))
	if !strings.Contains(out, "allowed_tools: [a]") {
		t.Errorf("expected allowed_tools rewrite, got:\n%s", out)
	}
	if !strings.Contains(out, "max_concurrent: 4") {
		t.Errorf("expected max_concurrent rewrite, got:\n%s", out)
	}
	if !strings.Contains(out, "  allowed-tools: keep-me") {
		t.Errorf("expected nested key preserved, got:\n%s", out)
	}
}

func TestReloadAllFromSources_PreservesUUID(t *testing.T) {
	root := t.TempDir()
	p1 := filepath.Join(root, ".anvil", "todos", "p1")
	md := filepath.Join(p1, "hello.md")
	writeFile(t, md, "---\nid: \"task-uuid-123\"\nname: hello\n---\nold body\n")

	src := filepath.Join(root, "source.md")
	writeFile(t, src, "---\nname: hello\n---\nnew body\n")

	srcBytes, _ := os.ReadFile(src)
	// Seed sidecar with a stale hash so reload will rewrite md.
	meta := &SourceMeta{SourcePath: src, SourceHashSHA256: "deadbeef", LastLoadStatus: loadStatusOK}
	if err := WriteSourceMeta(md, meta); err != nil {
		t.Fatalf("WriteSourceMeta: %v", err)
	}

	summary, err := ReloadAllFromSources(root)
	if err != nil {
		t.Fatalf("ReloadAllFromSources: %v", err)
	}
	if summary.Reloaded != 1 {
		t.Errorf("expected 1 reloaded, got %+v", summary)
	}

	newMD, err := os.ReadFile(md)
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	if !strings.Contains(string(newMD), "new body") {
		t.Errorf("expected new body, got:\n%s", newMD)
	}
	if !strings.Contains(string(newMD), `id: "task-uuid-123"`) {
		t.Errorf("expected UUID preserved, got:\n%s", newMD)
	}

	// sidecar hash should now match src
	got, err := ReadSourceMeta(md)
	if err != nil {
		t.Fatalf("ReadSourceMeta: %v", err)
	}
	expected, _ := HashSource(src)
	if got.SourceHashSHA256 != expected {
		t.Errorf("hash not updated: got %q want %q", got.SourceHashSHA256, expected)
	}
	_ = srcBytes
}

func TestReloadAllFromSources_InvalidFrontmatter(t *testing.T) {
	root := t.TempDir()
	p1 := filepath.Join(root, ".anvil", "todos", "p1")
	md := filepath.Join(p1, "broken.md")
	writeFile(t, md, "---\nid: \"uuid-1\"\nname: broken\n---\nbody\n")

	src := filepath.Join(root, "src.md")
	writeFile(t, src, "---\nname: broken\n  : invalid yaml\n---\nbody\n")

	meta := &SourceMeta{SourcePath: src, SourceHashSHA256: "stale", LastLoadStatus: loadStatusOK}
	if err := WriteSourceMeta(md, meta); err != nil {
		t.Fatalf("WriteSourceMeta: %v", err)
	}

	summary, err := ReloadAllFromSources(root)
	if err != nil {
		t.Fatalf("ReloadAllFromSources: %v", err)
	}
	if summary.Invalid != 1 {
		t.Errorf("expected 1 invalid, got %+v", summary)
	}

	got, err := ReadSourceMeta(md)
	if err != nil {
		t.Fatalf("ReadSourceMeta: %v", err)
	}
	if got.LastLoadStatus != loadStatusInvalid {
		t.Errorf("expected status invalid, got %q", got.LastLoadStatus)
	}
	if got.LastLoadError == "" {
		t.Errorf("expected error recorded")
	}
}

func TestReloadAllFromSources_MissingSource(t *testing.T) {
	root := t.TempDir()
	p1 := filepath.Join(root, ".anvil", "todos", "p1")
	md := filepath.Join(p1, "orphan.md")
	writeFile(t, md, "---\nname: orphan\n---\nbody\n")

	meta := &SourceMeta{
		SourcePath:       filepath.Join(root, "does-not-exist.md"),
		SourceHashSHA256: "any",
		LastLoadStatus:   loadStatusOK,
	}
	if err := WriteSourceMeta(md, meta); err != nil {
		t.Fatalf("WriteSourceMeta: %v", err)
	}

	summary, err := ReloadAllFromSources(root)
	if err != nil {
		t.Fatalf("ReloadAllFromSources: %v", err)
	}
	if summary.Missing != 1 {
		t.Errorf("expected 1 missing, got %+v", summary)
	}
}

func TestSetOrInsertIDLine(t *testing.T) {
	// insert: no id line present
	fm := "name: task\n"
	got := setOrInsertIDLine(fm, "abc-123")
	if !strings.Contains(got, `id: "abc-123"`) {
		t.Errorf("missing inserted id: %s", got)
	}
	if !strings.Contains(got, "name: task") {
		t.Errorf("lost existing content: %s", got)
	}

	// replace: existing id line
	fm2 := "id: \"old\"\nname: task\n"
	got2 := setOrInsertIDLine(fm2, "new")
	if strings.Contains(got2, `"old"`) {
		t.Errorf("old id still present: %s", got2)
	}
	if !strings.Contains(got2, `id: "new"`) {
		t.Errorf("missing replacement id: %s", got2)
	}
}

func TestSidecarPath(t *testing.T) {
	got := SidecarPath("/tmp/foo/task.md")
	want := "/tmp/foo/task.meta.json"
	if got != want {
		t.Errorf("SidecarPath: got %q want %q", got, want)
	}
}
