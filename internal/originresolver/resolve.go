package originresolver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type ProofStep struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type SymbolEvidence struct {
	ID                string      `json:"id"`
	OriginalSpelling  string      `json:"original_spelling"`
	EffectiveSpelling string      `json:"effective_spelling"`
	StableIdentity    string      `json:"stable_identity,omitempty"`
	CaptureDecision   string      `json:"capture_decision"`
	OriginProofPath   []ProofStep `json:"origin_proof_path"`
}

type ReferenceEvidence struct {
	ID                string      `json:"id"`
	Spelling          string      `json:"spelling"`
	EffectiveSpelling string      `json:"effective_spelling"`
	ExpectedTarget    string      `json:"expected_target"`
	ActualTarget      string      `json:"actual_target,omitempty"`
	CaptureDecision   string      `json:"capture_decision"`
	OriginProofPath   []ProofStep `json:"origin_proof_path"`
}

type ReplayEvidence struct {
	SourceScenario string `json:"source_scenario"`
	ExpectedSame   bool   `json:"expected_same"`
	SameIdentities bool   `json:"same_identities"`
	SameNames      bool   `json:"same_names"`
	SameDecisions  bool   `json:"same_decisions"`
	Status         Status `json:"status"`
}

type Report struct {
	Schema         string              `json:"schema"`
	Authority      string              `json:"authority"`
	ContractDigest string              `json:"contract_digest,omitempty"`
	Scenario       string              `json:"scenario"`
	ExpectedStatus Status              `json:"expected_status"`
	Status         Status              `json:"status"`
	Reason         string              `json:"reason,omitempty"`
	Unknown        *UnknownRecord      `json:"unknown,omitempty"`
	Symbols        []SymbolEvidence    `json:"symbols"`
	References     []ReferenceEvidence `json:"references"`
	Replay         *ReplayEvidence     `json:"replay,omitempty"`
}

type AllReports struct {
	Schema         string   `json:"schema"`
	Authority      string   `json:"authority"`
	ContractDigest string   `json:"contract_digest,omitempty"`
	Reports        []Report `json:"reports"`
}

type selectedGraph struct {
	symbols    map[string]Symbol
	ordered    []string
	introduced map[string]bool
	references map[string]Reference
	parents    map[string]string
	byScope    map[string][]string
	effective  map[string]string
	identities map[string]string
	decisions  map[string]string
}

func ResolveScenario(spec Spec, scenarioID string) (Report, error) {
	scenario, ok := spec.scenarioMap()[scenarioID]
	if !ok {
		return Report{}, fmt.Errorf("scenario %q is not declared", scenarioID)
	}
	graph, err := buildGraph(spec, scenario)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Schema:         "gooo.origin-resolve/v1",
		Authority:      spec.Authority,
		Scenario:       scenario.ID,
		ExpectedStatus: scenario.ExpectedStatus,
		Status:         StatusClosed,
		Symbols:        make([]SymbolEvidence, 0, len(graph.ordered)),
		References:     make([]ReferenceEvidence, 0, len(scenario.ReferenceSymbols)),
	}
	for _, symbolID := range graph.ordered {
		symbol := graph.symbols[symbolID]
		proof, complete, proofErr := symbolProof(spec, symbol, graph.parents)
		if proofErr != nil {
			return Report{}, proofErr
		}
		evidence := SymbolEvidence{
			ID:                symbol.ID,
			OriginalSpelling:  symbol.Spelling,
			EffectiveSpelling: symbol.Spelling,
			CaptureDecision:   spec.Semantics.CaptureJudgment.NoCollision,
			OriginProofPath:   proof,
		}
		if complete {
			identity := stableIdentity(spec, symbol)
			graph.identities[symbol.ID] = identity
			evidence.StableIdentity = identity
		} else {
			graph.decisions[symbol.ID] = spec.Semantics.CaptureJudgment.MissingOrigin
			evidence.CaptureDecision = spec.Semantics.CaptureJudgment.MissingOrigin
			raise(&report, spec, StatusUnknown, "origin proof is incomplete")
		}
		if graph.introduced[symbol.ID] {
			if symbol.CapturePolicy == spec.Semantics.AlphaRenaming.FreshPolicy {
				collision, collisionErr := hasVisibleCollision(symbol.ID, graph)
				if collisionErr != nil {
					return Report{}, collisionErr
				}
				if collision && complete {
					evidence.EffectiveSpelling = alphaName(spec, symbol, graph.identities[symbol.ID], graph)
					evidence.CaptureDecision = spec.Semantics.CaptureJudgment.FreshCollision
				} else if collision {
					evidence.CaptureDecision = spec.Semantics.CaptureJudgment.MissingOrigin
					raise(&report, spec, StatusUnknown, "collision cannot be alpha-renamed without a stable identity")
				}
			} else if hasVisibleCollisionWithoutRename(symbol.ID, graph) {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.UnintendedTarget
			}
		}
		graph.effective[symbol.ID] = evidence.EffectiveSpelling
		graph.decisions[symbol.ID] = evidence.CaptureDecision
		report.Symbols = append(report.Symbols, evidence)
	}
	for _, referenceID := range scenario.ReferenceSymbols {
		reference := graph.references[referenceID]
		proof, complete, proofErr := referenceProof(spec, reference, graph)
		if proofErr != nil {
			return Report{}, proofErr
		}
		evidence := ReferenceEvidence{
			ID:                reference.ID,
			Spelling:          reference.Spelling,
			EffectiveSpelling: reference.Spelling,
			ExpectedTarget:    reference.TargetSymbol,
			CaptureDecision:   spec.Semantics.CaptureJudgment.NoCollision,
			OriginProofPath:   proof,
		}
		if !complete {
			evidence.CaptureDecision = spec.Semantics.CaptureJudgment.MissingOrigin
			raise(&report, spec, StatusUnknown, "reference origin proof is incomplete")
		} else if target, ok := graph.symbols[reference.TargetSymbol]; !ok {
			return Report{}, fmt.Errorf("reference %q targets a symbol outside scenario %q", reference.ID, scenario.ID)
		} else {
			if graph.introduced[target.ID] && target.CapturePolicy == spec.Semantics.AlphaRenaming.FreshPolicy {
				evidence.EffectiveSpelling = graph.effective[target.ID]
			}
			actual, actualErr := resolveReference(reference, graph, evidence.EffectiveSpelling)
			if actualErr != nil {
				return Report{}, actualErr
			}
			evidence.ActualTarget = actual
			if actual != reference.TargetSymbol {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.UnintendedTarget
				raise(&report, spec, StatusRefuted, fmt.Sprintf("reference %q resolves to %q instead of %q", reference.ID, actual, reference.TargetSymbol))
			} else if reference.CapturePolicy == "intentional" && !graph.introduced[target.ID] {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.IntendedTarget
			} else if graph.introduced[target.ID] && graph.effective[target.ID] != target.Spelling {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.FreshCollision
			}
		}
		report.References = append(report.References, evidence)
	}
	if report.Status == StatusClosed {
		report.Reason = "all selected symbol origins and reference targets are proved"
	}
	return report, nil
}

func ResolveAll(spec Spec) (AllReports, error) {
	all := AllReports{Schema: "gooo.origin-resolve-set/v1", Authority: spec.Authority, Reports: make([]Report, 0, len(spec.Scenarios))}
	indexes := map[string]int{}
	for _, scenario := range spec.Scenarios {
		report, err := ResolveScenario(spec, scenario.ID)
		if err != nil {
			return AllReports{}, err
		}
		indexes[scenario.ID] = len(all.Reports)
		all.Reports = append(all.Reports, report)
	}
	for index := range all.Reports {
		scenario, _ := spec.scenarioMap()[all.Reports[index].Scenario]
		if scenario.ReplayOf == "" {
			continue
		}
		baseIndex, ok := indexes[scenario.ReplayOf]
		if !ok {
			return AllReports{}, fmt.Errorf("replay source %q is unavailable", scenario.ReplayOf)
		}
		base := all.Reports[baseIndex]
		current := &all.Reports[index]
		sameIdentities, sameNames, sameDecisions := compareReplay(base, *current)
		replayStatus := StatusClosed
		if !sameIdentities || !sameNames || !sameDecisions {
			replayStatus = StatusRefuted
			current.Status = StatusRefuted
			current.Reason = "replay changed stable identity, effective name, or capture decision"
		}
		current.Replay = &ReplayEvidence{
			SourceScenario: scenario.ReplayOf,
			ExpectedSame:   scenario.ExpectedReplay,
			SameIdentities: sameIdentities,
			SameNames:      sameNames,
			SameDecisions:  sameDecisions,
			Status:         replayStatus,
		}
	}
	for _, report := range all.Reports {
		if report.Status != report.ExpectedStatus {
			return AllReports{}, fmt.Errorf("scenario %q got %s, contract expects %s", report.Scenario, report.Status, report.ExpectedStatus)
		}
	}
	return all, nil
}

func buildGraph(spec Spec, scenario Scenario) (selectedGraph, error) {
	all := spec.declarationMap()
	refs := spec.referenceMap()
	graph := selectedGraph{
		symbols:    map[string]Symbol{},
		ordered:    make([]string, 0, len(scenario.DeclaredSymbols)+len(scenario.IntroducedSymbols)),
		introduced: map[string]bool{},
		references: map[string]Reference{},
		parents:    map[string]string{},
		byScope:    map[string][]string{},
		effective:  map[string]string{},
		identities: map[string]string{},
		decisions:  map[string]string{},
	}
	for _, edge := range spec.ScopeEdges {
		if _, exists := graph.parents[edge.Child]; exists {
			return selectedGraph{}, fmt.Errorf("scope %q has more than one parent", edge.Child)
		}
		graph.parents[edge.Child] = edge.Parent
	}
	for _, symbolID := range append(append([]string{}, scenario.DeclaredSymbols...), scenario.IntroducedSymbols...) {
		if _, exists := graph.symbols[symbolID]; exists {
			return selectedGraph{}, fmt.Errorf("symbol %q is selected twice", symbolID)
		}
		symbol, ok := all[symbolID]
		if !ok {
			return selectedGraph{}, fmt.Errorf("scenario %q selects unknown symbol %q", scenario.ID, symbolID)
		}
		graph.symbols[symbolID] = symbol
		graph.ordered = append(graph.ordered, symbolID)
		graph.byScope[symbol.Scope] = append(graph.byScope[symbol.Scope], symbolID)
	}
	for _, symbolID := range scenario.IntroducedSymbols {
		graph.introduced[symbolID] = true
	}
	for _, referenceID := range scenario.ReferenceSymbols {
		reference, ok := refs[referenceID]
		if !ok {
			return selectedGraph{}, fmt.Errorf("scenario %q selects unknown reference %q", scenario.ID, referenceID)
		}
		graph.references[referenceID] = reference
	}
	if _, err := scopeChain(scenario.RootScope, graph.parents); err != nil {
		return selectedGraph{}, err
	}
	return graph, nil
}

func symbolProof(spec Spec, symbol Symbol, parents map[string]string) ([]ProofStep, bool, error) {
	proof := make([]ProofStep, 0, 8)
	complete := true
	declarationOrigins := spec.declarationOriginMap()
	if symbol.DeclarationOrigin == "" {
		complete = false
		proof = append(proof, ProofStep{Kind: "missing-declaration-origin", ID: symbol.ID})
	} else {
		origin := declarationOrigins[symbol.DeclarationOrigin]
		proof = append(proof, ProofStep{Kind: "declaration-origin", ID: origin.ID, Detail: fmt.Sprintf("%s:%d:%d", origin.Source, origin.Line, origin.Column)})
	}
	expansionProof, expansionComplete, err := expansionProof(spec, symbol.ExpansionOrigin)
	if err != nil {
		return nil, false, err
	}
	proof = append(proof, expansionProof...)
	complete = complete && expansionComplete
	scopeProof, scopeErr := scopeProof(symbol.Scope, parents)
	if scopeErr != nil {
		return nil, false, scopeErr
	}
	proof = append(proof, scopeProof...)
	return proof, complete, nil
}

func referenceProof(spec Spec, reference Reference, graph selectedGraph) ([]ProofStep, bool, error) {
	proof := make([]ProofStep, 0, 8)
	complete := true
	if reference.ReferenceOrigin == "" {
		complete = false
		proof = append(proof, ProofStep{Kind: "missing-reference-origin", ID: reference.ID})
	} else if origin, ok := spec.referenceOriginMap()[reference.ReferenceOrigin]; ok {
		proof = append(proof, ProofStep{Kind: "reference-origin", ID: origin.ID, Detail: fmt.Sprintf("%s:%d:%d", origin.Source, origin.Line, origin.Column)})
	} else {
		return nil, false, fmt.Errorf("reference %q has an undeclared reference origin", reference.ID)
	}
	expansionProof, expansionComplete, err := expansionProof(spec, reference.ExpansionOrigin)
	if err != nil {
		return nil, false, err
	}
	proof = append(proof, expansionProof...)
	complete = complete && expansionComplete
	scopeProof, scopeErr := scopeProof(reference.Scope, graph.parents)
	if scopeErr != nil {
		return nil, false, scopeErr
	}
	proof = append(proof, scopeProof...)
	if target, ok := graph.symbols[reference.TargetSymbol]; ok {
		if identity := graph.identities[target.ID]; identity != "" {
			proof = append(proof, ProofStep{Kind: "target-symbol-identity", ID: target.ID, Detail: identity})
		} else {
			proof = append(proof, ProofStep{Kind: "target-symbol", ID: target.ID})
		}
	}
	return proof, complete, nil
}

func expansionProof(spec Spec, expansionID string) ([]ProofStep, bool, error) {
	if expansionID == "" {
		return []ProofStep{{Kind: "missing-expansion-origin"}}, false, nil
	}
	items := spec.expansionOriginMap()
	proof := make([]ProofStep, 0, 4)
	seen := map[string]bool{}
	complete := true
	for expansionID != "" {
		if seen[expansionID] {
			return nil, false, errors.New("expansion origin graph contains a cycle")
		}
		seen[expansionID] = true
		origin, ok := items[expansionID]
		if !ok {
			proof = append(proof, ProofStep{Kind: "missing-expansion-origin", ID: expansionID})
			complete = false
			break
		}
		proof = append(proof, ProofStep{Kind: "expansion-origin", ID: origin.ID, Detail: origin.Macro + "@" + origin.CallSite})
		expansionID = origin.Parent
	}
	return proof, complete, nil
}

func scopeProof(scope string, parents map[string]string) ([]ProofStep, error) {
	chain, err := scopeChain(scope, parents)
	if err != nil {
		return nil, err
	}
	proof := make([]ProofStep, 0, len(chain))
	for index := 0; index+1 < len(chain); index++ {
		proof = append(proof, ProofStep{Kind: "scope-edge", From: chain[index], To: chain[index+1]})
	}
	return proof, nil
}

func scopeChain(scope string, parents map[string]string) ([]string, error) {
	if scope == "" {
		return nil, errors.New("scope is empty")
	}
	chain := []string{}
	seen := map[string]bool{}
	for scope != "" {
		if seen[scope] {
			return nil, fmt.Errorf("scope graph contains a cycle at %q", scope)
		}
		seen[scope] = true
		chain = append(chain, scope)
		scope = parents[scope]
	}
	return chain, nil
}

func hasVisibleCollision(symbolID string, graph selectedGraph) (bool, error) {
	symbol := graph.symbols[symbolID]
	chain, err := scopeChain(symbol.Scope, graph.parents)
	if err != nil {
		return false, err
	}
	visible := map[string]bool{}
	for _, scope := range chain {
		visible[scope] = true
	}
	for _, otherID := range graph.ordered {
		if otherID == symbolID {
			continue
		}
		other := graph.symbols[otherID]
		if other.Spelling == symbol.Spelling && visible[other.Scope] {
			return true, nil
		}
	}
	return false, nil
}

func hasVisibleCollisionWithoutRename(symbolID string, graph selectedGraph) bool {
	symbol := graph.symbols[symbolID]
	chain, err := scopeChain(symbol.Scope, graph.parents)
	if err != nil {
		return false
	}
	visible := map[string]bool{}
	for _, scope := range chain {
		visible[scope] = true
	}
	for _, otherID := range graph.ordered {
		if otherID != symbolID && graph.symbols[otherID].Spelling == symbol.Spelling && visible[graph.symbols[otherID].Scope] {
			return true
		}
	}
	return false
}

func alphaName(spec Spec, symbol Symbol, identity string, graph selectedGraph) string {
	chars := spec.Semantics.AlphaRenaming.IdentityChars
	if chars > len(identity) {
		chars = len(identity)
	}
	candidate := symbol.Spelling + spec.Semantics.AlphaRenaming.Separator + identity[:chars]
	used := map[string]bool{}
	for _, name := range graph.effective {
		used[name] = true
	}
	if !used[candidate] {
		return candidate
	}
	for suffix := 1; ; suffix++ {
		alternative := fmt.Sprintf("%s_%d", candidate, suffix)
		if !used[alternative] {
			return alternative
		}
	}
}

func resolveReference(reference Reference, graph selectedGraph, spelling string) (string, error) {
	chain, err := scopeChain(reference.Scope, graph.parents)
	if err != nil {
		return "", err
	}
	for _, scope := range chain {
		for _, symbolID := range graph.byScope[scope] {
			if graph.effective[symbolID] == spelling {
				return symbolID, nil
			}
		}
	}
	return "", nil
}

func stableIdentity(spec Spec, symbol Symbol) string {
	payload := strings.Join([]string{symbol.DeclarationOrigin, symbol.ExpansionOrigin, symbol.ID}, spec.Semantics.Identity.Delimiter)
	sum := sha256.Sum256([]byte(payload))
	return spec.Semantics.Identity.Prefix + hex.EncodeToString(sum[:])
}

func unknownFrom(spec Spec) *UnknownRecord {
	unknown := UnknownRecord{
		Stage:         spec.Semantics.Unknown.Stage,
		Step:          spec.Semantics.Unknown.Step,
		Reason:        spec.Semantics.Unknown.Reason,
		UnknownClass:  spec.Semantics.Unknown.UnknownClass,
		NextOperation: spec.Semantics.Unknown.NextOperation,
		BlockedBy:     spec.Semantics.Unknown.BlockedBy,
	}
	return &unknown
}

func raise(report *Report, spec Spec, candidate Status, reason string) {
	if rank(spec.Semantics.StatusPrecedence, candidate) > rank(spec.Semantics.StatusPrecedence, report.Status) {
		report.Status = candidate
		report.Reason = reason
	}
	if candidate == StatusUnknown && report.Unknown == nil {
		report.Unknown = unknownFrom(spec)
	}
}

func rank(precedence []Status, status Status) int {
	for index, candidate := range precedence {
		if candidate == status {
			return len(precedence) - index
		}
	}
	return 0
}

func compareReplay(base, current Report) (bool, bool, bool) {
	identities := make([]string, 0, len(base.Symbols))
	currentIdentities := make([]string, 0, len(current.Symbols))
	names := make([]string, 0, len(base.Symbols))
	currentNames := make([]string, 0, len(current.Symbols))
	decisions := make([]string, 0, len(base.Symbols)+len(base.References))
	currentDecisions := make([]string, 0, len(current.Symbols)+len(current.References))
	for _, symbol := range base.Symbols {
		identities = append(identities, symbol.ID+"="+symbol.StableIdentity)
		names = append(names, symbol.ID+"="+symbol.EffectiveSpelling)
		decisions = append(decisions, symbol.ID+"="+symbol.CaptureDecision)
	}
	for _, symbol := range current.Symbols {
		currentIdentities = append(currentIdentities, symbol.ID+"="+symbol.StableIdentity)
		currentNames = append(currentNames, symbol.ID+"="+symbol.EffectiveSpelling)
		currentDecisions = append(currentDecisions, symbol.ID+"="+symbol.CaptureDecision)
	}
	for _, reference := range base.References {
		decisions = append(decisions, reference.ID+"="+reference.CaptureDecision)
	}
	for _, reference := range current.References {
		currentDecisions = append(currentDecisions, reference.ID+"="+reference.CaptureDecision)
	}
	return reflect.DeepEqual(identities, currentIdentities), reflect.DeepEqual(names, currentNames), reflect.DeepEqual(decisions, currentDecisions)
}

func EncodeReport(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
