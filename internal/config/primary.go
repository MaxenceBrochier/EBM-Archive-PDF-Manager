package config

import (
	"regexp"
	"strings"
)

var (
	coverHeaderRE      = regexp.MustCompile(`(?mi)^[ \t]*Ordre[ \t]+de[ \t]+fabrication[ \t]*:`)
	productionEndRE    = regexp.MustCompile(`(?mi)^[^\r\n]*(?:Operation|Opération)[ \t]*:[^\r\n]*(?:\r?\n|$)`)
	productionTitleRE  = regexp.MustCompile(`(?mi)^[ \t]*Production[ \t]+report\b`)
	paragraphBreakRE   = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)
	metadataWORE       = regexp.MustCompile(`(?mi)^[ \t]*(?:Num[ée]ro[ \t]+de[ \t]+WO|Work[ \t]+Order|WO)[ \t]*(?::|[ \t])`)
	transferQuantityRE = regexp.MustCompile(`(?mi)^[ \t]*Quantit[ée][ \t]+transf[ée]r[ée]e\b[^\r\n]*`)
	headerStartRE      = regexp.MustCompile(`(?i)^[ \t]*(?:Dossier[ \t]+de[ \t]+lot\b|(?:Lot(?:[ \t]+(?:Number|No\.?))?|Batch(?:[ \t]+Number)?|(?:N[°º]|No\.?|Num[ée]ro)[ \t]*(?:(?:de|du)[ \t]+)?lot|Ordre[ \t]+de[ \t]+fabrication|Num[ée]ro[ \t]+de[ \t]+WO|Work[ \t]+Order|WO|OF|Code[ \t]+(?:Produit|article)|Product[ \t]+code|Taille[ \t]+du[ \t]+lot|Quantit[ée][ \t]+(?:du[ \t]+WO|transf[ée]r[ée]e)|(?:Début|Fin)[ \t]+de[ \t]+fabrication)[ \t]*(?::|[ \t]+|$))`)
)

// primaryText écarte les opérations et consommations avant l'application des regex.
func (c *Config) primaryText(text string) string {
	page, _, _ := strings.Cut(text, "\f")
	if start := coverHeaderRE.FindStringIndex(page); start != nil {
		return page[:paragraphEnd(page, start[0])]
	}

	var header string
	if end := productionEndRE.FindStringIndex(page); end != nil {
		header = page[:end[1]]
	} else {
		trimmed := strings.TrimLeft(page, " \t\r\n")
		firstLine, _, _ := strings.Cut(trimmed, "\n")
		if headerStartRE.MatchString(firstLine) {
			header = page[:paragraphEnd(page, len(page)-len(trimmed))]
		}
	}

	owner := c.primaryWO(header)
	emptyIdentity := owner == "" && c.primaryValue("lot", header) == "" && c.primaryValue("code_produit", header) == ""
	source := header
	matchingTable := false
	for _, start := range metadataWORE.FindAllStringIndex(page, -1) {
		line, _, _ := strings.Cut(page[start[0]:], "\n")
		tableWO := c.primaryWO(line)
		if tableWO == "" {
			continue
		}
		table := page[start[0]:paragraphEnd(page, start[0])]
		if emptyIdentity && productionTitleRE.MatchString(header) &&
			!strings.HasPrefix(tableWO, "SIP") && !strings.HasPrefix(tableWO, "CIP") && !strings.HasPrefix(tableWO, "SAN") &&
			c.primaryValue("lot", table) != "" && c.primaryValue("code_produit", table) != "" {
			owner = tableWO
		}
		// Le premier tableau décrit parfois une opération fille, pas le dossier.
		if owner != "" && tableWO == owner {
			source += "\n" + table
			matchingTable = true
		}
		break
	}
	if matchingTable || strings.HasPrefix(owner, "STO") {
		if quantity := transferQuantityRE.FindString(text); quantity != "" {
			source += "\n" + quantity
		}
	}
	return source
}

func paragraphEnd(text string, start int) int {
	if end := paragraphBreakRE.FindStringIndex(text[start:]); end != nil {
		return start + end[0]
	}
	return len(text)
}

func (c *Config) primaryWO(text string) string {
	parts := strings.Fields(strings.ToUpper(c.primaryValue("of", text)))
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimPrefix(parts[0], "WO")
}

func (c *Config) primaryValue(name, text string) string {
	for _, field := range c.Fields {
		if field.Name != name {
			continue
		}
		match := field.re.FindStringSubmatch(text)
		if len(match) < 2 {
			return ""
		}
		return strings.TrimSpace(match[1])
	}
	return ""
}
