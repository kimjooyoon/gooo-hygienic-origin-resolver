package originresolver

import (
	"fmt"
	"strings"
)

func RenderHumanReport(value any) ([]byte, error) {
	var report strings.Builder
	report.WriteString("# Gooo staged quasiquote expansion report\n\n")
	report.WriteString("The `.gooo` contract is the semantic authority. Go parses the contract, evaluates the bounded static judgment, and emits the generated Go/IR view. This report makes no global hygiene claim.\n\n")
	switch value := value.(type) {
	case AllReports:
		fmt.Fprintf(&report, "Case vector: %d cases; CLOSED %d, UNKNOWN %d, REFUTED %d.\n\n", value.Vector.Cases, value.Vector.Closed, value.Vector.Unknown, value.Vector.Refuted)
		for _, item := range value.Reports {
			writeHumanCase(&report, item)
		}
	case Report:
		writeHumanCase(&report, value)
	default:
		return nil, fmt.Errorf("unsupported report value %T", value)
	}
	return []byte(report.String()), nil
}

func writeHumanCase(report *strings.Builder, value Report) {
	fmt.Fprintf(report, "## `%s` — %s\n\n", value.Scenario, value.Status)
	fmt.Fprintf(report, "%s\n\n", value.Reason)
	if value.Unknown != nil {
		fmt.Fprintf(report, "UNKNOWN: stage=%s; step=%s; reason=%s; unknown_class=%s; next_operation=%s; blocked_by=%s.\n\n", value.Unknown.Stage, value.Unknown.Step, value.Unknown.Reason, value.Unknown.UnknownClass, value.Unknown.NextOperation, value.Unknown.BlockedBy)
	}
	for _, symbol := range value.Symbols {
		fmt.Fprintf(report, "- binder `%s`: `%s` → `%s`, decision `%s`, proof steps %d\n", symbol.ID, symbol.OriginalSpelling, symbol.EffectiveSpelling, symbol.CaptureDecision, len(symbol.OriginProofPath))
	}
	for _, reference := range value.References {
		fmt.Fprintf(report, "- reference `%s`: `%s` → `%s`, expected `%s`, decision `%s`\n", reference.ID, reference.Spelling, reference.ActualTarget, reference.ExpectedTarget, reference.CaptureDecision)
	}
	fmt.Fprintf(report, "- IR nodes: %d\n\n", len(value.IR))
}
