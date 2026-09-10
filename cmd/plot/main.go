// Kommando plot startet die Rollenspiel-Engine: ein Prozess, eine
// SQLite-Datei, ein Datenordner.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/noledge5/plot/internal/config"
	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/server"
)

// version wird beim Bauen gesetzt (-ldflags "-X main.version=..."). Ohne
// Angabe steht hier "dev" - damit man einem laufenden Container ansieht,
// welcher Stand darin steckt.
var version = "dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		log.Error("Start abgebrochen", "fehler", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Error("Datenbank nicht nutzbar", "ordner", cfg.DataDir, "fehler", err)
		os.Exit(1)
	}
	defer database.Close()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: server.New(cfg, database, log, version),
		// Kein WriteTimeout: eine Erzählantwort streamt minutenlang, ein
		// Zeitlimit auf der Schreibseite würde sie mittendrin abschneiden.
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	beenden := make(chan os.Signal, 1)
	signal.Notify(beenden, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Info("Plot läuft", "version", version, "adresse", cfg.Addr, "daten", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server gestoppt", "fehler", err)
			os.Exit(1)
		}
	}()

	<-beenden
	log.Info("Beende, laufende Antworten dürfen auslaufen")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Warn("Beenden unsauber", "fehler", err)
	}
}
