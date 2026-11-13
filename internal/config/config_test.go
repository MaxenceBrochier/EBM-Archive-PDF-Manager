package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestVALFieldsAreScopedAndExtracted(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}

	text := `N° de lot : LOT-123
Ordre de fabrication : WO3382633 / 0
Lot : 17CU004
Lot : 17DO2X20007
Taille du Lot : 1'000 kg
Code Produit : 0302359
Début de Fabrication : 07.02.2017
Fin de Fabrication : 07.02.2017`

	got := cfg.ExtractSection(text, "val")
	want := map[string][]string{
		"lot":               {"LOT-123", "17CU004", "17DO2X20007"},
		"of":                {"WO3382633 / 0"},
		"taille_lot":        {"1'000 kg"},
		"code_produit":      {"0302359"},
		"debut_fabrication": {"07.02.2017"},
		"fin_fabrication":   {"07.02.2017"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractSection(VAL) = %#v, want %#v", got, want)
	}
}

func TestVALTIFFields(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}

	text := `Ordre de fabrication : TIF000446 / 0
Lot : TIF000446    Taille du Lot : 1 un
Code Produit : GEN_TIF
Début de Fabrication : 12.07.2023
Fin de Fabrication : 12.07.2023`

	got := cfg.ExtractSection(text, filepath.Join("root", "VAL", "PDF", "2023"))
	want := map[string][]string{
		"of":                {"TIF000446 / 0"},
		"lot":               {"TIF000446"},
		"code_produit":      {"GEN_TIF"},
		"debut_fabrication": {"12.07.2023"},
		"fin_fabrication":   {"12.07.2023"},
		"taille_lot":        {"1 un"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractSection(TIF VAL) = %#v, want %#v", got, want)
	}
}

func TestVALBatchSizeLitres(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		text string
		want []string
	}{
		{"Taille du Lot : 2'607 l", []string{"2'607 l"}},
		{"Taille du Lot : 17 L", []string{"17 L"}},
		{"Taille du Lot : 1'000 ml", []string{"1'000 ml"}},
		{"Taille du Lot : 5 ku", []string{"5 ku"}},
		{"Taille du Lot : 17 litre", nil},
	} {
		got := cfg.ExtractSection(tt.text, "VAL")["taille_lot"]
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestVALFieldsAreNotExtractedInOtherSections(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Le champ "lot" est global : il attrape aussi les lignes "Lot :" partout.
	got := cfg.ExtractSection("Lot : 17CU004\nCode Produit : 0302359", filepath.Join("root", "AUTRE", "2023"))
	want := map[string][]string{"lot": {"17CU004"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractSection(AUTRE) = %#v, want %#v", got, want)
	}
}
