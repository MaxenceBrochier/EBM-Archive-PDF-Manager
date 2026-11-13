package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ebmarchive/internal/config"
	"ebmarchive/internal/manifest"
)

func TestIsPDF(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		{"document.pdf", true},
		{"document.PDF", true},
		{"~$temporary.pdf", false},
		{"document.txt", false},
	} {
		if got := isPDF(tt.name); got != tt.want {
			t.Errorf("isPDF(%q) = %t, want %t", tt.name, got, tt.want)
		}
	}
}

func TestDiscoverAndScan(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{ExcludeDirs: []string{"System Volume Information", "skip"}}
	paths := []string{
		"root.PDF",
		"~$temporary.pdf",
		"Visible/second.PDF",
		"Visible/nested/third.pdf",
		"Visible/skip/ignored.pdf",
		"Visible/.cache/ignored.pdf",
		".hidden/ignored.pdf",
		"System Volume Information/ignored.pdf",
	}
	for _, rel := range paths {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	o := &opts{root: root}
	sections, err := discover(o, cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantSections := []section{
		{name: manifest.RootSection, dir: root},
		{name: "Visible", dir: filepath.Join(root, "Visible"), recursive: true},
	}
	if !reflect.DeepEqual(sections, wantSections) {
		t.Fatalf("discover() = %#v, want %#v", sections, wantSections)
	}

	got, err := scan(root, sections[1], cfg)
	if err != nil {
		t.Fatal(err)
	}
	var gotPaths []string
	for _, f := range got {
		gotPaths = append(gotPaths, f.rel)
	}
	wantPaths := []string{"Visible/nested/third.pdf", "Visible/second.PDF"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("scan() paths = %#v, want %#v", gotPaths, wantPaths)
	}

	o.only = map[string]bool{"Visible": true}
	sections, err = discover(o, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].name != "Visible" {
		t.Fatalf("discover(-only Visible) = %#v", sections)
	}
}

func TestScanNonRecursiveSection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "root.pdf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "ignored.pdf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := scan(root, section{name: manifest.RootSection, dir: root}, &config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].rel != "root.pdf" {
		t.Fatalf("scan() = %#v, want only root.pdf", got)
	}
}

func TestLockRejectsConcurrentIndexerAndUnlocks(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock(dir); err == nil || !strings.Contains(err.Error(), "déjà en cours") {
		t.Fatalf("second lock error = %v, want already-running error", err)
	}

	unlock()
	unlock()
	nextUnlock, err := lock(dir)
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	nextUnlock()
}

func TestTransientAndFirstLine(t *testing.T) {
	for _, tt := range []struct {
		message string
		want    bool
	}{
		{"timeout après 60s", true},
		{"pdftotext : accès refusé", true},
		{"aucun texte", false},
		{"", false},
	} {
		if got := transient(tt.message); got != tt.want {
			t.Errorf("transient(%q) = %t, want %t", tt.message, got, tt.want)
		}
	}
	if got := firstLine("first\nsecond"); got != "first" {
		t.Errorf("firstLine() = %q, want first", got)
	}
	if got := firstLine("single line"); got != "single line" {
		t.Errorf("firstLine() = %q, want unchanged text", got)
	}
}

func TestRunWritesIndexAndRecordsExtractionError(t *testing.T) {
	root := t.TempDir()
	pdf := filepath.Join(root, "Visible", "document.pdf")
	if err := os.MkdirAll(filepath.Dir(pdf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdf, []byte("not a real PDF; extraction is stubbed by a missing executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(root, "fields.json")
	if err := os.WriteFile(configPath, []byte(`{
		"fields": [{"name": "lot", "regex": "Lot: ([A-Z0-9]+)"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, hash, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, manifest.DirName)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	o := &opts{
		root:      root,
		out:       out,
		pdftotext: filepath.Join(root, "missing-pdftotext"),
		workers:   1,
		timeout:   time.Second,
	}
	if err := run(o, cfg, hash); err != nil {
		t.Fatal(err)
	}

	idx, err := manifest.ReadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Sections) != 1 || idx.Sections[0].Name != "Visible" || idx.Sections[0].Errors != 1 {
		t.Fatalf("index sections = %#v, want one failed Visible document", idx.Sections)
	}
	shard, err := manifest.ReadShard(filepath.Join(out, idx.Sections[0].File))
	if err != nil {
		t.Fatal(err)
	}
	if len(shard.Docs) != 1 || !strings.HasPrefix(shard.Docs[0].Err, "pdftotext :") {
		t.Fatalf("shard docs = %#v, want one pdftotext error", shard.Docs)
	}
	errorsFile, err := os.ReadFile(filepath.Join(out, "errors.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(errorsFile), "Visible/document.pdf\tpdftotext :") {
		t.Fatalf("errors.txt = %q, want the extraction failure", errorsFile)
	}
}
