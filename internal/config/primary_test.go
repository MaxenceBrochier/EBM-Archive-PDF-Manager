package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrimaryHeaderExcludesAssociatedIdentifiers(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxMatches = 10
	for _, tt := range []struct {
		name string
		text string
		want map[string][]string
	}{
		{
			name: "production-with-child-operation-and-consumptions",
			text: `Production report - V 2.05

Product code: 0306091                       PI: NE_VWF_M_VWF
Description: LNA/R-VWF                      WO: WO3399548
Lot: 17TSTCHU17                            Operation: TMC (Rapport intermédiaire)

TMC
Numéro de WO                               339954802
Code article                               9603671
Quantité du WO                             1 un

Sélection poche cFWA
N° de lot                                  17FWA006

Contrôle des flacons
Numéro de lot                              17A18-11
WO                                         SIP0000102`,
			want: map[string][]string{
				"lot": {"17TSTCHU17"}, "of": {"WO3399548"}, "code_produit": {"0306091"},
			},
		},
		{
			name: "same-owner-table-provides-batch-size",
			text: `Product code: 0300054
Description: BUFFER                        WO: WO3399985
Lot: 17AR1X0005                            Operation: BUFFER

Préparation buffer
Numéro de WO                               WO3399985
Numéro de lot                              17AR1X0005
Code article                               0300054
Quantité du WO                             1'025 l

Consommations
Numéro de lot                              16K17-30
Code article                               0200348
WO                                         SIP0000102`,
			want: map[string][]string{
				"lot": {"17AR1X0005"}, "of": {"WO3399985"}, "code_produit": {"0300054"}, "taille_lot": {"1'025 l"},
			},
		},
		{
			name: "legacy-primary-table-and-sip",
			text: `Dossier de lot                            OF : 104339701 0

Texte Opératoire : BATCH REPORT

MAB Effluent
Numéro de WO                               104339701
Numéro de lot                              15NEV002301
Code article                               9603669
Quantité du WO                             1 un

Check SIP cuve
WO                                         SIP0000270
Numéro de lot                              FOREIGN123`,
			want: map[string][]string{
				"lot": {"15NEV002301"}, "of": {"104339701"}, "code_produit": {"9603669"}, "taille_lot": {"1 un"},
			},
		},
		{
			name: "storage-keeps-transfer-quantity-not-associated-lots",
			text: `Dossier de lot                            OF : STO0000128 0

Storage tank

WO                                         STO0000128
OP TRANSFER IN/OUT démarrée                 23.07.2015 17:02` + "\f" + `
N° lot de la préparation buffer             15ESA016
Numéro de lot de solution dans la cuve de stockage 15ESA02-01
Quantité transférée                         1'001.000 kg

Check SIP cuve
WO                                         SIP0000270`,
			want: map[string][]string{"of": {"STO0000128"}, "taille_lot": {"1'001.000 kg"}},
		},
		{
			name: "empty-production-header-with-complete-primary-table",
			text: `Production report - V 2.03

Operation: MAB Collecte (Rapport intermédiaire)

MAB Effluent
Numéro de WO                               OF00000030
Numéro de lot                              FOR000031
Code article                               9603669
Quantité du WO                             1 un

Check SIP cuve
WO                                         SIP0000096
Numéro de lot                              OTHER123`,
			want: map[string][]string{
				"of": {"OF00000030"}, "lot": {"FOR000031"}, "code_produit": {"9603669"}, "taille_lot": {"1 un"},
			},
		},
		{
			name: "partial-parent-header-does-not-adopt-child-wo",
			text: `Production report - V 2.03
Product code: 0306091
Lot: 17NEV010                  Operation: MAB Collecte

Numéro de WO                               338270201
Numéro de lot                              17NEV01001
Code article                               9603669
Quantité du WO                             1 un`,
			want: map[string][]string{"lot": {"17NEV010"}, "code_produit": {"0306091"}},
		},
		{
			name: "empty-header-does-not-adopt-technical-table",
			text: `Production report - V 2.03
Operation: MAB Collecte

Numéro de WO                               SIP0000096
Numéro de lot                              OTHER123
Code article                               0200010
Quantité du WO                             1 un`,
			want: map[string][]string{},
		},
		{
			name: "missing-header-lot-is-not-a-consumed-lot",
			text: `Product code: 0306091
Description: LNA/R-VWF                      WO: WO3399548
Lot:
17TSTCHU17
Operation: TMC

Numéro de WO                               339954802

Consommations
Numéro de lot                              17FWA006`,
			want: map[string][]string{"of": {"WO3399548"}, "code_produit": {"0306091"}},
		},
		{
			name: "cover-header-before-second-page",
			text: `DOSSIER DE LOT                            OF : WO3382633 0

Ordre de fabrication : WO3382633 / 0
Lot : 17CU004               Taille du Lot : 70 kg
Code Produit : 0302359
Début de Fabrication : 07.02.2017
Fin de Fabrication : 07.02.2017

Consommations
Numéro de lot : FOREIGN1` + "\f" + `Lot : FOREIGN2
WO : SIP0000102`,
			want: map[string][]string{
				"of": {"WO3382633"}, "lot": {"17CU004"}, "code_produit": {"0302359"}, "taille_lot": {"70 kg"},
				"debut_fabrication": {"07.02.2017"}, "fin_fabrication": {"07.02.2017"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ExtractSection(tt.text, "VAL")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractSection() = %#v, want %#v", got, tt.want)
			}
			got = cfg.ExtractSection(strings.ReplaceAll(tt.text, "\n", "\r\n"), "VAL")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractSection(CRLF) = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGenericExtractionCanStillKeepMultipleValues(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.PrimaryOnly = false
	cfg.MaxMatches = 10
	got := cfg.ExtractSection("Lot: L1\nLot: L2", "VAL")["lot"]
	if !reflect.DeepEqual(got, []string{"L1", "L2"}) {
		t.Fatalf("generic extraction = %v, want [L1 L2]", got)
	}
}

func TestPrimaryLotRequiresAnAlignedLabel(t *testing.T) {
	cfg, _, err := Load(filepath.Join("..", "..", "fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Lot:", "Lot Number:", "Lot No.:", "Batch Number", "N° de lot :", "Nº lot", "No. de lot", "Numéro de lot", "Numero du lot"} {
		text := "\r\n\t" + label + "\t17abc-001\r\n\r\nConsommations\r\nNuméro de lot OTHER123"
		got := cfg.ExtractSection(text, "VAL")["lot"]
		if !reflect.DeepEqual(got, []string{"17ABC-001"}) {
			t.Errorf("%q: got %v, want 17ABC-001", label, got)
		}
	}
	for _, text := range []string{
		"17ABC001",
		"17ABC001 Lot Number",
		"Lot Number:\n17ABC001",
		"Lot Number:\r\n17ABC001",
		"Lot Number: \t\n17ABC001",
		"Lot Number: 17ABC.001",
		"Lot Number: UNKNOWN",
		"N° lot de la préparation buffer 17ABC001",
		"Consommations\nNuméro de lot 17ABC001",
	} {
		if got := cfg.ExtractSection(text, "VAL")["lot"]; len(got) != 0 {
			t.Errorf("%q: unexpected lot %v", text, got)
		}
	}
}
