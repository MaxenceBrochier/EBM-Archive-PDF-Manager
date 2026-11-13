//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// openFile ouvre le PDF avec l'application par défaut du poste client.
func openFile(p string) error {
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("accès au fichier : %w", err)
	}
	path, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return fmt.Errorf("chemin invalide : %w", err)
	}
	if err := windows.ShellExecute(0, nil, path, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("ouverture avec l'application par défaut : %w", err)
	}
	return nil
}

// revealFile ouvre l'Explorateur sur le dossier, fichier sélectionné.
// explorer.exe a un parsing d'arguments particulier : on écrit la ligne
// de commande nous-mêmes.
func revealFile(p string) error {
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + p + `"`}
	return cmd.Start() // explorer renvoie souvent un code ≠ 0 : on n'attend pas
}
