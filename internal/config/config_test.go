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
		"lot":               {"LOT-123"},
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
		{"Quantité transférée 1'001.000 kg", []string{"1'001.000 kg"}},
		{"Quantité transférée : 106.000 kg", []string{"106.000 kg"}},
		{"Quantité transférée 12,5 l", []string{"12,5 l"}},
		{"Quantité transférée 1.000", nil},
		{"Quantité transférée 123'45 kg", nil},
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
	got := cfg.ExtractSection("Lot : 17CU004\nCode Produit : 0302359\nWO: WO3399985\nProduct code: 0300054\nQuantité du WO 1'025 l", filepath.Join("root", "AUTRE", "2023"))
	want := map[string][]string{"lot": {"17CU004"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractSection(AUTRE) = %#v, want %#v", got, want)
	}
}

func TestBatchReportFields(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		text string
		want map[string][]string
	}{
		{
			name: "legacy",
			text: `Dossier de lot                                      OF : WO1043039 0
Numéro de WO                             WO1043039
Numéro de lot                            15AQ2002
Code article                             0300050
Quantité du WO                           2'400 l`,
			want: map[string][]string{
				"of":           {"WO1043039"},
				"lot":          {"15AQ2002"},
				"code_produit": {"0300050"},
				"taille_lot":   {"2'400 l"},
			},
		},
		{
			name: "intermediate",
			text: `Product code: 0300054                  PI: NE_BUF_M_BUFFERPROCESS - V 25
Description: AR1 MAB REGENERATION BUFFER               WO: WO3399985
Lot: 17AR1X0005                       Operation: BUFFER (Rapport intermédiaire)
WO status: Terminated                          WO review status: Not reviewed
Numéro de WO                             WO3399985
Numéro de lot                            17AR1X0005
Code article                             0300054
Quantité du WO                           1'025 l`,
			want: map[string][]string{
				"of":           {"WO3399985"},
				"lot":          {"17AR1X0005"},
				"code_produit": {"0300054"},
				"taille_lot":   {"1'025 l"},
			},
		},
		{
			name: "storage-without-lot",
			text: `Dossier de lot                         OF : STO0000128 0
WO                                       STO0000128
OP TRANSFER IN/OUT démarrée               23.07.2015 17:02`,
			want: map[string][]string{"of": {"STO0000128"}},
		},
		{
			name: "storage-preparation-and-solution-lots",
			text: `Dossier de lot                         OF : STO0000128 0
N° lot de la préparation buffer           15ESA016
Numéro de lot de solution dans la cuve de stockage    15ESA02-01
Quantité transférée                       1'001.000 kg`,
			want: map[string][]string{
				"of":         {"STO0000128"},
				"taille_lot": {"1'001.000 kg"},
			},
		},
		{
			name: "non-WO-prefix",
			text: `Product code: GEN_STORAGE
Description: CB40_TRANSFER                WO: STO0000126
Lot: STO0000126                           Operation: STORAGE
WO                                       STO0000126`,
			want: map[string][]string{
				"of":           {"STO0000126"},
				"lot":          {"STO0000126"},
				"code_produit": {"GEN_STORAGE"},
			},
		},
		{
			name: "table-WO-only",
			text: "Numéro de WO                CAT000279",
			want: map[string][]string{"of": {"CAT000279"}},
		},
		{
			name: "storage-WO-only",
			text: "WO                          STO0000128",
			want: map[string][]string{"of": {"STO0000128"}},
		},
		{
			name: "unrelated-labels-and-consumptions",
			text: `WO status: Terminated             WO review status: Not reviewed
Numéro de WO SIP                         SIP0000423
Quantité du WO
Code article
WO
OP TRANSFER IN/OUT démarrée               23.07.2015 17:02
Item                Lot                  Quantity
0200348             16K17-30              3'848.903 g`,
			want: map[string][]string{},
		},
	} {
		for _, section := range []string{
			filepath.Join("archive", "Batch Report", "2015"),
			filepath.Join("archive", "Batch Report intermédiaire", "2017"),
			filepath.Join("archive", "VAL", "PDF"),
		} {
			t.Run(tt.name+"/"+section, func(t *testing.T) {
				if got := cfg.ExtractSection(tt.text, section); !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("ExtractSection() = %#v, want %#v", got, tt.want)
				}
			})
		}
	}
}
