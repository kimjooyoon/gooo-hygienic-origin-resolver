package originresolver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureSpec(t *testing.T) Spec {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", ".gooo", "origin-resolver.gooo")
	spec, _, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestDeclaredScenarios(t *testing.T) {
	spec := fixtureSpec(t)
	all, err := ResolveAll(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Reports) != 5 {
		t.Fatalf("got %d reports, want 5", len(all.Reports))
	}
	want := map[string]Status{
		"normal-nested-expansion": StatusClosed,
		"intended-capture":        StatusClosed,
		"unintended-capture":      StatusRefuted,
		"missing-origin":          StatusUnknown,
		"replay":                  StatusClosed,
	}
	for _, report := range all.Reports {
		if report.Status != want[report.Scenario] {
			t.Fatalf("scenario %s got %s, want %s", report.Scenario, report.Status, want[report.Scenario])
		}
		if report.Status != report.ExpectedStatus {
			t.Fatalf("scenario %s got %s, contract expects %s", report.Scenario, report.Status, report.ExpectedStatus)
		}
		for _, symbol := range report.Symbols {
			if len(symbol.OriginProofPath) == 0 {
				t.Fatalf("symbol %s has no origin proof path", symbol.ID)
			}
		}
		for _, reference := range report.References {
			if len(reference.OriginProofPath) == 0 {
				t.Fatalf("reference %s has no origin proof path", reference.ID)
			}
		}
	}
}

func TestNestedFreshBindersAreAlphaRenamed(t *testing.T) {
	spec := fixtureSpec(t)
	report, err := ResolveScenario(spec, "normal-nested-expansion")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusClosed {
		t.Fatalf("got %s, want CLOSED", report.Status)
	}
	byID := map[string]SymbolEvidence{}
	for _, symbol := range report.Symbols {
		byID[symbol.ID] = symbol
	}
	outer := byID["normal.outer.x"]
	inner := byID["normal.inner.x"]
	if outer.StableIdentity == "" || inner.StableIdentity == "" || outer.StableIdentity == inner.StableIdentity {
		t.Fatalf("stable identities are not distinct: %#v %#v", outer, inner)
	}
	if outer.EffectiveSpelling == "x" || inner.EffectiveSpelling == "x" || outer.EffectiveSpelling == inner.EffectiveSpelling {
		t.Fatalf("fresh binders were not alpha-renamed: %#v %#v", outer, inner)
	}
	for _, reference := range report.References {
		if reference.ActualTarget != reference.ExpectedTarget {
			t.Fatalf("reference %s got %s, want %s", reference.ID, reference.ActualTarget, reference.ExpectedTarget)
		}
		if reference.CaptureDecision != spec.Semantics.CaptureJudgment.FreshCollision {
			t.Fatalf("reference %s got decision %s", reference.ID, reference.CaptureDecision)
		}
	}
}

func TestIntendedCaptureIsClosed(t *testing.T) {
	spec := fixtureSpec(t)
	report, err := ResolveScenario(spec, "intended-capture")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusClosed || len(report.References) != 1 || report.References[0].CaptureDecision != spec.Semantics.CaptureJudgment.IntendedTarget {
		t.Fatalf("unexpected intended capture report: %#v", report)
	}
	if report.References[0].ActualTarget != "user.x" {
		t.Fatalf("intended capture target is %q", report.References[0].ActualTarget)
	}
}

func TestUnintendedCaptureIsRefuted(t *testing.T) {
	spec := fixtureSpec(t)
	report, err := ResolveScenario(spec, "unintended-capture")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusRefuted || len(report.References) != 1 {
		t.Fatalf("unexpected refuted report: %#v", report)
	}
	if report.References[0].CaptureDecision != spec.Semantics.CaptureJudgment.UnintendedTarget || report.References[0].ActualTarget != "refuted.x" {
		t.Fatalf("unexpected capture decision: %#v", report.References[0])
	}
}

func TestMissingOriginPreservesSixFields(t *testing.T) {
	spec := fixtureSpec(t)
	report, err := ResolveScenario(spec, "missing-origin")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusUnknown || report.Unknown == nil {
		t.Fatalf("unexpected unknown report: %#v", report)
	}
	if err := report.Unknown.Validate(); err != nil {
		t.Fatal(err)
	}
	if report.Symbols[1].CaptureDecision != spec.Semantics.CaptureJudgment.MissingOrigin || report.References[0].CaptureDecision != spec.Semantics.CaptureJudgment.MissingOrigin {
		t.Fatalf("missing origin was not preserved: %#v %#v", report.Symbols, report.References)
	}
}

func TestReplayPreservesEvidence(t *testing.T) {
	spec := fixtureSpec(t)
	all, err := ResolveAll(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, report := range all.Reports {
		if report.Scenario != "replay" {
			continue
		}
		if report.Replay == nil || report.Replay.Status != StatusClosed || !report.Replay.SameIdentities || !report.Replay.SameNames || !report.Replay.SameDecisions {
			t.Fatalf("replay evidence is not closed: %#v", report.Replay)
		}
		return
	}
	t.Fatal("replay scenario was not reported")
}

func TestEmitterProducesStructuredCaptureFreeGo(t *testing.T) {
	spec := fixtureSpec(t)
	source, err := EmitExample(spec, "normal-nested-expansion")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{"func CaptureFree() string", "func stableIdentity", "func resolveNames", "__gooo_"} {
		if !strings.Contains(text, required) {
			t.Fatalf("generated source is missing %q", required)
		}
	}
	if strings.Contains(text, "strings.Replace") || strings.Contains(text, "strings.ReplaceAll") {
		t.Fatal("generated source contains arbitrary string replacement")
	}
	if err := os.WriteFile(filepath.Join(t.TempDir(), "generated.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
}
