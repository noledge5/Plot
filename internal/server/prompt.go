package server

import (
	"strings"

	"github.com/noledge5/plot/internal/db"
	"github.com/noledge5/plot/internal/openrouter"
)

// baueNachrichten setzt den Prompt zusammen: der System-Prompt der Geschichte,
// danach der jüngste Verlauf.
//
// Der Verlauf wird bewusst hart begrenzt, auch wenn das Modell mehr Kontext
// könnte. Kleine Modelle verlieren Anweisungen in langen Prompts - mehr
// Verlauf heißt schlechtere Regelbefolgung, nicht bessere (PLAN.md 5.1).
// Ab M3 wandert das Abgeschnittene in eine Zusammenfassung, statt einfach zu
// verschwinden.
func baueNachrichten(st *db.Story, pfad []db.Node, maxTurns int) (msgs []openrouter.Message, abgeschnitten int) {
	if maxTurns < 1 {
		maxTurns = 1
	}
	if s := strings.TrimSpace(st.SystemPrompt); s != "" {
		msgs = append(msgs, openrouter.Message{Role: "system", Content: s})
	}
	von := 0
	if len(pfad) > maxTurns {
		von = len(pfad) - maxTurns
		abgeschnitten = von
	}
	for _, n := range pfad[von:] {
		rolle := n.Role
		if rolle != "user" && rolle != "assistant" {
			rolle = "user"
		}
		if strings.TrimSpace(n.Content) == "" {
			continue
		}
		msgs = append(msgs, openrouter.Message{Role: rolle, Content: n.Content})
	}
	return msgs, abgeschnitten
}
