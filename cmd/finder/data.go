package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"ebmarchive/internal/manifest"
)

// entry est un document prêt pour la recherche (chaînes pré-normalisées).
type entry struct {
	doc       manifest.Doc
	section   string
	base, dir string
	cells     []string // valeurs affichées, une par champ
	lowFields []string // valeurs en minuscules, une par champ
	lowAll    string   // tout le cherchable : champs + chemin
}

type tab struct {
	name    string // nom de section, "" pour l'onglet "Tous"
	label   string
	entries []*entry
}

type catalog struct {
	idx  *manifest.Index
	tabs []tab
	docs int
}

func sectionLabel(name string) string {
	if name == manifest.RootSection {
		return "Racine"
	}
	return name
}

func loadCatalog(indexDir string) (*catalog, error) {
	idx, err := manifest.ReadIndex(indexDir)
	if err != nil {
		return nil, fmt.Errorf("lecture du manifest impossible : %w", err)
	}
	shards := make([]*manifest.Shard, len(idx.Sections))
	errs := make([]error, len(idx.Sections))
	var wg sync.WaitGroup
	for i, s := range idx.Sections {
		wg.Add(1)
		go func(i int, s manifest.SectionInfo) {
			defer wg.Done()
			shards[i], errs[i] = manifest.ReadShard(filepath.Join(indexDir, s.File))
		}(i, s)
	}
	wg.Wait()

	c := &catalog{idx: idx}
	all := tab{label: "Tous"}
	for i, s := range idx.Sections {
		if errs[i] != nil {
			return nil, errs[i]
		}
		t := tab{name: s.Name, label: sectionLabel(s.Name)}
		t.entries = make([]*entry, 0, len(shards[i].Docs))
		for _, d := range shards[i].Docs {
			t.entries = append(t.entries, newEntry(d, s.Name, idx.Fields))
		}
		all.entries = append(all.entries, t.entries...)
		c.tabs = append(c.tabs, t)
		c.docs += len(t.entries)
	}
	c.tabs = append([]tab{all}, c.tabs...)
	return c, nil
}

func newEntry(d manifest.Doc, section string, fields []manifest.Field) *entry {
	e := &entry{doc: d, section: section, base: path.Base(d.Path), dir: path.Dir(d.Path)}
	if e.dir == "." {
		e.dir = ""
	}
	var all strings.Builder
	for _, f := range fields {
		v := strings.Join(d.Fields[f.Name], ", ")
		e.cells = append(e.cells, v)
		lv := strings.ToLower(v)
		e.lowFields = append(e.lowFields, lv)
		all.WriteString(lv)
		all.WriteByte(' ')
	}
	all.WriteString(strings.ToLower(d.Path))
	e.lowAll = all.String()
	return e
}

// term : un mot de la recherche, éventuellement restreint à un champ
// avec la syntaxe "lot:L24".
type term struct {
	field int // -1 = partout
	text  string
}

func parseQuery(q string, fields []manifest.Field) []term {
	var ts []term
	for _, w := range strings.Fields(strings.ToLower(q)) {
		t := term{field: -1, text: w}
		if k, v, ok := strings.Cut(w, ":"); ok && v != "" {
			for i, f := range fields {
				if k == strings.ToLower(f.Name) || k == strings.ToLower(f.Label) {
					t = term{field: i, text: v}
					break
				}
			}
		}
		ts = append(ts, t)
	}
	return ts
}

func (e *entry) match(ts []term) bool {
	for _, t := range ts {
		hay := e.lowAll
		if t.field >= 0 {
			hay = e.lowFields[t.field]
		}
		if !strings.Contains(hay, t.text) {
			return false
		}
	}
	return true
}

// filter renvoie au plus limit résultats, et le nombre total de correspondances.
// Les correspondances exactes sur un champ passent en tête.
func filter(entries []*entry, ts []term, limit int) ([]*entry, int) {
	if len(ts) == 0 {
		n := min(limit, len(entries))
		return entries[:n], len(entries)
	}
	var exact, other []*entry
	total := 0
	for _, e := range entries {
		if !e.match(ts) {
			continue
		}
		total++
		if len(exact)+len(other) >= limit*2 {
			continue // on compte, mais on ne stocke plus
		}
		if e.exact(ts[0].text) {
			exact = append(exact, e)
		} else {
			other = append(other, e)
		}
	}
	res := append(exact, other...)
	if len(res) > limit {
		res = res[:limit]
	}
	return res, total
}

func (e *entry) exact(s string) bool {
	for _, lv := range e.lowFields {
		for _, v := range strings.Split(lv, ", ") {
			if v == s {
				return true
			}
		}
	}
	return false
}

// findRoot : -root, sinon le dossier de l'exe (cas normal : exe posé à la
// racine du share), sinon le dossier courant.
func findRoot(flagRoot string) (string, error) {
	cands := []string{}
	if flagRoot != "" {
		cands = append(cands, flagRoot)
	} else {
		if exe, err := os.Executable(); err == nil {
			cands = append(cands, filepath.Dir(exe))
		}
		if wd, err := os.Getwd(); err == nil {
			cands = append(cands, wd)
		}
	}
	for _, c := range cands {
		if _, err := os.Stat(filepath.Join(c, manifest.DirName, manifest.IndexFile)); err == nil {
			return filepath.Abs(c)
		}
	}
	return "", fmt.Errorf("aucun manifest trouvé (%s\\%s) dans %s", manifest.DirName, manifest.IndexFile, strings.Join(cands, ", "))
}
