package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ebmarchive/internal/manifest"
)

var testFields = []manifest.Field{{Name: "lot", Label: "N° de lot"}}

func bigSet(n int) []*entry {
	es := make([]*entry, n)
	for i := range es {
		d := manifest.Doc{Path: fmt.Sprintf("data_profess/2026/%02d/EBR-%06d.pdf", i%12, i),
			Fields: map[string][]string{"lot": {fmt.Sprintf("L24A%06d", i)}}}
		es[i] = newEntry(d, "data_profess", testFields)
	}
	return es
}

func TestFilter(t *testing.T) {
	es := bigSet(1000)
	res, total := filter(es, parseQuery("lot:l24a000123", testFields), 50)
	if total != 1 || res[0].cells[0] != "L24A000123" {
		t.Fatalf("got %d", total)
	}
	_, total = filter(es, parseQuery("lot:L24A0001 2026/03", testFields), 50)
	if total == 0 {
		t.Fatal("AND multi-termes")
	}
	// correspondance exacte en tête
	res, _ = filter(es, parseQuery("L24A000010", testFields), 50)
	if res[0].cells[0] != "L24A000010" {
		t.Fatal("exact first")
	}
}

func TestParseQuery(t *testing.T) {
	fields := []manifest.Field{
		{Name: "lot", Label: "Lot"},
		{Name: "product", Label: "Product"},
	}
	got := parseQuery("LOT:L24A product:gen unknown:value lot:", fields)
	want := []term{
		{field: 0, text: "l24a"},
		{field: 1, text: "gen"},
		{field: -1, text: "unknown:value"},
		{field: -1, text: "lot:"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseQuery() = %#v, want %#v", got, want)
	}
}

func TestFilterEmptyQueryHonorsLimit(t *testing.T) {
	es := bigSet(3)
	got, total := filter(es, nil, 2)
	if total != 3 || len(got) != 2 || got[0] != es[0] || got[1] != es[1] {
		t.Fatalf("filter() = (%d entries, total %d), want first 2 entries and total 3", len(got), total)
	}
	got, total = filter(es, nil, 0)
	if total != 3 || len(got) != 0 {
		t.Fatalf("filter(limit 0) = (%d entries, total %d), want no entries and total 3", len(got), total)
	}
}

func TestLoadCatalogReadsAllSections(t *testing.T) {
	dir := t.TempDir()
	fields := []manifest.Field{{Name: "lot", Label: "Lot"}}
	sections := []manifest.SectionInfo{
		{Name: manifest.RootSection, File: manifest.ShardFile(manifest.RootSection), Docs: 1},
		{Name: "VAL", File: manifest.ShardFile("VAL"), Docs: 1},
	}
	idx := manifest.Index{Version: manifest.Version, Fields: fields, Sections: sections}
	if err := manifest.WriteAtomic(filepath.Join(dir, manifest.IndexFile), idx, false); err != nil {
		t.Fatal(err)
	}
	for _, s := range sections {
		doc := manifest.Doc{Path: s.Name + "/document.pdf", Fields: map[string][]string{"lot": {s.Name}}}
		if s.Name == manifest.RootSection {
			doc.Path = "root.pdf"
		}
		shard := manifest.Shard{
			Version: manifest.Version, Section: s.Name, Generated: time.Now(), Docs: []manifest.Doc{doc},
		}
		if err := manifest.WriteAtomic(filepath.Join(dir, s.File), shard, true); err != nil {
			t.Fatal(err)
		}
	}

	got, err := loadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.docs != 2 || len(got.tabs) != 3 {
		t.Fatalf("loadCatalog() = %d docs, %d tabs; want 2 docs, 3 tabs", got.docs, len(got.tabs))
	}
	if got.tabs[0].label != "Tous" || got.tabs[1].label != "Racine" || got.tabs[2].label != "VAL" {
		t.Fatalf("tab labels = %q, %q, %q", got.tabs[0].label, got.tabs[1].label, got.tabs[2].label)
	}
	if got.tabs[1].entries[0].base != "root.pdf" || got.tabs[1].entries[0].cells[0] != manifest.RootSection {
		t.Fatalf("root entry = %#v", got.tabs[1].entries[0])
	}
}

func TestLoadCatalogReturnsShardError(t *testing.T) {
	dir := t.TempDir()
	idx := manifest.Index{
		Version:  manifest.Version,
		Sections: []manifest.SectionInfo{{Name: "VAL", File: "missing.json.gz"}},
	}
	if err := manifest.WriteAtomic(filepath.Join(dir, manifest.IndexFile), idx, false); err != nil {
		t.Fatal(err)
	}

	if _, err := loadCatalog(dir); err == nil {
		t.Fatal("loadCatalog() succeeded with a missing shard")
	}
}

func TestFindRootUsesExplicitRoot(t *testing.T) {
	root := t.TempDir()
	indexDir := filepath.Join(root, manifest.DirName)
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manifest.WriteAtomic(
		filepath.Join(indexDir, manifest.IndexFile),
		manifest.Index{Version: manifest.Version},
		false,
	); err != nil {
		t.Fatal(err)
	}

	got, err := findRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("findRoot() = %q, want %q", got, want)
	}
}

func BenchmarkFilter170k(b *testing.B) {
	es := bigSet(170000)
	ts := parseQuery("l24a0123", testFields)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(es, ts, maxRows)
	}
}
