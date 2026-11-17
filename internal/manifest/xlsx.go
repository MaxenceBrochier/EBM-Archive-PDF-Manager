package manifest

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

//go:embed manifest_template.xlsx
var manifestTemplate []byte

type ExportStats struct {
	Documents int
	Errors    int
}

// ExportXLSX exporte l'index existant, avec une ligne par PDF, sans relire les PDF.
func ExportXLSX(root, indexDir, output string) (stats ExportStats, err error) {
	root, err = filepath.Abs(root)
	if err != nil {
		return stats, err
	}
	if !strings.EqualFold(filepath.Ext(output), ".xlsx") {
		return stats, fmt.Errorf("le fichier de sortie doit avoir l'extension .xlsx")
	}
	if _, err := os.Lstat(output); err == nil {
		return stats, fmt.Errorf("le fichier de sortie existe déjà : %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return stats, err
	}
	idx, err := ReadIndex(indexDir)
	if err != nil {
		return stats, err
	}

	book, err := excelize.OpenReader(bytes.NewReader(manifestTemplate))
	if err != nil {
		return stats, err
	}
	defer func() { err = errors.Join(err, book.Close()) }()
	const sheet = "Manifest Sample"
	const templateSheet = "Manifest Template"
	headerHeight, err := book.GetRowHeight(templateSheet, 1)
	if err != nil {
		return stats, err
	}
	templateColumns := []string{"A", "B", "C", "D", "E", "F"}
	headers := []string{"DOS File", "File Name", "Batch Number", "Work Order", "Product Code", "Batch Size"}
	headerCells := make([]interface{}, len(headers))
	bodyStyles := make([]int, len(headers))
	widths := make([]float64, len(headers))
	for i, column := range templateColumns {
		style, err := book.GetCellStyle(templateSheet, column+"1")
		if err != nil {
			return stats, err
		}
		headerCells[i] = excelize.Cell{StyleID: style, Value: headers[i]}
		bodyStyles[i], err = book.GetCellStyle("Manifest Description", column+"3")
		if err != nil {
			return stats, err
		}
		widths[i], err = book.GetColWidth(templateSheet, column)
		if err != nil {
			return stats, err
		}
	}
	writer, err := book.NewStreamWriter(sheet)
	if err != nil {
		return stats, err
	}
	for i, width := range widths {
		if err := writer.SetColWidth(i+1, i+1, width); err != nil {
			return stats, err
		}
	}
	if err := writer.SetRow("A1", headerCells, excelize.RowOpts{Height: headerHeight}); err != nil {
		return stats, err
	}

	seen := map[string]bool{}
	for _, section := range idx.Sections {
		if !filepath.IsLocal(section.File) {
			return stats, fmt.Errorf("chemin de section invalide : %q", section.File)
		}
		shard, err := ReadShard(filepath.Join(indexDir, section.File))
		if err != nil {
			return stats, err
		}
		for _, doc := range shard.Docs {
			rel := filepath.FromSlash(doc.Path)
			if !filepath.IsLocal(rel) || !strings.EqualFold(filepath.Ext(rel), ".pdf") {
				return stats, fmt.Errorf("chemin de PDF invalide : %q", doc.Path)
			}
			key := strings.ToLower(filepath.Clean(rel))
			if seen[key] {
				return stats, fmt.Errorf("chemin de PDF répété dans l'index : %s", doc.Path)
			}
			seen[key] = true
			dosPath, err := dosFilePath(filepath.Join(root, rel))
			if err != nil {
				return stats, err
			}
			values := []string{
				dosPath,
				filepath.Base(rel),
				strings.Join(doc.Fields["lot"], ", "),
				strings.Join(doc.Fields["of"], ", "),
				strings.Join(doc.Fields["code_produit"], ", "),
				strings.Join(doc.Fields["taille_lot"], ", "),
			}
			cells := make([]interface{}, len(values))
			for i, value := range values {
				if utf8.RuneCountInString(value) > 32767 {
					return stats, fmt.Errorf("%s : une cellule dépasse la limite Excel de 32767 caractères", doc.Path)
				}
				cells[i] = excelize.Cell{StyleID: bodyStyles[i], Value: value}
			}
			if err := writer.SetRow(fmt.Sprintf("A%d", stats.Documents+2), cells); err != nil {
				return stats, fmt.Errorf("%s : %w", doc.Path, err)
			}
			stats.Documents++
			if doc.Err != "" {
				stats.Errors++
			}
		}
	}
	if err := writer.Flush(); err != nil {
		return stats, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".manifest-*.xlsx")
	if err != nil {
		return stats, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := book.Write(tmp); err != nil {
		return stats, err
	}
	if err := tmp.Sync(); err != nil {
		return stats, err
	}
	if err := tmp.Close(); err != nil {
		return stats, err
	}
	return stats, os.Rename(tmp.Name(), output)
}

func dosFilePath(path string) (string, error) {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if strings.EqualFold(filepath.Base(dir), "VAL") {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return "", err
			}
			return `VAL\` + strings.ReplaceAll(rel, string(filepath.Separator), `\`), nil
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("le PDF n'est pas situé sous un dossier VAL : %s", path)
		}
	}
}
