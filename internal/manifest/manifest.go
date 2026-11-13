// Package manifest définit le format du manifest d'archive et sa
// lecture/écriture atomique.
//
// Organisation sur le share :
//
//	<root>/.index/index.json            résumé : champs, sections, date
//	<root>/.index/<section>.json.gz     un shard par onglet (sous-dossier)
//
// Les chemins des documents sont relatifs à <root>, séparés par "/",
// pour fonctionner quelle que soit la lettre de lecteur ou le chemin UNC.
package manifest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Version     = 1
	DirName     = ".index"
	IndexFile   = "index.json"
	RootSection = "_racine" // PDF posés directement dans le dossier racine
)

// Field décrit un champ extrait (ordre = ordre des colonnes dans le finder).
type Field struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// Doc est une entrée du manifest. Clés courtes : 170k entrées, ça compte.
type Doc struct {
	Path   string              `json:"p"`           // relatif à root, séparateur "/"
	Size   int64               `json:"s"`           // octets
	MTime  int64               `json:"m"`           // unix secondes
	Fields map[string][]string `json:"f,omitempty"` // nom du champ -> valeurs trouvées
	Err    string              `json:"e,omitempty"` // erreur d'extraction éventuelle
}

// Shard regroupe les documents d'une section (= un onglet).
type Shard struct {
	Version    int       `json:"v"`
	Section    string    `json:"section"`
	Generated  time.Time `json:"generated"`
	ConfigHash string    `json:"config_hash"`
	Docs       []Doc     `json:"docs"`
}

type SectionInfo struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Docs   int    `json:"docs"`
	Errors int    `json:"errors"`
}

// Index est le point d'entrée lu par le finder.
type Index struct {
	Version   int           `json:"v"`
	Generated time.Time     `json:"generated"`
	Fields    []Field       `json:"fields"`
	Sections  []SectionInfo `json:"sections"`
}

// ShardFile renvoie le nom de fichier du shard d'une section.
func ShardFile(section string) string {
	r := strings.NewReplacer(`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_",
		`"`, "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(section) + ".json.gz"
}

// WriteAtomic sérialise v en JSON (gzip si gz) dans un fichier temporaire
// puis le renomme : un lecteur ne voit jamais un fichier à moitié écrit.
func WriteAtomic(path string, v any, gz bool) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op si le rename a réussi

	var w io.Writer = tmp
	var zw *gzip.Writer
	if gz {
		zw, _ = gzip.NewWriterLevel(tmp, gzip.BestCompression)
		w = zw
	}
	enc := json.NewEncoder(w)
	if !gz {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if zw != nil {
		if err := zw.Close(); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmpName, 0o644) // CreateTemp crée en 0600 : illisible pour les clients sur un share Linux
	// Sur un share SMB, le rename peut échouer brièvement si un client
	// est en train de lire l'ancien fichier : on réessaie.
	for i := 0; ; i++ {
		err = os.Rename(tmpName, path)
		if err == nil || i >= 10 {
			return err
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
}

func readJSON(path string, gz bool, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader = f
	if gz {
		zr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		defer zr.Close()
		r = zr
	}
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func ReadShard(path string) (*Shard, error) {
	var s Shard
	if err := readJSON(path, true, &s); err != nil {
		return nil, err
	}
	if s.Version != Version {
		return nil, fmt.Errorf("%s: version %d non supportée", path, s.Version)
	}
	return &s, nil
}

func ReadIndex(dir string) (*Index, error) {
	var idx Index
	if err := readJSON(filepath.Join(dir, IndexFile), false, &idx); err != nil {
		return nil, err
	}
	if idx.Version != Version {
		return nil, fmt.Errorf("index version %d non supportée", idx.Version)
	}
	return &idx, nil
}
