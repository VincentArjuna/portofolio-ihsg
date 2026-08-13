package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ai_test.go — covers the two non-trivial pieces of the Hermes bridge:
// parseAIResult (robust JSON extraction across the output shapes Hermes -Q can
// emit) and runHermes (the os/exec subprocess call), driven through a stub
// binary that mimics `hermes chat -q … -Q`.

func TestCanonVerdict(t *testing.T) {
	cases := map[string]string{
		"BUY":       verdictBuy,
		"buy":       verdictBuy,
		"Strong Buy": verdictBuy,
		"beli":      verdictBuy,
		"SELL":      verdictSell,
		"jual":      verdictSell,
		"HOLD":      verdictHold,
		"tahan":     verdictHold,
		"keep":      verdictHold,
		"":          "",
		"garbage":   "",
	}
	for in, want := range cases {
		if got := canonVerdict(in); got != want {
			t.Errorf("canonVerdict(%q) = %q, want %q", in, got, want)
		}
	}
}

// validPayload is the canonical Hermes response used across parse cases.
const validPayload = `{"short_term":{"verdict":"BUY","confidence":0.82,"reasoning":"tren di atas MA"},"long_term":{"verdict":"HOLD","confidence":0.61,"reasoning":"fundamental baik"},"risk_factors":["koreksi pasar"],"data_limitations":["harga tertunda 15 menit"]}`

func TestParseAIResult_Clean(t *testing.T) {
	// Hermes -Q appends a blank line + `session_id: …` (no braces) after the JSON.
	in := []byte(validPayload + "\n\nsession_id: 20260814_003303_ee3a27\n")
	r, err := parseAIResult(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.ShortTerm.Verdict != verdictBuy || r.LongTerm.Verdict != verdictHold {
		t.Fatalf("verdicts = %q/%q, want BUY/HOLD", r.ShortTerm.Verdict, r.LongTerm.Verdict)
	}
	if r.ShortTerm.Confidence != 0.82 {
		t.Fatalf("short confidence = %v, want 0.82", r.ShortTerm.Confidence)
	}
	if len(r.RiskFactors) != 1 || r.RiskFactors[0] != "koreksi pasar" {
		t.Fatalf("risk factors = %v", r.RiskFactors)
	}
	if len(r.DataLimitations) != 1 {
		t.Fatalf("data limitations = %v", r.DataLimitations)
	}
}

func TestParseAIResult_MarkdownFenced(t *testing.T) {
	// Some models wrap JSON in ```json fences. Fences carry no braces, so the
	// first-'{'..last-'}' bound still captures the whole object.
	in := []byte("Berikut analisisnya:\n```json\n" + validPayload + "\n```\n\nsession_id: x")
	r, err := parseAIResult(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.ShortTerm.Verdict != verdictBuy {
		t.Fatalf("short verdict = %q, want BUY", r.ShortTerm.Verdict)
	}
}

func TestParseAIResult_NormalizesVerdict(t *testing.T) {
	in := []byte(`{"short_term":{"verdict":"Strong Buy","confidence":0.9,"reasoning":"x"},"long_term":{"verdict":"jual","confidence":0.3,"reasoning":"y"},"risk_factors":[],"data_limitations":[]}`)
	r, err := parseAIResult(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.ShortTerm.Verdict != verdictBuy || r.LongTerm.Verdict != verdictSell {
		t.Fatalf("normalized = %q/%q, want BUY/SELL", r.ShortTerm.Verdict, r.LongTerm.Verdict)
	}
}

func TestParseAIResult_NoJSON(t *testing.T) {
	if _, err := parseAIResult([]byte("Hermes error: model unavailable\nsession_id: z")); err == nil {
		t.Fatal("expected error for output without JSON")
	}
}

func TestParseAIResult_EmptyVerdict(t *testing.T) {
	// Missing verdict field → canonVerdict("") → error (caller must not store).
	in := []byte(`{"short_term":{"confidence":0.5,"reasoning":"x"},"long_term":{"verdict":"HOLD","confidence":0.5,"reasoning":"y"},"risk_factors":[],"data_limitations":[]}`)
	if _, err := parseAIResult(in); err == nil {
		t.Fatal("expected error for empty short_term verdict")
	}
}

// writeHermesStub creates a fake `hermes` executable that prints a canned -Q
// response (JSON + trailing session_id line) regardless of its arguments.
func writeHermesStub(t *testing.T, payload string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hermes")
	// /bin/sh ignores positional args; prints payload then a session_id line.
	script := "#!/bin/sh\ncat <<'__HERMES_EOF__'\n" + payload + "\n\nsession_id: stub-session\n__HERMES_EOF__\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

// TestRunHermes_Stub verifies the subprocess call end-to-end: runHermes invokes
// the stub exactly as it would the real `hermes chat -q … -Q`, and the captured
// stdout then parses into the expected verdicts.
func TestRunHermes_Stub(t *testing.T) {
	stub := writeHermesStub(t, validPayload)

	out, err := runHermes(context.Background(), stub, "prompt apa pun")
	if err != nil {
		t.Fatalf("runHermes failed: %v", err)
	}
	r, err := parseAIResult(out)
	if err != nil {
		t.Fatalf("parse failed: %v (raw=%q)", err, out)
	}
	if r.ShortTerm.Verdict != verdictBuy || r.LongTerm.Verdict != verdictHold {
		t.Fatalf("verdicts = %q/%q, want BUY/HOLD", r.ShortTerm.Verdict, r.LongTerm.Verdict)
	}
	// The stub's session_id line must not corrupt parsing.
	if r.ShortTerm.Confidence != 0.82 {
		t.Fatalf("confidence = %v, want 0.82", r.ShortTerm.Confidence)
	}
}

// TestRunHermes_MissingBinary — a path that doesn't resolve yields an error,
// not a panic (the handler maps this to the "unavailable" fallback via
// LookPath, but runHermes itself must also fail cleanly).
func TestRunHermes_MissingBinary(t *testing.T) {
	_, err := runHermes(context.Background(), "/no/such/hermes-binary", "x")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}
