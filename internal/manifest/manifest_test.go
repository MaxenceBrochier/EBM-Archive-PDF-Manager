package manifest

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestShardFileSanitizesSectionName(t *testing.T) {
	got := ShardFile(`a/b:c*d?e"f<g>h|i`)
	want := "a_b_c_d_e_f_g_h_i.json.gz"
	if got != want {
		t.Fatalf("ShardFile() = %q, want %q", got, want)
	}
}

func TestWriteAtomicAndReadShard(t *testing.T) {
	path := filepath.Join(t.TempDir(), ShardFile("VAL/2023"))
	want := &Shard{
		Version:    Version,
		Section:    "VAL/2023",
		Generated:  time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		ConfigHash: "config-v1",
		Docs: []Doc{{
			Path:   "VAL/2023/document.pdf",
			Size:   42,
			MTime:  1763035200,
			Fields: map[string][]string{"lot": {"L24A"}},
		}},
	}

	if err := WriteAtomic(path, want, true); err != nil {
		t.Fatal(err)
	}
	got, err := ReadShard(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadShard() = %#v, want %#v", got, want)
	}

	want.ConfigHash = "config-v2"
	if err := WriteAtomic(path, want, true); err != nil {
		t.Fatal(err)
	}
	got, err = ReadShard(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigHash != want.ConfigHash {
		t.Fatalf("overwritten ConfigHash = %q, want %q", got.ConfigHash, want.ConfigHash)
	}
}

func TestReadShardRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shard.json.gz")
	if err := WriteAtomic(path, Shard{Version: Version + 1}, true); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadShard(path); err == nil {
		t.Fatal("ReadShard() accepted an unsupported version")
	}
}

func TestWriteAtomicAndReadIndex(t *testing.T) {
	dir := t.TempDir()
	want := Index{
		Version:   Version,
		Generated: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		Fields:    []Field{{Name: "lot", Label: "Lot"}},
		Sections:  []SectionInfo{{Name: "VAL", File: "VAL.json.gz", Docs: 2, Errors: 1}},
	}
	if err := WriteAtomic(filepath.Join(dir, IndexFile), want, false); err != nil {
		t.Fatal(err)
	}

	got, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("ReadIndex() = %#v, want %#v", got, want)
	}
}

func TestReadIndexRejectsUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAtomic(filepath.Join(dir, IndexFile), Index{Version: Version + 1}, false); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadIndex(dir); err == nil {
		t.Fatal("ReadIndex() accepted an unsupported version")
	}
}
