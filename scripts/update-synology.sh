#!/bin/sh
# Aktualisiert Plot auf der Synology: neues Abbild holen, Container neu starten,
# alte Abbilder aufräumen. Gedacht für den DSM-Aufgabenplaner (Benutzer: root).
#
#   Systemsteuerung → Aufgabenplaner → Erstellen → Geplante Aufgabe →
#   Benutzerdefiniertes Skript, Benutzer root, im Skriptfeld:
#       sh /volume1/docker/plot/update-synology.sh
#
# Ohne Zeitplan lässt sich dieselbe Aufgabe jederzeit von Hand über
# "Ausführen" auslösen.

set -eu

PROJEKT="${PLOT_PROJEKT:-/volume1/docker/plot}"
ABBILD="${PLOT_ABBILD:-ghcr.io/noledge5/plot:latest}"
CONTAINER="${PLOT_CONTAINER:-plot}"
PORT="${PLOT_PORT:-8080}"
PROTOKOLL="$PROJEKT/update.log"

# Der Aufgabenplaner startet mit einem sehr kargen PATH, deshalb der volle Pfad.
DOCKER=/usr/local/bin/docker
[ -x "$DOCKER" ] || DOCKER="$(command -v docker || true)"
if [ -z "$DOCKER" ] || [ ! -x "$DOCKER" ]; then
	echo "docker nicht gefunden - läuft Container Manager?" >&2
	exit 1
fi

melde() { echo "$(date '+%Y-%m-%d %H:%M:%S')  $*" | tee -a "$PROTOKOLL"; }

# Fängt die Ausgabe eines Befehls getrennt ein: im Fehlerfall soll genau sie
# erscheinen, nicht das Ende eines über die Zeit gewachsenen Protokolls.
LETZTE="$(mktemp)"
trap 'rm -f "$LETZTE"' EXIT
lauf() {
	if "$@" >"$LETZTE" 2>&1; then
		cat "$LETZTE" >>"$PROTOKOLL"
		return 0
	fi
	cat "$LETZTE" >>"$PROTOKOLL"
	return 1
}

melde "Aktualisierung beginnt: $ABBILD"

# Vorher merken, was läuft: nur wenn sich die Kennung ändert, gab es wirklich
# ein Update - sonst bleibt die Meldung ehrlich.
VORHER="$("$DOCKER" image inspect --format '{{.Id}}' "$ABBILD" 2>/dev/null || echo keins)"

if ! lauf "$DOCKER" pull "$ABBILD"; then
	melde "FEHLER: Abbild konnte nicht geholt werden."
	cat "$LETZTE" >&2
	exit 1
fi
NACHHER="$("$DOCKER" image inspect --format '{{.Id}}' "$ABBILD")"

if [ "$VORHER" = "$NACHHER" ]; then
	melde "Bereits aktuell, nichts zu tun."
	exit 0
fi

# Container Manager legt Projekte mit einer Compose-Datei im Projektordner ab.
# Gibt es eine, ist sie der verlässlichere Weg, weil sie Ports und Einhängungen
# schon kennt.
COMPOSE=""
for kandidat in "$PROJEKT/docker-compose.yml" "$PROJEKT/docker-compose.yaml" "$PROJEKT/compose.yaml"; do
	[ -f "$kandidat" ] && COMPOSE="$kandidat" && break
done

neustart_fehlgeschlagen() {
	melde "FEHLER: Neustart misslungen."
	cat "$LETZTE" >&2
	exit 1
}

if [ -n "$COMPOSE" ]; then
	melde "Starte über $COMPOSE neu"
	lauf "$DOCKER" compose -f "$COMPOSE" up -d || neustart_fehlgeschlagen
else
	melde "Keine Compose-Datei gefunden, starte den Container direkt neu"
	lauf "$DOCKER" rm -f "$CONTAINER" || true
	lauf "$DOCKER" run -d --name "$CONTAINER" --restart unless-stopped \
		-p "$PORT:8080" -v "$PROJEKT:/data" "$ABBILD" || neustart_fehlgeschlagen
fi

# Ohne das sammeln sich die abgelösten Abbilder an, und auf einer kleinen NAS
# fällt das irgendwann auf.
lauf "$DOCKER" image prune -f || true

if "$DOCKER" ps --filter "name=$CONTAINER" --filter "status=running" --format '{{.Names}}' | grep -q "$CONTAINER"; then
	melde "Aktualisiert und gestartet."
else
	melde "WARNUNG: Der Container läuft nach dem Neustart nicht. Siehe $PROTOKOLL"
	exit 1
fi
