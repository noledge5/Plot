# Das Bild wird für die Zielarchitektur gebaut, ohne Emulation: beide
# Builder-Stufen laufen nativ auf der Bauplattform und übersetzen quer.
# Auf einer NAS mit 1 GB RAM soll nichts kompiliert werden müssen.

FROM --platform=$BUILDPLATFORM node:22-alpine AS oberflaeche
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
# Vite schreibt nach ../internal/webui/dist, deshalb muss der Zielpfad existieren.
RUN mkdir -p /internal/webui/dist && npm run build && cp -r /internal/webui/dist /fertig

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS bau
WORKDIR /quelle
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=oberflaeche /fertig/ ./internal/webui/dist/
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /plot ./cmd/plot

FROM scratch
# Ohne Wurzelzertifikate scheitert jede HTTPS-Verbindung zu OpenRouter.
COPY --from=bau /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=bau /plot /plot
VOLUME ["/data"]
EXPOSE 8080
ENV PLOT_DATA=/data PLOT_ADDR=:8080
ENTRYPOINT ["/plot"]
