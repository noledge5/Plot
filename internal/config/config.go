// Package config hält die wenigen Angaben, die beim Start feststehen müssen.
// Alles andere - OpenRouter-Key, Modelle, Routing - steht in der Datenbank und
// wird in der Weboberfläche gesetzt, damit auf der NAS nie eine Datei von Hand
// bearbeitet werden muss.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir string
	Addr    string
}

func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("plot", flag.ContinueOnError)
	c := &Config{}
	fs.StringVar(&c.DataDir, "data", envOr("PLOT_DATA", "/data"),
		"Ordner für Datenbank und Uploads")
	fs.StringVar(&c.Addr, "addr", envOr("PLOT_ADDR", ":8080"),
		"Adresse, auf der der Server lauscht")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return nil, fmt.Errorf("Datenordner auflösen: %w", err)
	}
	c.DataDir = abs
	if err := os.MkdirAll(c.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("Datenordner %s anlegen: %w", c.DataDir, err)
	}
	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
