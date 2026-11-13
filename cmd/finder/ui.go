package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const maxRows = 1000

// ─── Palette (s'adapte au fond clair / sombre du terminal) ─────────────────

var (
	cText   = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#C0CAF5"}
	cAccent = lipgloss.AdaptiveColor{Light: "#2E59C9", Dark: "#7AA2F7"}
	cOnAcc  = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#16161E"}
	cMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#7880A8"}
	cSubtle = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#4E5679"}
	cBorder = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#3B4261"}
	cBar    = lipgloss.AdaptiveColor{Light: "#E9ECF3", Dark: "#1F2335"}
	cSel    = lipgloss.AdaptiveColor{Light: "#DBE5FB", Dark: "#2A3658"}
	cOK     = lipgloss.AdaptiveColor{Light: "#2F7D32", Dark: "#9ECE6A"}
	cErr    = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#F7768E"}
	cWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#E0AF68"}
)

func fg(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func pill(text string, bg, fgc lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Background(bg).Foreground(fgc).Bold(true).Padding(0, 1).Render(text)
}

// ─── Modèle ───────────────────────────────────────────────────────────────

type loadedMsg struct {
	cat *catalog
	err error
}
type clearStatusMsg int

type model struct {
	root, indexDir string
	cat            *catalog
	loadErr        error
	loading        bool

	active  int
	input   textinput.Model
	spin    spinner.Model
	results []*entry
	total   int
	terms   []string // termes de recherche, pour le surlignage
	cursor  int
	offset  int

	status    string
	statusErr bool
	statusID  int

	w, h int
}

func newModel(root string) model {
	ti := textinput.New()
	ti.Placeholder = "N° de lot, code produit, nom de fichier…"
	ti.Prompt = "› "
	ti.PromptStyle = fg(cAccent).Bold(true)
	ti.TextStyle = fg(cText)
	ti.PlaceholderStyle = fg(cSubtle)
	ti.Cursor.Style = fg(cAccent)
	ti.Focus()
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(fg(cAccent)))
	return model{root: root, indexDir: filepath.Join(root, ".index"), input: ti, spin: sp, loading: true}
}

func loadCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		c, err := loadCatalog(dir)
		return loadedMsg{c, err}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick, loadCmd(m.indexDir))
}

func (m *model) setStatus(s string, isErr bool) tea.Cmd {
	m.status, m.statusErr = s, isErr
	m.statusID++
	id := m.statusID
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg { return clearStatusMsg(id) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.Width = max(10, m.w-30)
		m.clampScroll()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.cat, m.loadErr = msg.cat, nil
		if m.active >= len(m.cat.tabs) {
			m.active = 0
		}
		m.refresh()
		return m, nil

	case clearStatusMsg:
		if int(msg) == m.statusID {
			m.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.input.Value() != "" {
				m.input.SetValue("")
				m.refresh()
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+r", "f5":
			m.loading = true
			return m, tea.Batch(m.spin.Tick, loadCmd(m.indexDir))
		}
		if m.cat == nil {
			return m, nil
		}
		page := max(1, m.dims().rows-1)
		switch msg.String() {
		case "tab", "ctrl+right":
			m.active = (m.active + 1) % len(m.cat.tabs)
			m.refresh()
			return m, nil
		case "shift+tab", "ctrl+left":
			m.active = (m.active - 1 + len(m.cat.tabs)) % len(m.cat.tabs)
			m.refresh()
			return m, nil
		case "up":
			m.move(-1)
			return m, nil
		case "down":
			m.move(1)
			return m, nil
		case "pgup":
			m.move(-page)
			return m, nil
		case "pgdown":
			m.move(page)
			return m, nil
		case "ctrl+home":
			m.move(-len(m.results))
			return m, nil
		case "ctrl+end":
			m.move(len(m.results))
			return m, nil
		case "enter":
			return m, m.open(false)
		case "ctrl+o":
			return m, m.open(true)
		}
		prev := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != prev {
			m.refresh()
		}
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) move(n int) {
	m.cursor = max(0, min(len(m.results)-1, m.cursor+n))
	m.clampScroll()
}

func (m *model) clampScroll() {
	rows := m.dims().rows
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, len(m.results)-rows))
}

func (m *model) selected() *entry {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return nil
	}
	return m.results[m.cursor]
}

func (m *model) open(folder bool) tea.Cmd {
	e := m.selected()
	if e == nil {
		return nil
	}
	p := filepath.Join(m.root, filepath.FromSlash(e.doc.Path))
	var err error
	if folder {
		err = revealFile(p)
	} else {
		err = openFile(p)
	}
	if err != nil {
		return m.setStatus("Impossible d'ouvrir : "+err.Error(), true)
	}
	if folder {
		return m.setStatus("Dossier ouvert dans l'Explorateur", false)
	}
	return m.setStatus("Ouverture demandée : "+e.base, false)
}

func (m *model) refresh() {
	if m.cat == nil {
		return
	}
	ts := parseQuery(m.input.Value(), m.cat.idx.Fields)
	m.results, m.total = filter(m.cat.tabs[m.active].entries, ts, maxRows)
	m.terms = m.terms[:0]
	for _, t := range ts {
		m.terms = append(m.terms, t.text)
	}
	m.cursor, m.offset = 0, 0
}

// ─── Géométrie ────────────────────────────────────────────────────────────

type dims struct {
	panelH, rows   int
	resW, detailW  int
	showDetail     bool
	fieldW, fileW  int
	dirW, contentW int
}

// header 1 · vide 1 · onglets 1 · vide 1 · recherche 3 · panneaux · barre d'état 1
func (m model) dims() dims {
	var d dims
	d.panelH = max(7, m.h-8)
	d.rows = d.panelH - 4 // bord haut, en-têtes, séparateur, bord bas
	d.showDetail = m.w >= 104
	d.resW = m.w
	if d.showDetail {
		d.detailW = 38
		d.resW = m.w - d.detailW - 1
	}
	d.contentW = d.resW - 4 - 2 // bordures + padding, puis espace + barre de défilement
	nf := 1
	if m.cat != nil {
		nf = len(m.cat.idx.Fields)
	}
	d.fieldW = max(10, min(16, (d.contentW-2)/(nf+3)))
	rest := max(20, d.contentW-2-nf*(d.fieldW+2)-2)
	d.fileW = rest * 55 / 100
	d.dirW = rest - d.fileW
	return d
}

// ─── Rendu ────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.w == 0 {
		return ""
	}
	if m.cat == nil {
		var body string
		if m.loadErr != nil {
			body = lipgloss.JoinVertical(lipgloss.Center,
				fg(cErr).Bold(true).Render("✕  Index introuvable ou illisible"),
				"",
				fg(cMuted).Width(min(80, m.w-4)).Align(lipgloss.Center).Render(m.loadErr.Error()),
				"",
				fg(cSubtle).Render("Ctrl+R pour réessayer  ·  Échap pour quitter"))
		} else {
			body = m.spin.View() + fg(cMuted).Render(" Chargement de l'index…")
		}
		return m.header() + "\n" + lipgloss.Place(m.w, m.h-2, lipgloss.Center, lipgloss.Center, body) + "\n" + m.statusBar()
	}

	d := m.dims()
	search := panel("Rechercher", fg(cSubtle).Render("lot:L24  plusieurs mots = ET"),
		[]string{m.input.View()}, m.w, 3, cAccent)

	main := m.resultsPanel(d)
	if d.showDetail {
		main = lipgloss.JoinHorizontal(lipgloss.Top, main, " ", m.detailPanel(d))
	}
	return strings.Join([]string{m.header(), "", m.tabsView(), "", search, main, m.statusBar()}, "\n")
}

func (m model) header() string {
	bar := lipgloss.NewStyle().Background(cBar)
	brand := pill("◆ EBM ARCHIVE", cAccent, cOnAcc)
	title := bar.Foreground(cText).Bold(true).Render("  Recherche de documents")
	author := pill("Brochier Maxence ◆", cAccent, cOnAcc)
	right := ""
	if m.cat != nil {
		date := m.cat.idx.Generated.Local().Format("02.01.2006 15:04")
		right = bar.Foreground(cMuted).Render(m.root+"   ") +
			bar.Foreground(cSubtle).Render("index ") + bar.Foreground(cText).Render(date) + bar.Render(" ")
		if lipgloss.Width(brand+title+right+author) >= m.w {
			right = bar.Foreground(cSubtle).Render("index ") + bar.Foreground(cText).Render(date) + bar.Render(" ")
		}
	}
	gap := max(0, m.w-lipgloss.Width(brand)-lipgloss.Width(title)-lipgloss.Width(right)-lipgloss.Width(author))
	return brand + title + bar.Render(strings.Repeat(" ", gap)) + right + author
}

func (m model) tabsView() string {
	parts := make([]string, len(m.cat.tabs))
	for i, t := range m.cat.tabs {
		n := thousands(len(t.entries))
		if i == m.active {
			parts[i] = pill(t.label+"  "+n, cAccent, cOnAcc)
		} else {
			parts[i] = lipgloss.NewStyle().Padding(0, 1).Render(fg(cMuted).Render(t.label) + " " + fg(cSubtle).Render(n))
		}
	}
	join := func(p []string) string { return strings.Join(p, " ") }
	avail := m.w - 4
	start, end := 0, len(parts)
	for start < m.active && lipgloss.Width(join(parts[start:m.active+1])) > avail {
		start++
	}
	for end > m.active+1 && lipgloss.Width(join(parts[start:end])) > avail {
		end--
	}
	vis := parts[start:end]
	if start > 0 {
		vis = append([]string{fg(cSubtle).Render("‹")}, vis...)
	}
	if end < len(parts) {
		vis = append(vis, fg(cSubtle).Render("›"))
	}
	return " " + join(vis)
}

func (m model) resultsPanel(d dims) string {
	sep := "  "
	head := "  "
	for _, f := range m.cat.idx.Fields {
		head += colHead(f.Label, d.fieldW) + sep
	}
	head += colHead("Fichier", d.fileW) + sep + colHead("Dossier", d.dirW)

	lines := []string{head, fg(cBorder).Render(strings.Repeat("─", d.resW-4))}

	if len(m.results) == 0 {
		msg := fg(cMuted).Render("Aucun document dans cet onglet")
		if q := strings.TrimSpace(m.input.Value()); q != "" {
			msg = fg(cMuted).Render("Aucun document ne correspond à ") + fg(cText).Bold(true).Render("« "+q+" »")
		}
		hint := fg(cSubtle).Render("Essaie un autre onglet (Tab) ou une partie du numéro seulement")
		for i := 0; i < d.rows/2-1; i++ {
			lines = append(lines, "")
		}
		lines = append(lines, lipgloss.PlaceHorizontal(d.resW-4, lipgloss.Center, msg), "",
			lipgloss.PlaceHorizontal(d.resW-4, lipgloss.Center, hint))
	} else {
		bar := scrollbar(len(m.results), d.rows, m.offset)
		showSection := m.cat.tabs[m.active].name == ""
		for i := 0; i < d.rows; i++ {
			idx := m.offset + i
			if idx >= len(m.results) {
				lines = append(lines, strings.Repeat(" ", d.contentW+1)+bar[i])
				continue
			}
			lines = append(lines, m.row(m.results[idx], idx == m.cursor, d, showSection)+" "+bar[i])
		}
	}

	right := fg(cMuted).Render(thousands(m.total) + " résultat")
	if m.total > 1 {
		right += fg(cMuted).Render("s")
	}
	if m.total > len(m.results) {
		right += fg(cWarn).Render("  · " + thousands(len(m.results)) + " affichés")
	}
	return panel("Documents", right, lines, d.resW, d.panelH, cBorder)
}

func colHead(s string, w int) string {
	return fg(cMuted).Bold(true).Render(fit(strings.ToUpper(s), w))
}

func (m model) row(e *entry, sel bool, d dims, showSection bool) string {
	base := fg(cText)
	hi := fg(cAccent).Bold(true)
	dim := fg(cMuted)
	marker := "  "
	if sel {
		base = base.Background(cSel)
		hi = hi.Background(cSel)
		dim = dim.Background(cSel)
		marker = fg(cAccent).Background(cSel).Render("▌ ")
	}
	gap := base.Render("  ")
	var b strings.Builder
	b.WriteString(marker)
	for i, v := range e.cells {
		st := base
		if i == 0 {
			st = st.Bold(true)
		}
		b.WriteString(highlight(v, d.fieldW, st, hi, m.terms))
		b.WriteString(gap)
	}
	b.WriteString(highlight(e.base, d.fileW, base, hi, m.terms))
	b.WriteString(gap)
	dir := e.dir
	if !showSection {
		dir = strings.TrimPrefix(strings.TrimPrefix(dir, e.section), "/")
	}
	if dir == "" {
		dir = "—"
	}
	b.WriteString(highlight(dir, d.dirW-2, dim, hi, m.terms))
	b.WriteString(base.Render("  "))
	return b.String()
}

func (m model) detailPanel(d dims) string {
	iw := d.detailW - 4
	label := func(s string) string { return fg(cSubtle).Bold(true).Render(strings.ToUpper(s)) }
	wrap := func(s string, st lipgloss.Style) []string {
		return strings.Split(st.Width(iw).Render(s), "\n")
	}
	var lines []string
	e := m.selected()
	if e == nil {
		lines = append(lines, "", fg(cSubtle).Render("Aucun document sélectionné"))
		return panel("Détails", "", lines, d.detailW, d.panelH, cBorder)
	}

	lines = append(lines, "")
	for i, f := range m.cat.idx.Fields {
		vals := e.doc.Fields[f.Name]
		lines = append(lines, label(f.Label))
		if len(vals) == 0 {
			lines = append(lines, fg(cSubtle).Render("non trouvé"))
		}
		for j, v := range vals {
			st := fg(cText)
			if j == 0 && i == 0 {
				st = fg(cAccent).Bold(true)
			} else if j == 0 {
				st = fg(cText).Bold(true)
			}
			lines = append(lines, wrap(v, st)...)
		}
		lines = append(lines, "")
	}
	lines = append(lines, label("Fichier"))
	lines = append(lines, wrap(e.base, fg(cText))...)
	lines = append(lines, "", label("Emplacement"))
	lines = append(lines, wrap(filepath.Join(m.root, filepath.FromSlash(e.dir)), fg(cMuted))...)
	lines = append(lines, "", label("Modifié"),
		fg(cText).Render(time.Unix(e.doc.MTime, 0).Format("02.01.2006 15:04"))+fg(cSubtle).Render("  ·  "+humanSize(e.doc.Size)),
		"")
	if e.doc.Err == "" {
		lines = append(lines, fg(cOK).Render("●")+fg(cMuted).Render(" Indexé"))
	} else {
		lines = append(lines, wrap("▲ "+e.doc.Err, fg(cWarn))...)
	}

	actions := []string{
		key("⏎") + fg(cMuted).Render("  Ouvrir le PDF"),
		key("^O") + fg(cMuted).Render(" Ouvrir le dossier"),
	}
	inner := d.panelH - 2
	for len(lines) < inner-len(actions)-1 {
		lines = append(lines, "")
	}
	lines = append(lines, "")
	lines = append(lines, actions...)
	return panel("Détails", "", lines, d.detailW, d.panelH, cBorder)
}

func key(k string) string { return pill(k, cBar, cAccent) }

func (m model) statusBar() string {
	bar := lipgloss.NewStyle().Background(cBar)
	var left string
	switch {
	case m.status != "" && m.statusErr:
		left = pill("✕ ERREUR", cErr, cOnAcc) + bar.Foreground(cErr).Render(" "+m.status)
	case m.status != "":
		left = pill("✓ INFO", cOK, cOnAcc) + bar.Foreground(cText).Render(" "+m.status)
	case m.loading:
		left = pill("CHARGEMENT", cWarn, cOnAcc)
	default:
		left = pill("RECHERCHE", cAccent, cOnAcc)
		if len(m.results) > 0 {
			left += bar.Foreground(cMuted).Render(fmt.Sprintf("  %s / %s", thousands(m.cursor+1), thousands(m.total)))
		}
	}
	hints := [][2]string{{"tab", "onglet"}, {"↑↓", "naviguer"}, {"⏎", "ouvrir"}, {"^O", "dossier"}, {"^R", "recharger"}, {"esc", "quitter"}}
	for n := len(hints); n >= 0; n-- {
		var hs []string
		for _, h := range hints[:n] {
			hs = append(hs, bar.Foreground(cAccent).Bold(true).Render(h[0])+bar.Foreground(cMuted).Render(" "+h[1]))
		}
		right := strings.Join(hs, bar.Render("   ")) + bar.Render(" ")
		gap := m.w - lipgloss.Width(left) - lipgloss.Width(right)
		if gap >= 1 || n == 0 {
			return left + bar.Render(strings.Repeat(" ", max(0, gap))) + right
		}
	}
	return left
}

// ─── Briques de rendu ─────────────────────────────────────────────────────

// panel dessine un cadre arrondi avec un titre dans la bordure haute.
func panel(title, right string, lines []string, w, h int, border lipgloss.TerminalColor) string {
	b := fg(border)
	iw := w - 4
	t := " " + fg(cText).Bold(true).Render(title) + " "
	r := ""
	if right != "" {
		r = " " + right + " "
	}
	fill := max(0, w-4-lipgloss.Width(t)-lipgloss.Width(r))
	var sb strings.Builder
	sb.WriteString(b.Render("╭─") + t + b.Render(strings.Repeat("─", fill)) + r + b.Render("─╮"))
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		sb.WriteString("\n" + b.Render("│") + " " + fit(l, iw) + " " + b.Render("│"))
	}
	sb.WriteString("\n" + b.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return sb.String()
}

// fit tronque (avec …) ou complète à exactement w colonnes, sans casser l'ANSI.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// highlight rend un texte sur w colonnes en mettant en valeur les termes recherchés.
func highlight(s string, w int, base, hi lipgloss.Style, terms []string) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	pad := base.Render(strings.Repeat(" ", w-lipgloss.Width(s)))
	low := strings.ToLower(s)
	if len(terms) == 0 || len(low) != len(s) {
		return base.Render(s) + pad
	}
	mark := make([]bool, len(s))
	any := false
	for _, t := range terms {
		if t == "" {
			continue
		}
		for from := 0; ; {
			i := strings.Index(low[from:], t)
			if i < 0 {
				break
			}
			for k := from + i; k < from+i+len(t); k++ {
				mark[k] = true
			}
			any = true
			from += i + len(t)
		}
	}
	if !any {
		return base.Render(s) + pad
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && mark[j] == mark[i] {
			j++
		}
		if mark[i] {
			b.WriteString(hi.Render(s[i:j]))
		} else {
			b.WriteString(base.Render(s[i:j]))
		}
		i = j
	}
	return b.String() + pad
}

func scrollbar(total, rows, offset int) []string {
	out := make([]string, rows)
	if total <= rows {
		for i := range out {
			out[i] = " "
		}
		return out
	}
	thumb := max(1, rows*rows/total)
	pos := offset * (rows - thumb) / max(1, total-rows)
	for i := range out {
		if i >= pos && i < pos+thumb {
			out[i] = fg(cAccent).Render("┃")
		} else {
			out[i] = fg(cBorder).Render("│")
		}
	}
	return out
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	var out []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(out)
}

func humanSize(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f Mo", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f Ko", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d o", b)
}
