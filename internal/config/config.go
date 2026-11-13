// Package config charge la définition des champs à extraire (regex).
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// extractorVersion : à incrémenter si la logique d'extraction change,
// pour forcer la réindexation complète.
const extractorVersion = 3

type FieldSpec struct {
	Name      string   `json:"name"`               // clé courte stockée dans le manifest
	Label     string   `json:"label"`              // en-tête de colonne dans le finder
	Regex     string   `json:"regex"`              // RE2 ; groupe 1 = valeur, sinon match entier
	Uppercase bool     `json:"uppercase"`          // normaliser la valeur en majuscules
	Sections  []string `json:"sections,omitempty"` // segments de chemin auxquels le champ s'applique ; vide = tous

	re *regexp.Regexp
}

type Config struct {
	MaxPages    int         `json:"max_pages"`    // pages lues par PDF (0 = toutes)
	MaxMatches  int         `json:"max_matches"`  // valeurs max gardées par champ
	ExcludeDirs []string    `json:"exclude_dirs"` // noms de dossiers ignorés
	Fields      []FieldSpec `json:"fields"`
}

// Load lit et valide le fichier ; renvoie aussi une empreinte qui change
// dès que la config change (=> réindexation des documents).
func Load(path string) (*Config, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	if len(c.Fields) == 0 {
		return nil, "", fmt.Errorf("%s: aucun champ défini", path)
	}
	if c.MaxMatches <= 0 {
		c.MaxMatches = 10
	}
	seen := map[string]bool{}
	for i := range c.Fields {
		f := &c.Fields[i]
		if f.Name == "" || f.Regex == "" {
			return nil, "", fmt.Errorf("champ #%d : name et regex obligatoires", i+1)
		}
		if seen[f.Name] {
			return nil, "", fmt.Errorf("champ %q défini deux fois", f.Name)
		}
		seen[f.Name] = true
		if f.Label == "" {
			f.Label = f.Name
		}
		f.re, err = regexp.Compile(f.Regex)
		if err != nil {
			return nil, "", fmt.Errorf("champ %q : regex invalide : %w", f.Name, err)
		}
	}
	norm, _ := json.Marshal(struct {
		V int
		C Config
	}{extractorVersion, c})
	sum := sha256.Sum256(norm)
	return &c, hex.EncodeToString(sum[:8]), nil
}

// Extract applique toutes les regex au texte et renvoie les valeurs
// uniques trouvées, dans l'ordre d'apparition.
func (c *Config) Extract(text string) map[string][]string {
	return c.ExtractSection(text, "")
}

// ExtractSection applique les champs globaux et ceux associés à un segment du chemin.
// Une section vide conserve le comportement historique et inclut tous les champs.
func (c *Config) ExtractSection(text, section string) map[string][]string {
	out := map[string][]string{}
	for _, f := range c.Fields {
		if section != "" && len(f.Sections) > 0 && !containsPathSegment(f.Sections, section) {
			continue
		}
		var vals []string
		seen := map[string]bool{}
		for _, m := range f.re.FindAllStringSubmatch(text, -1) {
			v := m[0]
			if len(m) > 1 {
				v = m[1]
			}
			v = strings.TrimSpace(v)
			if f.Uppercase {
				v = strings.ToUpper(v)
			}
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			vals = append(vals, v)
			if len(vals) >= c.MaxMatches {
				break
			}
		}
		if len(vals) > 0 {
			out[f.Name] = vals
		}
	}
	return out
}

func containsPathSegment(values []string, path string) bool {
	segments := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, v := range values {
		for _, segment := range segments {
			if strings.EqualFold(v, segment) {
				return true
			}
		}
	}
	return false
}
