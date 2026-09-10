// Package openrouter spricht mit der OpenRouter-API: Modellkatalog,
// Chat-Completions mit Streaming und die Routing-Filter, die bestimmen, welche
// Anbieter eine Anfrage überhaupt sehen dürfen.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://openrouter.ai/api/v1"

type Client struct {
	key     string
	baseURL string
	http    *http.Client
}

func New(key string) *Client {
	return &Client{
		key:     key,
		baseURL: BaseURL,
		// Kein Gesamttimeout: eine Erzählantwort darf streamen, solange sie
		// will. Abgebrochen wird über den Context.
		http: &http.Client{},
	}
}

// WithBaseURL setzt eine abweichende Basisadresse, etwa für Tests oder einen
// vorgeschalteten Proxy.
func (c *Client) WithBaseURL(u string) *Client { c.baseURL = u; return c }

func (c *Client) HasKey() bool { return strings.TrimSpace(c.key) != "" }

func (c *Client) request(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("X-Title", "Plot")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// apiError übersetzt die Fehlerantworten von OpenRouter in eine lesbare Meldung.
// Gerade beim Routing-Test ist das der interessante Teil: "kein Anbieter
// erfüllt die Bedingungen" muss als solches ankommen.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("OpenRouter antwortete mit Status %d", e.Status)
	}
	return e.Message
}

func parseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Error.Message != "" {
		return &apiError{Status: resp.StatusCode, Message: wrapped.Error.Message}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return &apiError{Status: resp.StatusCode, Message: msg}
}

// ---------- Modellkatalog ----------

type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

// perMillion rechnet die Preisangabe (Dollar pro Token, als Zeichenkette) in
// Dollar pro Million Token um - die Einheit, in der Preise verglichen werden.
// Gerundet wird, weil die Multiplikation sonst 0.19999999999999998 statt 0.2
// liefert und das so in der Oberfläche landet.
func perMillion(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Round(v*1e6*1e6) / 1e6
}

type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ContextLength int      `json:"context_length"`
	Pricing       Pricing  `json:"pricing"`
	Supported     []string `json:"supported_parameters"`
}

// ModelInfo ist die aufbereitete Sicht für die Oberfläche.
type ModelInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ContextLength int     `json:"contextLength"`
	PromptPerM    float64 `json:"promptPerM"`
	OutputPerM    float64 `json:"outputPerM"`
	// StrictJSON entscheidet, ob ein Modell als Analyst taugen würde: die
	// Zustandsmechanik ab M4 braucht schema-validiertes JSON.
	StrictJSON bool `json:"strictJson"`
}

func (c *Client) Models(ctx context.Context) ([]ModelInfo, error) {
	req, err := c.request(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Modellkatalog abrufen: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}
	var payload struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("Modellkatalog lesen: %w", err)
	}
	out := make([]ModelInfo, 0, len(payload.Data))
	for _, m := range payload.Data {
		info := ModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			ContextLength: m.ContextLength,
			PromptPerM:    perMillion(m.Pricing.Prompt),
			OutputPerM:    perMillion(m.Pricing.Completion),
		}
		for _, p := range m.Supported {
			if p == "structured_outputs" {
				info.StrictJSON = true
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// ---------- Chat ----------

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider bildet die Routing-Regeln ab. Für die Erzählung wird Only gesetzt
// (Allowlist), für den Analysten ZDR - siehe docs/modelle.md.
type Provider struct {
	Only           []string `json:"only,omitempty"`
	Ignore         []string `json:"ignore,omitempty"`
	Order          []string `json:"order,omitempty"`
	AllowFallbacks *bool    `json:"allow_fallbacks,omitempty"`
	DataCollection string   `json:"data_collection,omitempty"`
	ZDR            *bool    `json:"zdr,omitempty"`
}

type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	// Temperature ist ein Zeiger, damit 0 (völlig unkreativ) sich von
	// "nicht gesetzt" unterscheidet - Auswertungsaufrufe wollen die 0.
	Temperature    *float64        `json:"temperature,omitempty"`
	Provider       *Provider       `json:"provider,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat verlangt eine schema-validierte Antwort. Nicht jedes Modell
// kann das: der Katalog führt es als "structured_outputs" (siehe ModelInfo).
type ResponseFormat struct {
	Type       string      `json:"type"` // "json_schema"
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

type JSONSchema struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict"`
	Schema any    `json:"schema"`
}

// JSONAntwort baut das Format für einen Auswertungsaufruf.
func JSONAntwort(name string, schema any) *ResponseFormat {
	return &ResponseFormat{Type: "json_schema", JSONSchema: &JSONSchema{
		Name: name, Strict: true, Schema: schema,
	}}
}

type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

type Result struct {
	Content  string        `json:"content"`
	Usage    Usage         `json:"usage"`
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Latency  time.Duration `json:"-"`
}

// Stream schickt eine Anfrage und ruft onDelta für jedes Textstück auf.
// Zurück kommt der vollständige Text samt Verbrauch und tatsächlich
// bedienendem Anbieter.
func (c *Client) Stream(ctx context.Context, in ChatRequest, onDelta func(string)) (*Result, error) {
	in.Stream = true
	start := time.Now()

	req, err := c.request(ctx, http.MethodPost, "/chat/completions", in)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Anfrage an OpenRouter: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	res := &Result{Model: in.Model}
	var text strings.Builder

	sc := bufio.NewScanner(resp.Body)
	// Einzelne SSE-Zeilen können lang werden; die Vorgabe von 64 KB reicht
	// bei großen Chunks nicht immer.
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue // Kommentar-Zeilen halten die Verbindung offen
		}
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
			Choices  []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // unvollständige Zeile, nächste abwarten
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return nil, &apiError{Status: resp.StatusCode, Message: chunk.Error.Message}
		}
		if chunk.Provider != "" {
			res.Provider = chunk.Provider
		}
		if chunk.Model != "" {
			res.Model = chunk.Model
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				text.WriteString(ch.Delta.Content)
				if onDelta != nil {
					onDelta(ch.Delta.Content)
				}
			}
		}
		// Der Verbrauch steht im letzten Chunk und wird immer mitgeliefert.
		if chunk.Usage != nil {
			res.Usage = *chunk.Usage
		}
	}
	if err := sc.Err(); err != nil {
		// Abbruch durch den Nutzer ist kein Fehler - der Text bis hierhin zählt.
		if ctx.Err() != nil {
			res.Content = text.String()
			res.Latency = time.Since(start)
			return res, ctx.Err()
		}
		return nil, fmt.Errorf("Antwortstrom abgebrochen: %w", err)
	}

	res.Content = text.String()
	res.Latency = time.Since(start)
	return res, nil
}

// CheckRouting stellt die Frage aus M0: Kommt dieses Modell unter diesen
// Routing-Bedingungen überhaupt bei einem Anbieter an? Kostet einen einzigen
// Token.
func (c *Client) CheckRouting(ctx context.Context, model string, p *Provider) (provider string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	res, err := c.Stream(ctx, ChatRequest{
		Model:     model,
		Messages:  []Message{{Role: "user", Content: "ok"}},
		MaxTokens: 1,
		Provider:  p,
	}, nil)
	if err != nil {
		return "", err
	}
	return res.Provider, nil
}
