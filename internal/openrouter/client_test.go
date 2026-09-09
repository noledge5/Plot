package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamSammeltTextUndVerbrauch(t *testing.T) {
	var gesehen ChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gesehen); err != nil {
			t.Errorf("Anfrage nicht lesbar: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": ping\n\n")
		io.WriteString(w, `data: {"provider":"Venice","model":"m","choices":[{"delta":{"content":"Mira "}}]}`+"\n\n")
		io.WriteString(w, `data: {"choices":[{"delta":{"content":"schweigt."}}]}`+"\n\n")
		io.WriteString(w, `data: {"usage":{"prompt_tokens":194,"completion_tokens":2,"total_tokens":196,"cost":0.0012}}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := New("sk-test").WithBaseURL(srv.URL)
	var stuecke []string
	res, err := c.Stream(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "Ich klopfe."}},
	}, func(s string) { stuecke = append(stuecke, s) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if res.Content != "Mira schweigt." {
		t.Fatalf("Text = %q", res.Content)
	}
	if len(stuecke) != 2 {
		t.Fatalf("Teilstücke = %d, erwartet 2", len(stuecke))
	}
	if res.Usage.Cost != 0.0012 || res.Usage.PromptTokens != 194 {
		t.Fatalf("Verbrauch nicht übernommen: %+v", res.Usage)
	}
	if res.Provider != "Venice" {
		t.Fatalf("Anbieter = %q", res.Provider)
	}
	if !gesehen.Stream {
		t.Fatal("stream wurde nicht gesetzt")
	}
}

// Die Routing-Filter müssen exakt so im Anfragekörper landen, sonst greift
// weder die Allowlist des Erzählers noch das ZDR des Analysten.
func TestProviderFilterImKoerper(t *testing.T) {
	var roh map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&roh)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"provider":"Venice","choices":[{"delta":{"content":"ok"}}]}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	nein, ja := false, true
	c := New("sk-test").WithBaseURL(srv.URL)
	if _, err := c.CheckRouting(context.Background(), "modell", &Provider{
		Only:           []string{"venice"},
		AllowFallbacks: &nein,
		ZDR:            &ja,
		DataCollection: "deny",
	}); err != nil {
		t.Fatalf("CheckRouting: %v", err)
	}

	p, ok := roh["provider"].(map[string]any)
	if !ok {
		t.Fatalf("kein provider-Objekt: %#v", roh)
	}
	if only, _ := p["only"].([]any); len(only) != 1 || only[0] != "venice" {
		t.Fatalf("only falsch: %#v", p["only"])
	}
	if p["allow_fallbacks"] != false {
		t.Fatalf("allow_fallbacks = %#v, erwartet false", p["allow_fallbacks"])
	}
	if p["zdr"] != true {
		t.Fatalf("zdr = %#v", p["zdr"])
	}
	if p["data_collection"] != "deny" {
		t.Fatalf("data_collection = %#v", p["data_collection"])
	}
}

// Ein leeres Provider-Feld darf nicht als "provider":{} mitgeschickt werden,
// sonst schränkt eine unbeabsichtigt leere Allowlist das Routing ein.
func TestOhneProviderKeinFeld(t *testing.T) {
	var roh map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&roh)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	c := New("k").WithBaseURL(srv.URL)
	if _, err := c.Stream(context.Background(), ChatRequest{Model: "m"}, nil); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if _, da := roh["provider"]; da {
		t.Fatal("provider wurde ohne Not mitgeschickt")
	}
}

// Wenn ZDR und Allowlist sich ausschließen, antwortet OpenRouter mit einem
// Fehler. Der muss im Wortlaut bis zur Oberfläche durchkommen - das ist der
// Routing-Test aus M0.
func TestRoutingfehlerBleibtLesbar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"message":"No endpoints found matching your data policy","code":404}}`)
	}))
	defer srv.Close()
	c := New("k").WithBaseURL(srv.URL)
	_, err := c.CheckRouting(context.Background(), "modell", &Provider{})
	if err == nil {
		t.Fatal("Fehler wurde verschluckt")
	}
	if !strings.Contains(err.Error(), "data policy") {
		t.Fatalf("Meldung unbrauchbar: %v", err)
	}
}

func TestModelleAufbereiten(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[
		 {"id":"a/b","name":"AB","context_length":128000,
		  "pricing":{"prompt":"0.0000002","completion":"0.0000009"},
		  "supported_parameters":["response_format"]},
		 {"id":"c/d","name":"CD","context_length":32768,
		  "pricing":{"prompt":"0.000000019","completion":"0.00000003"},
		  "supported_parameters":["structured_outputs","response_format"]}]}`)
	}))
	defer srv.Close()

	ms, err := New("k").WithBaseURL(srv.URL).Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("Modelle = %d", len(ms))
	}
	if ms[0].PromptPerM != 0.2 || ms[0].OutputPerM != 0.9 {
		t.Fatalf("Preisumrechnung falsch: %+v", ms[0])
	}
	// Dolphin kann nur response_format, taugt also nicht als Analyst.
	if ms[0].StrictJSON {
		t.Fatal("response_format wurde als striktes JSON gewertet")
	}
	if !ms[1].StrictJSON {
		t.Fatal("structured_outputs nicht erkannt")
	}
}
