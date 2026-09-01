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
	Scenarios          []Scenario          `json:"scenarios"`
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
	GenerationPlan               []string          `json:"generation_plan"`
	NoArbitraryStringReplacement bool              `json:"no_arbitrary_string_replacement"`
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

type Denominator struct {
	Scenarios   string `json:"scenarios"`
	Symbols     string `json:"symbols"`
	Improvement string `json:"improvement"`
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
}

type Reference struct {
	ID              string `json:"id"`
	Spelling        string `json:"spelling"`
	ReferenceOrigin string `json:"reference_origin"`
	ExpansionOrigin string `json:"expansion_origin"`
	Scope           string `json:"scope"`
	TargetSymbol    string `json:"target_symbol"`
	CapturePolicy   string `json:"capture_policy"`
}

type Scenario struct {
	ID                string          `json:"id"`
	Description       string          `json:"description"`
	RootScope         string          `json:"root_scope"`
	DeclaredSymbols   []string        `json:"declared_symbols"`
	IntroducedSymbols []string        `json:"introduced_symbols"`
	ReferenceSymbols  []string        `json:"reference_symbols"`
	ExpectedStatus    Status          `json:"expected_status"`
	Example           *ExampleProgram `json:"example,omitempty"`
	ReplayOf          string          `json:"replay_of,omitempty"`
	ExpectedReplay    bool            `json:"expected_replay,omitempty"`
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
	if s.Schema != "gooo.origin-resolver/v1" || s.Authority != "metacode" || s.ContractID == "" {
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
	if s.Semantics.CaptureJudgment.FreshCollision == "" || s.Semantics.CaptureJudgment.IntendedTarget == "" || s.Semantics.CaptureJudgment.NoCollision == "" || s.Semantics.CaptureJudgment.UnintendedTarget == "" || s.Semantics.CaptureJudgment.MissingOrigin == "" {
		return errors.New("capture judgments are incomplete")
	}
	if err := s.Semantics.Unknown.Validate(); err != nil {
		return err
	}
	if s.Semantics.Denominator.Scenarios == "" || s.Semantics.Denominator.Symbols == "" || s.Semantics.Denominator.Improvement == "" || len(s.Semantics.GenerationPlan) == 0 || !s.Semantics.NoArbitraryStringReplacement {
		return errors.New("denominator, generation plan, or no-replacement guard is incomplete")
	}
	if len(s.DeclarationOrigins) == 0 || len(s.ExpansionOrigins) == 0 || len(s.ScopeEdges) == 0 || len(s.IntroducedSymbols) == 0 || len(s.ReferenceSymbols) == 0 || len(s.Scenarios) == 0 {
		return errors.New("origin resolver contract is missing a required declaration")
	}
	declarations := map[string]bool{}
	for _, origin := range s.DeclarationOrigins {
		if origin.ID == "" || origin.Source == "" || origin.Line < 1 || origin.Column < 1 {
			return fmt.Errorf("invalid declaration origin %q", origin.ID)
		}
		declarations[origin.ID] = true
	}
	references := map[string]bool{}
	for _, origin := range s.ReferenceOrigins {
		if origin.ID == "" || origin.Source == "" || origin.Line < 1 || origin.Column < 1 {
			return fmt.Errorf("invalid reference origin %q", origin.ID)
		}
		references[origin.ID] = true
	}
	expansions := map[string]bool{}
	for _, origin := range s.ExpansionOrigins {
		if origin.ID == "" || origin.Macro == "" || origin.CallSite == "" {
			return fmt.Errorf("invalid expansion origin %q", origin.ID)
		}
		if origin.Parent != "" && !expansions[origin.Parent] {
			return fmt.Errorf("expansion %q refers to unknown parent %q", origin.ID, origin.Parent)
		}
		expansions[origin.ID] = true
	}
	for _, edge := range s.ScopeEdges {
		if edge.Child == "" || edge.Parent == "" || edge.Kind == "" {
			return errors.New("scope edge is incomplete")
		}
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
	}
	seenScenarios := map[string]bool{}
	for _, scenario := range s.Scenarios {
		if scenario.ID == "" || seenScenarios[scenario.ID] || scenario.RootScope == "" {
			return fmt.Errorf("invalid or duplicate scenario %q", scenario.ID)
		}
		seenScenarios[scenario.ID] = true
		if scenario.ExpectedStatus != StatusClosed && scenario.ExpectedStatus != StatusUnknown && scenario.ExpectedStatus != StatusRefuted {
			return fmt.Errorf("scenario %q has invalid expected status", scenario.ID)
		}
		for _, id := range append(append(append([]string{}, scenario.DeclaredSymbols...), scenario.IntroducedSymbols...), scenario.ReferenceSymbols...) {
			if !seenSymbols[id] && !seenReferences[id] {
				return fmt.Errorf("scenario %q refers to unknown item %q", scenario.ID, id)
			}
		}
		if scenario.ReplayOf != "" && !seenScenarios[scenario.ReplayOf] {
			return fmt.Errorf("scenario %q refers to replay scenario declared later or missing", scenario.ID)
		}
	}
	return nil
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

func (s Spec) scenarioMap() map[string]Scenario {
	items := make(map[string]Scenario, len(s.Scenarios))
	for _, scenario := range s.Scenarios {
		items[scenario.ID] = scenario
	}
	return items
}
