package originresolver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Status string

const (
	StatusClosed  Status = "CLOSED"
	StatusUnknown Status = "UNKNOWN"
	StatusRefuted Status = "REFUTED"
)

type Spec struct {
	Schema             string              `json:"schema"`
	Authority          string              `json:"authority"`
	ContractID         string              `json:"contract_id"`
	Toolchain          Toolchain           `json:"toolchain"`
	Semantics          Semantics           `json:"semantics"`
	DeclarationOrigins []DeclarationOrigin `json:"declaration_origins"`
	ReferenceOrigins   []ReferenceOrigin   `json:"reference_origins"`
	ExpansionOrigins   []ExpansionOrigin   `json:"expansion_origins"`
	ScopeEdges         []ScopeEdge         `json:"scope_edges"`
	DeclaredSymbols    []Symbol            `json:"declared_symbols"`
	IntroducedSymbols  []Symbol            `json:"introduced_symbols"`
	ReferenceSymbols   []Reference         `json:"reference_symbols"`
	CaptureGrants      []CaptureGrant      `json:"capture_grants"`
	Cases              []Scenario          `json:"cases"`
}

type Toolchain struct {
	Go string `json:"go"`
}

type Semantics struct {
	Identity                     IdentityRule      `json:"identity"`
	AlphaRenaming                AlphaRenamingRule `json:"alpha_renaming"`
	CaptureJudgment              CaptureJudgment   `json:"capture_judgment"`
	StatusPrecedence             []Status          `json:"status_precedence"`
	Unknown                      UnknownRule       `json:"unknown"`
	Denominator                  Denominator       `json:"denominator"`
	Bounds                       ExpansionBounds   `json:"bounds"`
	GenerationPlan               []string          `json:"generation_plan"`
	NoArbitraryStringReplacement bool              `json:"no_arbitrary_string_replacement"`
	NoAggregateScores            bool              `json:"no_aggregate_scores_or_percentages"`
}

type IdentityRule struct {
	Algorithm string   `json:"algorithm"`
	Inputs    []string `json:"inputs"`
	Delimiter string   `json:"delimiter"`
	Prefix    string   `json:"prefix"`
}

type AlphaRenamingRule struct {
	FreshPolicy   string `json:"fresh_policy"`
	Separator     string `json:"separator"`
	IdentityChars int    `json:"identity_chars"`
}

type CaptureJudgment struct {
	FreshCollision   string `json:"fresh_collision"`
	IntendedTarget   string `json:"intended_target"`
	NoCollision      string `json:"no_collision"`
	UnintendedTarget string `json:"unintended_target"`
	MissingOrigin    string `json:"missing_origin"`
	MissingStage     string `json:"missing_stage"`
	MissingGrant     string `json:"missing_grant"`
	ForgedGrant      string `json:"forged_grant"`
}

type UnknownRule struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

func (u UnknownRule) Validate() error {
	if u.Stage == "" || u.Step == "" || u.Reason == "" || u.UnknownClass == "" || u.NextOperation == "" || u.BlockedBy == "" {
		return errors.New("UNKNOWN must preserve stage, step, reason, unknown_class, next_operation, and blocked_by")
	}
	return nil
}

type ExpansionBounds struct {
	MaxNodes      int `json:"max_nodes"`
	MaxQuoteDepth int `json:"max_quote_depth"`
	MaxSplices    int `json:"max_splices"`
	MaxOriginHops int `json:"max_origin_hops"`
}

type Denominator struct {
	Cells          []DenominatorCell `json:"cells"`
	MetaActivities []string           `json:"meta_activities"`
	Proof          ProofLanes         `json:"proof"`
	Indicators     IndicatorLanes     `json:"indicators"`
	Improvement    string             `json:"improvement"`
}

type DenominatorCell struct {
	ID            string `json:"id"`
	MetaActivity  string `json:"meta_activity"`
	ProofLane     string `json:"proof_lane"`
	IndicatorLane string `json:"indicator_lane"`
}

type ProofLanes struct {
	Foundation []string `json:"FOUNDATION"`
	Coherence  []string `json:"COHERENCE"`
	Regression []string `json:"REGRESSION"`
}

type IndicatorLanes struct {
	Driver    []string `json:"DRIVER"`
	Outcome   []string `json:"OUTCOME"`
	Guardrail []string `json:"GUARDRAIL"`
}

type DeclarationOrigin struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Label  string `json:"label"`
}

type ReferenceOrigin struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Label  string `json:"label"`
}

type ExpansionOrigin struct {
	ID       string `json:"id"`
	Macro    string `json:"macro"`
	CallSite string `json:"call_site"`
	Parent   string `json:"parent"`
	Stage    *int   `json:"stage"`
}

type ScopeEdge struct {
	Child  string `json:"child"`
	Parent string `json:"parent"`
	Kind   string `json:"kind"`
}

type Symbol struct {
	ID                string `json:"id"`
	Spelling          string `json:"spelling"`
	Kind              string `json:"kind"`
	DeclarationOrigin string `json:"declaration_origin"`
	ExpansionOrigin   string `json:"expansion_origin"`
	Scope             string `json:"scope"`
	CapturePolicy     string `json:"capture_policy"`
	Stage             *int   `json:"stage"`
}

type Reference struct {
	ID              string `json:"id"`
	Spelling        string `json:"spelling"`
	ReferenceOrigin string `json:"reference_origin"`
	ExpansionOrigin string `json:"expansion_origin"`
	Scope           string `json:"scope"`
	TargetSymbol    string `json:"target_symbol"`
	CapturePolicy   string `json:"capture_policy"`
	CaptureGrant    string `json:"capture_grant,omitempty"`
	Stage           *int   `json:"stage"`
}

type CaptureGrant struct {
	ID              string `json:"id"`
	ReferenceSymbol string `json:"reference_symbol"`
	TargetSymbol    string `json:"target_symbol"`
	ExpansionOrigin string `json:"expansion_origin"`
	Stage           *int   `json:"stage"`
	Capability      string `json:"capability"`
	Evidence        string `json:"evidence"`
}

type SyntaxNode struct {
	Kind            string       `json:"kind"`
	Stage           *int         `json:"stage"`
	Origin          string       `json:"origin,omitempty"`
	ExpansionOrigin string       `json:"expansion_origin,omitempty"`
	Symbol          string       `json:"symbol,omitempty"`
	Reference       string       `json:"reference,omitempty"`
	Value           string       `json:"value,omitempty"`
	Children        []SyntaxNode `json:"children,omitempty"`
}

type Scenario struct {
	ID                string          `json:"id"`
	Description       string          `json:"description"`
	RootScope         string          `json:"root_scope"`
	DeclaredSymbols   []string        `json:"declared_symbols"`
	IntroducedSymbols []string        `json:"introduced_symbols"`
	ReferenceSymbols  []string        `json:"reference_symbols"`
	ExpectedStatus    Status          `json:"expected_status"`
	Form              SyntaxNode      `json:"form"`
	Example           *ExampleProgram `json:"example,omitempty"`
}

type ExampleProgram struct {
	Package         string        `json:"package"`
	Function        string        `json:"function"`
	Bindings        []ExampleBind `json:"bindings"`
	ReturnReference string        `json:"return_reference"`
}

type ExampleBind struct {
	Symbol string `json:"symbol"`
	Value  string `json:"value"`
}

type UnknownRecord struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

func (u UnknownRecord) Validate() error {
	if u.Stage == "" || u.Step == "" || u.Reason == "" || u.UnknownClass == "" || u.NextOperation == "" || u.BlockedBy == "" {
		return errors.New("UNKNOWN must preserve stage, step, reason, unknown_class, next_operation, and blocked_by")
	}
	return nil
}

func (s Spec) Validate() error {
	if s.Schema != "gooo.origin-resolver/v2" || s.Authority != "metacode" || s.ContractID == "" {
		return errors.New(".gooo origin resolver contract must be authoritative and versioned")
	}
	if s.Toolchain.Go != "1.27" {
		return fmt.Errorf("toolchain must be Go 1.27, got %q", s.Toolchain.Go)
	}
	if s.Semantics.Identity.Algorithm != "sha256" || !equal(s.Semantics.Identity.Inputs, []string{"declaration_origin", "expansion_origin", "symbol_id"}) || s.Semantics.Identity.Delimiter == "" || s.Semantics.Identity.Prefix == "" {
		return errors.New("identity semantics are incomplete")
	}
	if s.Semantics.AlphaRenaming.FreshPolicy != "fresh" || s.Semantics.AlphaRenaming.Separator == "" || s.Semantics.AlphaRenaming.IdentityChars < 8 {
		return errors.New("alpha-renaming semantics are incomplete")
	}
	if !equal(s.Semantics.StatusPrecedence, []Status{StatusRefuted, StatusUnknown, StatusClosed}) {
		return errors.New("status precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	judgment := s.Semantics.CaptureJudgment
	if judgment.FreshCollision == "" || judgment.IntendedTarget == "" || judgment.NoCollision == "" || judgment.UnintendedTarget == "" || judgment.MissingOrigin == "" || judgment.MissingStage == "" || judgment.MissingGrant == "" || judgment.ForgedGrant == "" {
		return errors.New("capture judgments are incomplete")
	}
	if err := s.Semantics.Unknown.Validate(); err != nil {
		return err
	}
	if s.Semantics.Bounds.MaxNodes < 1 || s.Semantics.Bounds.MaxQuoteDepth < 1 || s.Semantics.Bounds.MaxSplices < 1 || s.Semantics.Bounds.MaxOriginHops < 1 {
		return errors.New("bounded expansion limits are required")
	}
	if len(s.Semantics.GenerationPlan) != 7 || !s.Semantics.NoArbitraryStringReplacement || !s.Semantics.NoAggregateScores {
		return errors.New("generation plan or reporting guards are incomplete")
	}
	if err := s.Semantics.Denominator.Validate(); err != nil {
		return err
	}
	if len(s.DeclarationOrigins) == 0 || len(s.ExpansionOrigins) == 0 || len(s.ScopeEdges) == 0 || len(s.IntroducedSymbols) == 0 || len(s.ReferenceSymbols) == 0 || len(s.Cases) != 12 {
		return errors.New("origin resolver contract is missing a required declaration or case")
	}
	declarations := map[string]bool{}
	for _, origin := range s.DeclarationOrigins {
		if origin.ID == "" || origin.Source == "" || origin.Line < 1 || origin.Column < 1 {
			return fmt.Errorf("invalid declaration origin %q", origin.ID)
		}
		if declarations[origin.ID] {
			return fmt.Errorf("duplicate declaration origin %q", origin.ID)
		}
		declarations[origin.ID] = true
	}
	references := map[string]bool{}
	for _, origin := range s.ReferenceOrigins {
		if origin.ID == "" || origin.Source == "" || origin.Line < 1 || origin.Column < 1 {
			return fmt.Errorf("invalid reference origin %q", origin.ID)
		}
		if references[origin.ID] {
			return fmt.Errorf("duplicate reference origin %q", origin.ID)
		}
		references[origin.ID] = true
	}
	expansions := map[string]bool{}
	for _, origin := range s.ExpansionOrigins {
		if origin.ID == "" || origin.Macro == "" || origin.CallSite == "" {
			return fmt.Errorf("invalid expansion origin %q", origin.ID)
		}
		if expansions[origin.ID] {
			return fmt.Errorf("duplicate expansion origin %q", origin.ID)
		}
		if origin.Parent != "" && !expansions[origin.Parent] {
			return fmt.Errorf("expansion %q refers to unknown parent %q", origin.ID, origin.Parent)
		}
		expansions[origin.ID] = true
	}
	scopes := map[string]bool{"root": true}
	for _, edge := range s.ScopeEdges {
		if edge.Child == "" || edge.Parent == "" || edge.Kind == "" {
			return errors.New("scope edge is incomplete")
		}
		scopes[edge.Child] = true
		scopes[edge.Parent] = true
	}
	seenSymbols := map[string]bool{}
	for _, symbol := range append(append([]Symbol{}, s.DeclaredSymbols...), s.IntroducedSymbols...) {
		if seenSymbols[symbol.ID] || symbol.ID == "" || symbol.Spelling == "" || symbol.Kind == "" || symbol.Scope == "" || symbol.CapturePolicy == "" {
			return fmt.Errorf("invalid or duplicate symbol %q", symbol.ID)
		}
		seenSymbols[symbol.ID] = true
		if symbol.DeclarationOrigin != "" && !declarations[symbol.DeclarationOrigin] {
			return fmt.Errorf("symbol %q refers to unknown declaration origin", symbol.ID)
		}
		if symbol.ExpansionOrigin != "" && !expansions[symbol.ExpansionOrigin] {
			return fmt.Errorf("symbol %q refers to unknown expansion origin", symbol.ID)
		}
		if !scopes[symbol.Scope] {
			return fmt.Errorf("symbol %q refers to undeclared scope %q", symbol.ID, symbol.Scope)
		}
	}
	seenReferences := map[string]bool{}
	for _, reference := range s.ReferenceSymbols {
		if seenReferences[reference.ID] || reference.ID == "" || reference.Spelling == "" || reference.Scope == "" || reference.TargetSymbol == "" || reference.CapturePolicy == "" {
			return fmt.Errorf("invalid or duplicate reference %q", reference.ID)
		}
		seenReferences[reference.ID] = true
		if reference.ReferenceOrigin != "" && !references[reference.ReferenceOrigin] {
			return fmt.Errorf("reference %q refers to unknown reference origin", reference.ID)
		}
		if reference.ExpansionOrigin != "" && !expansions[reference.ExpansionOrigin] {
			return fmt.Errorf("reference %q refers to unknown expansion origin", reference.ID)
		}
		if !seenSymbols[reference.TargetSymbol] {
			return fmt.Errorf("reference %q targets unknown symbol %q", reference.ID, reference.TargetSymbol)
		}
		if !scopes[reference.Scope] {
			return fmt.Errorf("reference %q refers to undeclared scope %q", reference.ID, reference.Scope)
		}
	}
	seenGrants := map[string]bool{}
	for _, grant := range s.CaptureGrants {
		if grant.ID == "" || grant.ReferenceSymbol == "" || grant.TargetSymbol == "" || grant.ExpansionOrigin == "" || grant.Capability == "" || grant.Evidence == "" {
			return fmt.Errorf("capture grant %q is incomplete", grant.ID)
		}
		if seenGrants[grant.ID] {
			return fmt.Errorf("duplicate capture grant %q", grant.ID)
		}
		seenGrants[grant.ID] = true
		if !seenReferences[grant.ReferenceSymbol] {
			return fmt.Errorf("capture grant %q refers to unknown reference", grant.ID)
		}
		if !seenSymbols[grant.TargetSymbol] || !expansions[grant.ExpansionOrigin] {
			return fmt.Errorf("capture grant %q refers to an unknown target or expansion", grant.ID)
		}
	}
	caseCounts := map[Status]int{}
	seenCases := map[string]bool{}
	for _, scenario := range s.Cases {
		if scenario.ID == "" || seenCases[scenario.ID] || scenario.RootScope == "" {
			return fmt.Errorf("invalid or duplicate case %q", scenario.ID)
		}
		seenCases[scenario.ID] = true
		if scenario.ExpectedStatus != StatusClosed && scenario.ExpectedStatus != StatusUnknown && scenario.ExpectedStatus != StatusRefuted {
			return fmt.Errorf("case %q has invalid expected status", scenario.ID)
		}
		caseCounts[scenario.ExpectedStatus]++
		if !scopes[scenario.RootScope] {
			return fmt.Errorf("case %q refers to undeclared root scope", scenario.ID)
		}
		for _, id := range append(append(append([]string{}, scenario.DeclaredSymbols...), scenario.IntroducedSymbols...), scenario.ReferenceSymbols...) {
			if !seenSymbols[id] && !seenReferences[id] {
				return fmt.Errorf("case %q refers to unknown item %q", scenario.ID, id)
			}
		}
	}
	if caseCounts[StatusClosed] != 4 || caseCounts[StatusUnknown] != 4 || caseCounts[StatusRefuted] != 4 {
		return fmt.Errorf("case vector must be CLOSED/UNKNOWN/REFUTED 4/4/4")
	}
	for _, id := range []string{"nested-quasiquote", "two-splices", "shadowed-binder", "sibling-binder-collision", "explicit-intentional-capture", "missing-origin", "ambiguous-stage", "missing-grant", "missing-expansion-origin", "forged-capture-grant", "implicit-capture-counterexample", "fixed-binder-capture"} {
		if !seenCases[id] {
			return fmt.Errorf("missing required case %q", id)
		}
	}
	return nil
}

func (d Denominator) Validate() error {
	if len(d.Cells) != 12 || len(d.MetaActivities) != 12 || d.Improvement == "" {
		return errors.New("denominator must contain exactly 12 cells and 12 meta activities")
	}
	seenCells := map[string]bool{}
	seenActivities := map[string]bool{}
	for _, cell := range d.Cells {
		if cell.ID == "" || cell.MetaActivity == "" || cell.ProofLane == "" || cell.IndicatorLane == "" || seenCells[cell.ID] || seenActivities[cell.MetaActivity] {
			return errors.New("denominator cells and meta activities must be non-empty and unique")
		}
		seenCells[cell.ID] = true
		seenActivities[cell.MetaActivity] = true
	}
	for _, activity := range d.MetaActivities {
		if activity == "" || !seenActivities[activity] {
			return fmt.Errorf("meta activity %q is not a denominator cell", activity)
		}
	}
	if len(d.Proof.Foundation) != 4 || len(d.Proof.Coherence) != 4 || len(d.Proof.Regression) != 4 || len(d.Indicators.Driver) != 4 || len(d.Indicators.Outcome) != 4 || len(d.Indicators.Guardrail) != 4 {
		return errors.New("proof and indicator lanes must each contain 4 activities")
	}
	return validateLaneActivities(d, append(append(append([]string{}, d.Proof.Foundation...), d.Proof.Coherence...), d.Proof.Regression...), append(append(append([]string{}, d.Indicators.Driver...), d.Indicators.Outcome...), d.Indicators.Guardrail...))
}

func validateLaneActivities(d Denominator, proof, indicators []string) error {
	seenProof := map[string]bool{}
	for _, activity := range proof {
		if seenProof[activity] || !contains(d.MetaActivities, activity) {
			return fmt.Errorf("invalid or duplicate proof activity %q", activity)
		}
		seenProof[activity] = true
	}
	seenIndicators := map[string]bool{}
	for _, activity := range indicators {
		if seenIndicators[activity] || !contains(d.MetaActivities, activity) {
			return fmt.Errorf("invalid or duplicate indicator activity %q", activity)
		}
		seenIndicators[activity] = true
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func equal[T comparable](actual, expected []T) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func LoadSpec(path string) (Spec, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, nil, err
	}
	var spec Spec
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, nil, fmt.Errorf("parse .gooo origin resolver: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return Spec{}, nil, err
	}
	return spec, raw, nil
}

func (s Spec) declarationMap() map[string]Symbol {
	items := make(map[string]Symbol, len(s.DeclaredSymbols)+len(s.IntroducedSymbols))
	for _, symbol := range s.DeclaredSymbols {
		items[symbol.ID] = symbol
	}
	for _, symbol := range s.IntroducedSymbols {
		items[symbol.ID] = symbol
	}
	return items
}

func (s Spec) referenceMap() map[string]Reference {
	items := make(map[string]Reference, len(s.ReferenceSymbols))
	for _, reference := range s.ReferenceSymbols {
		items[reference.ID] = reference
	}
	return items
}

func (s Spec) grantMap() map[string]CaptureGrant {
	items := make(map[string]CaptureGrant, len(s.CaptureGrants))
	for _, grant := range s.CaptureGrants {
		items[grant.ID] = grant
	}
	return items
}

func (s Spec) declarationOriginMap() map[string]DeclarationOrigin {
	items := make(map[string]DeclarationOrigin, len(s.DeclarationOrigins))
	for _, origin := range s.DeclarationOrigins {
		items[origin.ID] = origin
	}
	return items
}

func (s Spec) referenceOriginMap() map[string]ReferenceOrigin {
	items := make(map[string]ReferenceOrigin, len(s.ReferenceOrigins))
	for _, origin := range s.ReferenceOrigins {
		items[origin.ID] = origin
	}
	return items
}

func (s Spec) expansionOriginMap() map[string]ExpansionOrigin {
	items := make(map[string]ExpansionOrigin, len(s.ExpansionOrigins))
	for _, origin := range s.ExpansionOrigins {
		items[origin.ID] = origin
	}
	return items
}

func (s Spec) caseMap() map[string]Scenario {
	items := make(map[string]Scenario, len(s.Cases))
	for _, scenario := range s.Cases {
		items[scenario.ID] = scenario
	}
	return items
}
