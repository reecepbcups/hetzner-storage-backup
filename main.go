// Reece Williams | Dec 2022 (Go port)
// Hetzner Backup Script for Linux Based Machines with Discord Notification Support
package main

import (
	"fmt"
	"os"

	"github.com/reecepbcups/hetzner-storage-backup/internal/backup"
	"github.com/reecepbcups/hetzner-storage-backup/internal/config"
)

const version = "0.1.0"

func main() {
	// secret.json is expected next to this binary's working directory, same
	// place the Python version expects it (repo root). Override with an arg.
	secretPath := "secret.json"
	if len(os.Args) > 1 {
		secretPath = os.Args[1]
	}

	cfg, err := config.Load(secretPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	fmt.Println("Backup Running...")

	b, err := backup.New(cfg, true, true, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := b.ZipFiles(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := b.SendFileToSFTPServer(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := b.DeleteOldestFilesInDirIfOverMax(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
