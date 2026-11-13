package main

import (
	"fmt"
	"testing"

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

func BenchmarkFilter170k(b *testing.B) {
	es := bigSet(170000)
	ts := parseQuery("l24a0123", testFields)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(es, ts, maxRows)
	}
}
