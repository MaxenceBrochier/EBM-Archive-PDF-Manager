// finder : recherche dans le manifest de l'archive EBM et ouvre le PDF.
//
// Posé à la racine du share à côté du dossier .index, il se lance tel
// quel depuis le poste client (double-clic ou terminal) :
//
//	\\srv\archive\EBM\finder.exe
//	finder.exe -root \\srv\archive\EBM
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	rootFlag := flag.String("root", "", "dossier racine de l'archive (défaut : dossier de l'exe)")
	flag.Parse()

	root, err := findRoot(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		pause()
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(root), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		pause()
		os.Exit(1)
	}
}

// pause garde la fenêtre ouverte en cas d'erreur après un double-clic.
func pause() {
	fmt.Fprint(os.Stderr, "\nAppuie sur Entrée pour fermer…")
	fmt.Scanln()
}
