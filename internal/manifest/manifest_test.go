package manifest

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestShardFileSanitizesSectionName(t *testing.T) {
	got := ShardFile(`a/b:c*d?e"f<g>h|i`)
	want := "a_b_c_d_e_f_g_h_i.json.gz"
	if got != want {
		t.Fatalf("ShardFile() = %q, want %q", got, want)
	}
}

func exportFixture(t *testing.T, docs []Doc) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private", "VAL", "PDF")
	indexDir := filepath.Join(root, DirName)
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := ShardFile("Batch Report")
	if err := WriteAtomic(filepath.Join(indexDir, file), Shard{
		Version: Version, Section: "Batch Report", Docs: docs,
	}, true); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(filepath.Join(indexDir, IndexFile), Index{
		Version: Version, Sections: []SectionInfo{{Name: "Batch Report", File: file, Docs: len(docs)}},
	}, false); err != nil {
		t.Fatal(err)
	}
	return root, indexDir
}

func TestExportXLSXTemplateAndMetadata(t *testing.T) {
	docs := []Doc{
		{
			Path: "Batch Report/2015/rapport été.pdf",
			Fields: map[string][]string{
				"lot":          {"15ESA016", "15ESA02-01"},
				"of":           {"STO0000128"},
				"code_produit": {"0300050"},
				"taille_lot":   {"1'001.000 kg"},
			},
		},
		{Path: "Batch Report/2015/=SUM(1,2).pdf", Err: "aucun champ trouvé"},
	}
	docs = append(docs, Doc{Path: "Batch Report/2015/autre édition.pdf", Fields: docs[0].Fields})
	root, indexDir := exportFixture(t, docs)
	// Les shards obsolètes non référencés ne font pas partie de l'index.
	if err := WriteAtomic(filepath.Join(indexDir, "obsolete.json.gz"), Shard{
		Version: Version, Docs: docs,
	}, true); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "manifest.xlsx")
	stats, err := ExportXLSX(root, indexDir, output)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (ExportStats{Documents: 3, Errors: 1}) {
		t.Fatalf("stats = %+v, want 3 documents including 1 extraction error", stats)
	}
	book, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"Manifest Template", "Manifest Description", "Manifest Sample"}) {
		t.Fatalf("sheets = %v", got)
	}
	rows, err := book.GetRows("Manifest Sample")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"DOS File", "File Name", "Batch Number", "Work Order", "Product Code", "Batch Size"},
		{`VAL\PDF\Batch Report\2015\rapport été.pdf`, "rapport été.pdf", "15ESA016, 15ESA02-01", "STO0000128", "0300050", "1'001.000 kg"},
		{`VAL\PDF\Batch Report\2015\=SUM(1,2).pdf`, "=SUM(1,2).pdf"},
		{`VAL\PDF\Batch Report\2015\autre édition.pdf`, "autre édition.pdf", "15ESA016, 15ESA02-01", "STO0000128", "0300050", "1'001.000 kg"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	formula, err := book.GetCellFormula("Manifest Sample", "B3")
	if err != nil || formula != "" {
		t.Fatalf("filename treated as a formula: %q, err=%v", formula, err)
	}
	cellType, err := book.GetCellType("Manifest Sample", "E2")
	if err != nil || (cellType != excelize.CellTypeInlineString && cellType != excelize.CellTypeSharedString) {
		t.Fatalf("product code is not stored as text: type=%v, err=%v", cellType, err)
	}

	template, err := excelize.OpenReader(bytes.NewReader(manifestTemplate))
	if err != nil {
		t.Fatal(err)
	}
	defer template.Close()
	for _, sheet := range []string{"Manifest Template", "Manifest Description"} {
		wantRows, err := template.GetRows(sheet)
		if err != nil {
			t.Fatal(err)
		}
		gotRows, err := book.GetRows(sheet)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotRows, wantRows) {
			t.Fatalf("%s content changed", sheet)
		}
		for r, row := range wantRows {
			for c := range row {
				cell, err := excelize.CoordinatesToCellName(c+1, r+1)
				if err != nil {
					t.Fatal(err)
				}
				wantStyle, err := template.GetCellStyle(sheet, cell)
				if err != nil {
					t.Fatal(err)
				}
				gotStyle, err := book.GetCellStyle(sheet, cell)
				if err != nil || gotStyle != wantStyle {
					t.Fatalf("%s!%s style changed: %d, want %d, err=%v", sheet, cell, gotStyle, wantStyle, err)
				}
			}
		}
	}
	for i, column := range []string{"A", "B", "C", "D", "E", "F"} {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			t.Fatal(err)
		}
		wantStyle, err := template.GetCellStyle("Manifest Template", column+"1")
		if err != nil {
			t.Fatal(err)
		}
		gotStyle, err := book.GetCellStyle("Manifest Sample", cell)
		if err != nil || gotStyle != wantStyle {
			t.Fatalf("header %s style = %d, want %d, err=%v", cell, gotStyle, wantStyle, err)
		}
	}
}

func TestExportXLSXPathsStartAtVAL(t *testing.T) {
	for _, tt := range []struct {
		name string
		root string
		path string
		want string
	}{
		{"root-is-VAL", "VAL", "report.pdf", `VAL\report.pdf`},
		{"root-above-VAL", "archive", "VAL/PDF/report.pdf", `VAL\PDF\report.pdf`},
		{"case-insensitive-VAL", "val", "PDF/report.pdf", `VAL\PDF\report.pdf`},
		{"missing-VAL", "validation", "report.pdf", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixtureRoot, indexDir := exportFixture(t, []Doc{{Path: tt.path}})
			root := filepath.Join(t.TempDir(), tt.root)
			output := filepath.Join(fixtureRoot, "manifest.xlsx")
			_, err := ExportXLSX(root, indexDir, output)
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), "VAL") {
					t.Fatalf("expected missing VAL error, got %v", err)
				}
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("partial output remains: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenFile(output)
			if err != nil {
				t.Fatal(err)
			}
			defer book.Close()
			got, err := book.GetCellValue("Manifest Sample", "A2")
			if err != nil || got != tt.want {
				t.Fatalf("DOS File = %q, want %q, err=%v", got, tt.want, err)
			}
		})
	}
}

func TestExportXLSXRejectsInvalidIndexWithoutPartialOutput(t *testing.T) {
	for _, tt := range []struct {
		name string
		docs []Doc
	}{
		{"outside-root", []Doc{{Path: "../outside.pdf"}}},
		{"duplicate-path", []Doc{{Path: "a.pdf"}, {Path: "a.pdf"}}},
		{"oversized-cell", []Doc{{Path: "a.pdf", Fields: map[string][]string{"lot": {strings.Repeat("X", 32768)}}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, indexDir := exportFixture(t, tt.docs)
			output := filepath.Join(root, "manifest.xlsx")
			if _, err := ExportXLSX(root, indexDir, output); err == nil {
				t.Fatal("invalid index accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("partial output remains: %v", err)
			}
		})
	}
}

func TestExportXLSXPreservesExistingOutputAndFailsOnMissingShard(t *testing.T) {
	root, indexDir := exportFixture(t, []Doc{{Path: "a.pdf"}})
	output := filepath.Join(root, "manifest.xlsx")
	if err := os.WriteFile(output, []byte("existing workbook"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportXLSX(root, indexDir, output); err == nil {
		t.Fatal("existing output accepted")
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "existing workbook" {
		t.Fatalf("existing output changed: %q, err=%v", data, err)
	}
	if err := os.Remove(filepath.Join(indexDir, ShardFile("Batch Report"))); err != nil {
		t.Fatal(err)
	}
	missingOutput := filepath.Join(root, "missing.xlsx")
	if _, err := ExportXLSX(root, indexDir, missingOutput); err == nil {
		t.Fatal("missing shard accepted")
	}
	if _, err := os.Stat(missingOutput); !os.IsNotExist(err) {
		t.Fatalf("partial output remains: %v", err)
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
