// indexer : parcourt l'archive, extrait le texte des PDF avec poppler
// (pdftotext) et écrit le manifest dans <root>/.index.
//
// Incrémental : un PDF dont la taille, la date de modification et la
// config n'ont pas changé n'est pas retraité.
//
//	indexer -root \\srv\archive\EBM
//	indexer -root D:\EBM -workers 16 -force
//	indexer -test D:\EBM\data_test\BR-0001.pdf     (mise au point des regex)
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ebmarchive/internal/config"
	"ebmarchive/internal/manifest"
)

type opts struct {
	root, out, cfgPath, pdftotext string
	workers                       int
	timeout                       time.Duration
	force                         bool
	only                          map[string]bool
}

func main() {
	var o opts
	var only, test string
	flag.StringVar(&o.root, "root", "", "dossier racine de l'archive (obligatoire)")
	flag.StringVar(&o.out, "out", "", "dossier du manifest (défaut : <root>/.index)")
	flag.StringVar(&o.cfgPath, "config", "", "fichier de config des champs (défaut : fields.json à côté de l'exe)")
	flag.StringVar(&o.pdftotext, "pdftotext", "pdftotext", "chemin vers pdftotext(.exe) de poppler")
	flag.IntVar(&o.workers, "workers", runtime.NumCPU(), "nombre d'extractions en parallèle")
	flag.DurationVar(&o.timeout, "timeout", 60*time.Second, "timeout par PDF")
	flag.BoolVar(&o.force, "force", false, "tout réindexer, ignorer le manifest existant")
	flag.StringVar(&only, "only", "", "sections à traiter, séparées par des virgules (ex : data_test)")
	flag.StringVar(&test, "test", "", "affiche le texte extrait d'un PDF et les valeurs trouvées, puis quitte")
	flag.Parse()
	log.SetFlags(log.Ltime)

	if o.cfgPath == "" {
		o.cfgPath = besideExe("fields.json")
	}
	cfg, hash, err := config.Load(o.cfgPath)
	if err != nil {
		log.Fatalf("config : %v", err)
	}
	if test != "" {
		runTest(&o, cfg, test)
		return
	}
	if o.root == "" {
		flag.Usage()
		os.Exit(2)
	}
	if o.root, err = filepath.Abs(o.root); err != nil {
		log.Fatal(err)
	}
	if o.out == "" {
		o.out = filepath.Join(o.root, manifest.DirName)
	}
	if only != "" {
		o.only = map[string]bool{}
		for _, s := range strings.Split(only, ",") {
			o.only[strings.TrimSpace(s)] = true
		}
	}
	if _, err := exec.LookPath(o.pdftotext); err != nil {
		log.Fatalf("pdftotext introuvable (%s) : installe poppler ou passe -pdftotext", o.pdftotext)
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		log.Fatal(err)
	}
	unlock, err := lock(o.out)
	if err != nil {
		log.Fatal(err)
	}
	defer unlock()

	if err := run(&o, cfg, hash); err != nil {
		unlock()
		log.Fatal(err)
	}
}

func besideExe(name string) string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

// lock empêche deux indexations simultanées sur le même manifest.
func lock(dir string) (func(), error) {
	p := filepath.Join(dir, "indexer.lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("une indexation est déjà en cours (%s). Si ce n'est pas le cas, supprime ce fichier", p)
		}
		return nil, err
	}
	host, _ := os.Hostname()
	fmt.Fprintf(f, "%s pid=%d %s\n", host, os.Getpid(), time.Now().Format(time.RFC3339))
	f.Close()
	var once sync.Once
	return func() { once.Do(func() { os.Remove(p) }) }, nil
}

type section struct {
	name      string
	dir       string
	recursive bool
}

func discover(o *opts, cfg *config.Config) ([]section, error) {
	excl := map[string]bool{}
	for _, d := range cfg.ExcludeDirs {
		excl[strings.ToLower(d)] = true
	}
	ents, err := os.ReadDir(o.root)
	if err != nil {
		return nil, err
	}
	var secs []section
	rootHasPDF := false
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() {
			if strings.HasPrefix(n, ".") || excl[strings.ToLower(n)] {
				continue
			}
			secs = append(secs, section{n, filepath.Join(o.root, n), true})
		} else if isPDF(n) {
			rootHasPDF = true
		}
	}
	if rootHasPDF {
		secs = append([]section{{manifest.RootSection, o.root, false}}, secs...)
	}
	if o.only != nil {
		var f []section
		for _, s := range secs {
			if o.only[s.name] {
				f = append(f, s)
			}
		}
		secs = f
	}
	return secs, nil
}

func isPDF(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pdf") && !strings.HasPrefix(name, "~$")
}

func run(o *opts, cfg *config.Config, hash string) error {
	start := time.Now()
	secs, err := discover(o, cfg)
	if err != nil {
		return err
	}
	if len(secs) == 0 {
		return fmt.Errorf("aucune section trouvée sous %s", o.root)
	}

	// On repart de l'index existant pour conserver les sections non traitées (-only).
	infos := map[string]manifest.SectionInfo{}
	if old, err := manifest.ReadIndex(o.out); err == nil && o.only != nil {
		for _, s := range old.Sections {
			infos[s.Name] = s
		}
	}

	var errLines []string
	var total, processed, failed int
	for _, s := range secs {
		info, st, el, err := indexSection(o, cfg, hash, s)
		if err != nil {
			return fmt.Errorf("section %s : %w", s.name, err)
		}
		infos[s.name] = info
		errLines = append(errLines, el...)
		total += info.Docs
		processed += st.processed
		failed += info.Errors
	}

	idx := manifest.Index{Version: manifest.Version, Generated: time.Now()}
	for _, f := range cfg.Fields {
		idx.Fields = append(idx.Fields, manifest.Field{Name: f.Name, Label: f.Label})
	}
	for _, s := range infos {
		idx.Sections = append(idx.Sections, s)
	}
	sort.Slice(idx.Sections, func(i, j int) bool {
		a, b := idx.Sections[i].Name, idx.Sections[j].Name
		if (a == manifest.RootSection) != (b == manifest.RootSection) {
			return a == manifest.RootSection
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})
	if err := manifest.WriteAtomic(filepath.Join(o.out, manifest.IndexFile), idx, false); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(o.out, "errors.txt"), []byte(strings.Join(errLines, "\n")+"\n"), 0o644)

	log.Printf("Terminé en %s : %d documents, %d (ré)extraits, %d en erreur (voir errors.txt)",
		time.Since(start).Round(time.Second), total, processed, failed)
	return nil
}

type stats struct{ processed int }

type file struct {
	rel   string // relatif à root, "/"
	abs   string
	size  int64
	mtime int64
}

func indexSection(o *opts, cfg *config.Config, hash string, s section) (manifest.SectionInfo, stats, []string, error) {
	shardPath := filepath.Join(o.out, manifest.ShardFile(s.name))
	prev := map[string]manifest.Doc{}
	if !o.force {
		if old, err := manifest.ReadShard(shardPath); err == nil && old.ConfigHash == hash {
			for _, d := range old.Docs {
				prev[d.Path] = d
			}
		}
	}

	files, err := scan(o.root, s, cfg)
	if err != nil {
		return manifest.SectionInfo{}, stats{}, nil, err
	}

	docs := make([]manifest.Doc, len(files))
	var todo []int
	for i, f := range files {
		// Les erreurs transitoires (timeout, lecture réseau…) sont retentées ;
		// "aucun texte" / "aucun champ" sont déterministes : inutile de refaire.
		if d, ok := prev[f.rel]; ok && d.Size == f.size && d.MTime == f.mtime && !transient(d.Err) {
			docs[i] = d
			continue
		}
		todo = append(todo, i)
	}
	log.Printf("[%s] %d PDF, %d à extraire", s.name, len(files), len(todo))

	var done atomic.Int64
	stop := make(chan struct{})
	go progress(s.name, len(todo), &done, stop)

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(1, o.workers); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				f := files[i]
				d := manifest.Doc{Path: f.rel, Size: f.size, MTime: f.mtime}
				text, err := extract(o, cfg, f.abs)
				switch {
				case err != nil:
					d.Err = err.Error()
				case strings.TrimSpace(text) == "":
					d.Err = "aucun texte (PDF scanné ? OCR nécessaire)"
				default:
					d.Fields = cfg.ExtractSection(text, s.dir)
					if len(d.Fields) == 0 {
						d.Err = "aucun champ trouvé"
					}
				}
				docs[i] = d
				done.Add(1)
			}
		}()
	}
	for _, i := range todo {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(stop)

	info := manifest.SectionInfo{Name: s.name, File: manifest.ShardFile(s.name), Docs: len(docs)}
	var errLines []string
	for _, d := range docs {
		if d.Err != "" {
			info.Errors++
			errLines = append(errLines, d.Path+"\t"+d.Err)
		}
	}
	shard := manifest.Shard{
		Version: manifest.Version, Section: s.name, Generated: time.Now(),
		ConfigHash: hash, Docs: docs,
	}
	if err := manifest.WriteAtomic(shardPath, shard, true); err != nil {
		return info, stats{}, nil, err
	}
	return info, stats{processed: len(todo)}, errLines, nil
}

func scan(root string, s section, cfg *config.Config) ([]file, error) {
	excl := map[string]bool{}
	for _, d := range cfg.ExcludeDirs {
		excl[strings.ToLower(d)] = true
	}
	var files []file
	add := func(p string, info fs.FileInfo) {
		rel, _ := filepath.Rel(root, p)
		files = append(files, file{filepath.ToSlash(rel), p, info.Size(), info.ModTime().Unix()})
	}
	if !s.recursive {
		ents, err := os.ReadDir(s.dir)
		if err != nil {
			return nil, err
		}
		for _, e := range ents {
			if !e.IsDir() && isPDF(e.Name()) {
				if info, err := e.Info(); err == nil {
					add(filepath.Join(s.dir, e.Name()), info)
				}
			}
		}
	} else {
		err := filepath.WalkDir(s.dir, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("  ignoré : %s (%v)", p, err)
				if e != nil && e.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if e.IsDir() {
				if p != s.dir && (strings.HasPrefix(e.Name(), ".") || excl[strings.ToLower(e.Name())]) {
					return fs.SkipDir
				}
				return nil
			}
			if isPDF(e.Name()) {
				if info, err := e.Info(); err == nil {
					add(p, info)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, nil
}

func extract(o *opts, cfg *config.Config, path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	args := []string{"-layout", "-enc", "UTF-8", "-q"}
	if cfg.MaxPages > 0 {
		args = append(args, "-f", "1", "-l", strconv.Itoa(cfg.MaxPages))
	}
	args = append(args, path, "-")
	cmd := exec.CommandContext(ctx, o.pdftotext, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("timeout après %s", o.timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				switch ee.ExitCode() {
				case 1:
					msg = "PDF illisible ou corrompu"
				case 3:
					msg = "PDF protégé (droits d'extraction)"
				}
			}
		}
		return "", fmt.Errorf("pdftotext : %s", firstLine(msg))
	}
	return out.String(), nil
}

func transient(e string) bool {
	return strings.HasPrefix(e, "timeout") || strings.HasPrefix(e, "pdftotext")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func progress(name string, total int, done *atomic.Int64, stop chan struct{}) {
	if total == 0 {
		return
	}
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	start := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			n := done.Load()
			rate := float64(n) / time.Since(start).Seconds()
			eta := "?"
			if rate > 0 {
				eta = (time.Duration(float64(int64(total)-n)/rate) * time.Second).Round(time.Second).String()
			}
			log.Printf("[%s] %d/%d (%.0f PDF/s, reste ~%s)", name, n, total, rate, eta)
		}
	}
}

func runTest(o *opts, cfg *config.Config, path string) {
	text, err := extract(o, cfg, path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("──────── texte extrait (pdftotext -layout) ────────")
	fmt.Println(text)
	fmt.Println("──────── valeurs trouvées ────────")
	got := cfg.Extract(text)
	for _, f := range cfg.Fields {
		v := got[f.Name]
		if len(v) == 0 {
			fmt.Printf("%-12s : (rien)\n", f.Label)
		} else {
			fmt.Printf("%-12s : %s\n", f.Label, strings.Join(v, " | "))
		}
	}
}
