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

func TestContractVectorAndDeclaredCases(t *testing.T) {
	spec := fixtureSpec(t)
	all, err := ResolveAll(spec)
	if err != nil {
		t.Fatal(err)
	}
	if all.Vector != (CaseVector{Cases: 12, Closed: 4, Unknown: 4, Refuted: 4}) {
		t.Fatalf("got vector %#v", all.Vector)
	}
	for _, report := range all.Reports {
		if len(report.IR) == 0 {
			t.Fatalf("case %s has no generated IR", report.Scenario)
		}
		for _, symbol := range report.Symbols {
			if len(symbol.OriginProofPath) == 0 {
				t.Fatalf("symbol %s has no origin proof path", symbol.ID)
			}
		}
	}
}

func TestNestedStagesAndTwoSplices(t *testing.T) {
	spec := fixtureSpec(t)
	report, err := ResolveScenario(spec, "nested-quasiquote")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusClosed || len(report.IR) != 1 || len(report.IR[0].Children) != 3 {
		t.Fatalf("unexpected nested report: %#v", report)
	}
	inner := report.IR[0].Children[2]
	if inner.Kind != "quasiquote" || len(inner.Children) != 1 || inner.Children[0].Kind != "splice" {
		t.Fatalf("nested quasiquote/splice was not preserved: %#v", inner)
	}
	two, err := ResolveScenario(spec, "two-splices")
	if err != nil {
		t.Fatal(err)
	}
	if len(two.IR[0].Children) != 2 || two.IR[0].Children[0].Kind != "splice" || two.IR[0].Children[1].Kind != "splice" {
		t.Fatalf("two splice order was not preserved: %#v", two.IR)
	}
}

func TestDeterministicAlphaRenamingAndShadowing(t *testing.T) {
	spec := fixtureSpec(t)
	first, err := ResolveScenario(spec, "two-splices")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ResolveScenario(spec, "two-splices")
	if err != nil {
		t.Fatal(err)
	}
	if first.Symbols[1].EffectiveSpelling != second.Symbols[1].EffectiveSpelling || first.Symbols[2].EffectiveSpelling != second.Symbols[2].EffectiveSpelling {
		t.Fatalf("alpha-renaming is not deterministic: %#v %#v", first.Symbols, second.Symbols)
	}
	if first.Symbols[1].EffectiveSpelling == first.Symbols[2].EffectiveSpelling || first.Symbols[1].EffectiveSpelling == "x" || first.Symbols[2].EffectiveSpelling == "x" {
		t.Fatalf("sibling collision was not separated: %#v", first.Symbols)
	}
	shadow, err := ResolveScenario(spec, "nested-quasiquote")
	if err != nil {
		t.Fatal(err)
	}
	if shadow.Status != StatusClosed || shadow.Symbols[1].EffectiveSpelling == "x" {
		t.Fatalf("shadowed binder was not alpha-renamed: %#v", shadow.Symbols)
	}
}

func TestExplicitCaptureAndRefutations(t *testing.T) {
	spec := fixtureSpec(t)
	grant, err := ResolveScenario(spec, "explicit-intentional-capture")
	if err != nil {
		t.Fatal(err)
	}
	if grant.Status != StatusClosed || grant.References[0].CaptureDecision != spec.Semantics.CaptureJudgment.IntendedTarget || grant.References[0].GrantID != "grant.valid" {
		t.Fatalf("valid capture grant was not accepted: %#v", grant)
	}
	for _, id := range []string{"forged-capture-grant", "invalid-capability", "implicit-capture-counterexample", "fixed-binder-capture"} {
		report, resolveErr := ResolveScenario(spec, id)
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		if report.Status != StatusRefuted {
			t.Fatalf("case %s got %s, want REFUTED", id, report.Status)
		}
	}
}

func TestUnknownPreservesSixFields(t *testing.T) {
	spec := fixtureSpec(t)
	for _, id := range []string{"missing-origin", "ambiguous-stage", "missing-grant", "missing-expansion-origin"} {
		report, err := ResolveScenario(spec, id)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status != StatusUnknown || report.Unknown == nil {
			t.Fatalf("case %s got %#v, want UNKNOWN", id, report)
		}
		if err := report.Unknown.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmitterProducesStructuredCaptureFreeGo(t *testing.T) {
	spec := fixtureSpec(t)
	source, err := EmitExample(spec, "nested-quasiquote")
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

func TestHumanReportIsAvailable(t *testing.T) {
	spec := fixtureSpec(t)
	all, err := ResolveAll(spec)
	if err != nil {
		t.Fatal(err)
	}
	human, err := RenderHumanReport(all)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(human), "Case vector") || !strings.Contains(string(human), "UNKNOWN:") {
		t.Fatalf("human report is incomplete: %s", human)
	}
}
