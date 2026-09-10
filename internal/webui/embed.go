// Package webui trägt die gebaute Oberfläche im Binary. Auf der NAS läuft nie
// ein Build - das Bundle entsteht in GitHub Actions und wird hier eingebettet.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS liefert das Wurzelverzeichnis der gebauten Oberfläche.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // kann nur bei einem kaputten Build passieren
	}
	return sub
}

// Vorhanden sagt, ob überhaupt eine gebaute Oberfläche eingebettet ist. In
// einem Arbeitsbaum ohne vorangegangenen Frontend-Build ist sie es nicht;
// der Server liefert dann einen Hinweis statt einer leeren Seite.
func Vorhanden() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}
