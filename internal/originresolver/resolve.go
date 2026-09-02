package originresolver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ProofStep struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Stage  *int   `json:"stage,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type SymbolEvidence struct {
	ID                string        `json:"id"`
	Scope             string        `json:"scope"`
	Stage             *int          `json:"stage"`
	OriginalSpelling  string        `json:"original_spelling"`
	EffectiveSpelling string        `json:"effective_spelling"`
	StableIdentity    string        `json:"stable_identity,omitempty"`
	CaptureDecision   string        `json:"capture_decision"`
	OriginProofPath   []ProofStep   `json:"origin_proof_path"`
	Unknown           *UnknownRecord `json:"unknown,omitempty"`
}

type ReferenceEvidence struct {
	ID                string        `json:"id"`
	Scope             string        `json:"scope"`
	Stage             *int          `json:"stage"`
	Spelling          string        `json:"spelling"`
	EffectiveSpelling string        `json:"effective_spelling"`
	ExpectedTarget    string        `json:"expected_target"`
	ActualTarget      string        `json:"actual_target,omitempty"`
	CaptureDecision   string        `json:"capture_decision"`
	GrantID           string        `json:"grant_id,omitempty"`
	Capability        string        `json:"capability,omitempty"`
	OriginProofPath   []ProofStep   `json:"origin_proof_path"`
	Unknown           *UnknownRecord `json:"unknown,omitempty"`
}

type IRNode struct {
	Kind         string      `json:"kind"`
	Stage        int         `json:"stage"`
	SymbolID     string      `json:"symbol_id,omitempty"`
	ReferenceID  string      `json:"reference_id,omitempty"`
	Spelling     string      `json:"spelling,omitempty"`
	Value        string      `json:"value,omitempty"`
	OriginStack  []ProofStep `json:"origin_stack"`
	Children     []IRNode    `json:"children,omitempty"`
}

type CaseVector struct {
	Cases   int `json:"cases"`
	Closed  int `json:"closed"`
	Unknown int `json:"unknown"`
	Refuted int `json:"refuted"`
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
	IR             []IRNode            `json:"ir"`
}

type AllReports struct {
	Schema         string      `json:"schema"`
	Authority      string      `json:"authority"`
	ContractDigest string      `json:"contract_digest,omitempty"`
	Vector         CaseVector  `json:"vector"`
	Reports        []Report    `json:"reports"`
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
	scenario, ok := spec.caseMap()[scenarioID]
	if !ok {
		return Report{}, fmt.Errorf("case %q is not declared", scenarioID)
	}
	graph, err := buildGraph(spec, scenario)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Schema:         "gooo.origin-resolve/v2",
		Authority:      spec.Authority,
		Scenario:       scenario.ID,
		ExpectedStatus: scenario.ExpectedStatus,
		Status:         StatusClosed,
		Symbols:        make([]SymbolEvidence, 0, len(graph.ordered)),
		References:     make([]ReferenceEvidence, 0, len(scenario.ReferenceSymbols)),
		IR:             make([]IRNode, 0, 1),
	}
	if err := assignNames(spec, graph, &report); err != nil {
		return Report{}, err
	}
	for _, referenceID := range scenario.ReferenceSymbols {
		reference := graph.references[referenceID]
		proof, complete, unknown, proofErr := referenceProof(spec, reference, graph)
		if proofErr != nil {
			return Report{}, proofErr
		}
		evidence := ReferenceEvidence{
			ID:                reference.ID,
			Scope:             reference.Scope,
			Stage:             reference.Stage,
			Spelling:          reference.Spelling,
			EffectiveSpelling: reference.Spelling,
			ExpectedTarget:    reference.TargetSymbol,
			CaptureDecision:   spec.Semantics.CaptureJudgment.NoCollision,
			OriginProofPath:   proof,
		}
		if !complete {
			evidence.CaptureDecision = unknownDecision(spec, unknown, spec.Semantics.CaptureJudgment.MissingOrigin)
			evidence.Unknown = unknown
			raise(&report, StatusUnknown, unknownReason(unknown, "reference proof is incomplete"), unknown)
		} else {
			target, targetOK := graph.symbols[reference.TargetSymbol]
			if !targetOK {
				return Report{}, fmt.Errorf("reference %q targets a symbol outside case %q", reference.ID, scenario.ID)
			}
			if graph.introduced[target.ID] {
				evidence.EffectiveSpelling = graph.effective[target.ID]
			}
			actual, actualErr := resolveReference(reference, graph, evidence.EffectiveSpelling)
			if actualErr != nil {
				return Report{}, actualErr
			}
			evidence.ActualTarget = actual
			if actual != reference.TargetSymbol {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.UnintendedTarget
				raise(&report, StatusRefuted, fmt.Sprintf("reference %q resolved to %q instead of %q", reference.ID, actual, reference.TargetSymbol), nil)
			}
			grantStatus, grant, grantUnknown := captureGrantStatus(spec, graph, reference)
			if grant != nil {
				evidence.GrantID = grant.ID
				evidence.Capability = grant.Capability
			}
			switch grantStatus {
			case "valid":
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.IntendedTarget
			case "missing":
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.MissingGrant
				evidence.Unknown = grantUnknown
				raise(&report, StatusUnknown, unknownReason(grantUnknown, "explicit capture grant is missing"), grantUnknown)
			case "forged":
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.ForgedGrant
				raise(&report, StatusRefuted, "capture grant failed identity, stage, capability, or evidence validation", nil)
			}
			if reference.CapturePolicy != "intentional" && !graph.introduced[target.ID] && actual == target.ID {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.UnintendedTarget
				raise(&report, StatusRefuted, "implicit capture of a caller binding is forbidden", nil)
			}
			if evidence.CaptureDecision == spec.Semantics.CaptureJudgment.NoCollision && graph.introduced[target.ID] && graph.effective[target.ID] != target.Spelling {
				evidence.CaptureDecision = spec.Semantics.CaptureJudgment.FreshCollision
			}
		}
		report.References = append(report.References, evidence)
	}
	if report.IR, err = expandSyntax(spec, scenario.Form, 0, graph, &report, 0, 0); err != nil {
		return Report{}, err
	}
	if report.Status == StatusClosed {
		report.Reason = "bounded staged expansion, origin stack, capture capability, and target resolution are proved"
	}
	return report, nil
}

func ResolveAll(spec Spec) (AllReports, error) {
	all := AllReports{
		Schema:    "gooo.origin-resolve-set/v2",
		Authority: spec.Authority,
		Reports:   make([]Report, 0, len(spec.Cases)),
	}
	for _, scenario := range spec.Cases {
		report, err := ResolveScenario(spec, scenario.ID)
		if err != nil {
			return AllReports{}, err
		}
		if report.Status != scenario.ExpectedStatus {
			return AllReports{}, fmt.Errorf("case %q got %s, contract expects %s", scenario.ID, report.Status, scenario.ExpectedStatus)
		}
		all.Reports = append(all.Reports, report)
		switch report.Status {
		case StatusClosed:
			all.Vector.Closed++
		case StatusUnknown:
			all.Vector.Unknown++
		case StatusRefuted:
			all.Vector.Refuted++
		}
	}
	all.Vector.Cases = len(all.Reports)
	if all.Vector.Closed != 4 || all.Vector.Unknown != 4 || all.Vector.Refuted != 4 {
		return AllReports{}, fmt.Errorf("resolved case vector must be CLOSED/UNKNOWN/REFUTED 4/4/4")
	}
	return all, nil
}

func buildGraph(spec Spec, scenario Scenario) (selectedGraph, error) {
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
	allSymbols := spec.declarationMap()
	for _, symbolID := range append(append([]string{}, scenario.DeclaredSymbols...), scenario.IntroducedSymbols...) {
		if _, exists := graph.symbols[symbolID]; exists {
			return selectedGraph{}, fmt.Errorf("symbol %q is selected twice", symbolID)
		}
		symbol, ok := allSymbols[symbolID]
		if !ok {
			return selectedGraph{}, fmt.Errorf("case %q selects unknown symbol %q", scenario.ID, symbolID)
		}
		graph.symbols[symbolID] = symbol
		graph.ordered = append(graph.ordered, symbolID)
		graph.byScope[symbol.Scope] = append(graph.byScope[symbol.Scope], symbolID)
	}
	for _, symbolID := range scenario.IntroducedSymbols {
		graph.introduced[symbolID] = true
	}
	allReferences := spec.referenceMap()
	for _, referenceID := range scenario.ReferenceSymbols {
		reference, ok := allReferences[referenceID]
		if !ok {
			return selectedGraph{}, fmt.Errorf("case %q selects unknown reference %q", scenario.ID, referenceID)
		}
		graph.references[referenceID] = reference
	}
	if _, err := scopeChain(scenario.RootScope, graph.parents); err != nil {
		return selectedGraph{}, err
	}
	return graph, nil
}

func assignNames(spec Spec, graph selectedGraph, report *Report) error {
	type candidate struct {
		id       string
		identity string
	}
	for _, symbolID := range graph.ordered {
		symbol := graph.symbols[symbolID]
		proof, complete, unknown, err := symbolProof(spec, symbol, graph.parents)
		if err != nil {
			return err
		}
		evidence := SymbolEvidence{
			ID:                symbol.ID,
			Scope:             symbol.Scope,
			Stage:             symbol.Stage,
			OriginalSpelling:  symbol.Spelling,
			EffectiveSpelling: symbol.Spelling,
			CaptureDecision:   spec.Semantics.CaptureJudgment.NoCollision,
			OriginProofPath:   proof,
			Unknown:           unknown,
		}
		if !complete {
			evidence.CaptureDecision = unknownDecision(spec, unknown, spec.Semantics.CaptureJudgment.MissingOrigin)
			raise(report, StatusUnknown, unknownReason(unknown, "symbol origin proof is incomplete"), unknown)
		} else {
			identity := stableIdentity(spec, symbol)
			graph.identities[symbol.ID] = identity
			evidence.StableIdentity = identity
		}
		graph.effective[symbol.ID] = symbol.Spelling
		graph.decisions[symbol.ID] = evidence.CaptureDecision
		report.Symbols = append(report.Symbols, evidence)
	}

	used := map[string]bool{}
	for _, symbolID := range graph.ordered {
		if !graph.introduced[symbolID] {
			used[graph.effective[symbolID]] = true
		}
	}
	candidates := make([]candidate, 0, len(graph.ordered))
	for _, symbolID := range graph.ordered {
		if graph.introduced[symbolID] && graph.identities[symbolID] != "" {
			candidates = append(candidates, candidate{id: symbolID, identity: graph.identities[symbolID]})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].identity == candidates[j].identity {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].identity < candidates[j].identity
	})
	for _, item := range candidates {
		symbol := graph.symbols[item.id]
		collision, err := hasVisibleCollision(item.id, graph)
		if err != nil {
			return err
		}
		if !collision {
			continue
		}
		name := alphaName(spec, symbol, item.identity, used)
		graph.effective[item.id] = name
		used[name] = true
		for index := range report.Symbols {
			if report.Symbols[index].ID == item.id {
				report.Symbols[index].EffectiveSpelling = name
				report.Symbols[index].CaptureDecision = spec.Semantics.CaptureJudgment.FreshCollision
				graph.decisions[item.id] = report.Symbols[index].CaptureDecision
			}
		}
	}
	return nil
}

func symbolProof(spec Spec, symbol Symbol, parents map[string]string) ([]ProofStep, bool, *UnknownRecord, error) {
	proof := make([]ProofStep, 0, 8)
	if symbol.DeclarationOrigin == "" {
		unknown := unknownFor(spec, "resolve", "prove-declaration-origin", "declaration origin is missing", "missing-origin", "supply declaration origin and replay the case", "declaration-origin-proof")
		proof = append(proof, ProofStep{Kind: "missing-declaration-origin", ID: symbol.ID})
		return proof, false, unknown, nil
	}
	origin, ok := spec.declarationOriginMap()[symbol.DeclarationOrigin]
	if !ok {
		return nil, false, nil, fmt.Errorf("symbol %q has undeclared declaration origin", symbol.ID)
	}
	proof = append(proof, ProofStep{Kind: "declaration-origin", ID: origin.ID, Detail: fmt.Sprintf("%s:%d:%d", origin.Source, origin.Line, origin.Column)})
	expansionProof, complete, unknown, err := expansionProof(spec, symbol.ExpansionOrigin, symbol.Stage)
	if err != nil {
		return nil, false, nil, err
	}
	proof = append(proof, expansionProof...)
	if !complete {
		return proof, false, unknown, nil
	}
	scopeProof, scopeErr := scopeProof(symbol.Scope, parents)
	if scopeErr != nil {
		return nil, false, nil, scopeErr
	}
	proof = append(proof, scopeProof...)
	return proof, true, nil, nil
}

func referenceProof(spec Spec, reference Reference, graph selectedGraph) ([]ProofStep, bool, *UnknownRecord, error) {
	proof := make([]ProofStep, 0, 8)
	if reference.ReferenceOrigin == "" {
		unknown := unknownFor(spec, "resolve", "prove-reference-origin", "reference origin is missing", "missing-origin", "supply reference origin and replay the case", "reference-origin-proof")
		proof = append(proof, ProofStep{Kind: "missing-reference-origin", ID: reference.ID})
		return proof, false, unknown, nil
	}
	origin, ok := spec.referenceOriginMap()[reference.ReferenceOrigin]
	if !ok {
		return nil, false, nil, fmt.Errorf("reference %q has an undeclared reference origin", reference.ID)
	}
	proof = append(proof, ProofStep{Kind: "reference-origin", ID: origin.ID, Detail: fmt.Sprintf("%s:%d:%d", origin.Source, origin.Line, origin.Column)})
	expansionProof, complete, unknown, err := expansionProof(spec, reference.ExpansionOrigin, reference.Stage)
	if err != nil {
		return nil, false, nil, err
	}
	proof = append(proof, expansionProof...)
	if !complete {
		return proof, false, unknown, nil
	}
	scopePath, scopeErr := scopeProof(reference.Scope, graph.parents)
	if scopeErr != nil {
		return nil, false, nil, scopeErr
	}
	proof = append(proof, scopePath...)
	if target, ok := graph.symbols[reference.TargetSymbol]; ok {
		proof = append(proof, ProofStep{Kind: "target-symbol-identity", ID: target.ID, Detail: graph.identities[target.ID]})
	}
	return proof, true, nil, nil
}

func expansionProof(spec Spec, expansionID string, expectedStage *int) ([]ProofStep, bool, *UnknownRecord, error) {
	if expansionID == "" {
		unknown := unknownFor(spec, "resolve", "prove-expansion-origin", "expansion origin is missing", "missing-origin", "supply expansion origin and replay the case", "expansion-origin-proof")
		return []ProofStep{{Kind: "missing-expansion-origin"}}, false, unknown, nil
	}
	items := spec.expansionOriginMap()
	proof := make([]ProofStep, 0, 4)
	seen := map[string]bool{}
	complete := true
	unknown := (*UnknownRecord)(nil)
	hops := 0
	first := true
	for expansionID != "" {
		if seen[expansionID] {
			return nil, false, nil, errors.New("expansion origin graph contains a cycle")
		}
		seen[expansionID] = true
		hops++
		if hops > spec.Semantics.Bounds.MaxOriginHops {
			unknown = unknownFor(spec, "resolve", "walk-origin-stack", "origin stack exceeded the static hop bound", "bounded-expansion", "reduce origin nesting and replay the case", "origin-hop-bound")
			complete = false
			break
		}
		origin, ok := items[expansionID]
		if !ok {
			unknown = unknownFor(spec, "resolve", "prove-expansion-origin", "expansion origin is undeclared", "missing-origin", "declare expansion origin and replay the case", "expansion-origin-proof")
			proof = append(proof, ProofStep{Kind: "missing-expansion-origin", ID: expansionID})
			complete = false
			break
		}
		if first {
			if origin.Stage == nil {
				unknown = unknownFor(spec, "expand", "match-expansion-stage", "expansion stage is missing", "missing-stage", "declare the expansion stage and replay the case", "stage-proof")
				complete = false
			} else if expectedStage == nil {
				unknown = unknownFor(spec, "expand", "match-expansion-stage", "required expansion stage is missing", "missing-stage", "declare the syntax stage and replay the case", "stage-proof")
				complete = false
			} else if *origin.Stage != *expectedStage {
				unknown = unknownFor(spec, "expand", "match-expansion-stage", fmt.Sprintf("expansion stage %d conflicts with required stage %d", *origin.Stage, *expectedStage), "ambiguous-stage", "align expansion and syntax stages, then replay the case", "stage-proof")
				complete = false
			}
			first = false
		}
		stage := origin.Stage
		proof = append(proof, ProofStep{Kind: "expansion-origin", ID: origin.ID, Stage: stage, Detail: origin.Macro + "@" + origin.CallSite})
		expansionID = origin.Parent
	}
	return proof, complete, unknown, nil
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

func alphaName(spec Spec, symbol Symbol, identity string, used map[string]bool) string {
	chars := spec.Semantics.AlphaRenaming.IdentityChars
	if chars > len(identity) {
		chars = len(identity)
	}
	candidate := symbol.Spelling + spec.Semantics.AlphaRenaming.Separator + identity[:chars]
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

func captureGrantStatus(spec Spec, graph selectedGraph, reference Reference) (string, *CaptureGrant, *UnknownRecord) {
	if reference.CapturePolicy != "intentional" && reference.CaptureGrant == "" {
		return "none", nil, nil
	}
	if reference.CaptureGrant == "" {
		return "missing", nil, unknownFor(spec, "resolve", "authorize-capture", "explicit capture grant is missing", "missing-grant", "declare a valid capture grant and replay the case", "capture-capability")
	}
	grant, ok := spec.grantMap()[reference.CaptureGrant]
	if !ok {
		return "forged", nil, nil
	}
	target := graph.symbols[reference.TargetSymbol]
	valid := !graph.introduced[target.ID] && grant.ReferenceSymbol == reference.ID && grant.TargetSymbol == reference.TargetSymbol && grant.ExpansionOrigin == reference.ExpansionOrigin && grant.Capability == "capture" && grant.Evidence == "origin-stack" && sameStage(grant.Stage, reference.Stage)
	if !valid {
		return "forged", &grant, nil
	}
	return "valid", &grant, nil
}

func sameStage(left, right *int) bool {
	return left != nil && right != nil && *left == *right
}

func expandSyntax(spec Spec, node SyntaxNode, depth int, graph selectedGraph, report *Report, nodes, splices int) ([]IRNode, error) {
	if nodes >= spec.Semantics.Bounds.MaxNodes {
		unknown := unknownFor(spec, "expand", "visit-syntax-node", "syntax expansion exceeded the static node bound", "bounded-expansion", "reduce the form or raise the declared bound", "node-bound")
		raise(report, StatusUnknown, unknown.Reason, unknown)
		return nil, nil
	}
	if depth > spec.Semantics.Bounds.MaxQuoteDepth {
		unknown := unknownFor(spec, "expand", "enter-quasiquote", "nested quasiquote exceeded the static depth bound", "bounded-expansion", "reduce quote nesting or raise the declared bound", "quote-depth-bound")
		raise(report, StatusUnknown, unknown.Reason, unknown)
		return nil, nil
	}
	nodes++
	switch node.Kind {
	case "sequence":
		out := make([]IRNode, 0, len(node.Children))
		for _, child := range node.Children {
			items, err := expandSyntax(spec, child, depth, graph, report, nodes, splices)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		}
		return out, nil
	case "quasiquote":
		if ok, unknown := stageMatches(spec, node, depth, "enter-quasiquote"); !ok {
			raise(report, StatusUnknown, unknown.Reason, unknown)
			return []IRNode{{Kind: node.Kind, Stage: stageValue(node.Stage), OriginStack: []ProofStep{{Kind: "quasiquote-stage-unknown"}}}}, nil
		}
		stack, complete, unknown, err := expansionProof(spec, node.ExpansionOrigin, node.Stage)
		if err != nil {
			return nil, err
		}
		stack = append(stack, ProofStep{Kind: "syntax-origin", ID: node.Origin, Stage: node.Stage})
		if !complete {
			raise(report, StatusUnknown, unknownReason(unknown, "quasiquote origin proof is incomplete"), unknown)
		}
		children := make([]IRNode, 0, len(node.Children))
		for _, child := range node.Children {
			items, childErr := expandSyntax(spec, child, depth+1, graph, report, nodes, splices)
			if childErr != nil {
				return nil, childErr
			}
			children = append(children, items...)
		}
		return []IRNode{{Kind: node.Kind, Stage: depth, OriginStack: stack, Children: children}}, nil
	case "splice":
		if splices >= spec.Semantics.Bounds.MaxSplices {
			unknown := unknownFor(spec, "expand", "visit-splice", "splice count exceeded the static bound", "bounded-expansion", "reduce splice count or raise the declared bound", "splice-bound")
			raise(report, StatusUnknown, unknown.Reason, unknown)
			return nil, nil
		}
		if ok, unknown := stageMatches(spec, node, depth, "expand-splice"); !ok {
			raise(report, StatusUnknown, unknown.Reason, unknown)
			return []IRNode{{Kind: node.Kind, Stage: stageValue(node.Stage), OriginStack: []ProofStep{{Kind: "splice-stage-unknown"}}}}, nil
		}
		stack, complete, unknown, err := expansionProof(spec, node.ExpansionOrigin, node.Stage)
		if err != nil {
			return nil, err
		}
		stack = append(stack, ProofStep{Kind: "syntax-origin", ID: node.Origin, Stage: node.Stage})
		if !complete {
			raise(report, StatusUnknown, unknownReason(unknown, "splice origin proof is incomplete"), unknown)
		}
		children := make([]IRNode, 0, len(node.Children))
		for _, child := range node.Children {
			items, childErr := expandSyntax(spec, child, depth, graph, report, nodes, splices+1)
			if childErr != nil {
				return nil, childErr
			}
			children = append(children, items...)
		}
		return []IRNode{{Kind: node.Kind, Stage: depth, OriginStack: stack, Children: children}}, nil
	case "binder":
		symbol, ok := graph.symbols[node.Symbol]
		if !ok {
			return nil, fmt.Errorf("syntax binder %q is not selected by the case", node.Symbol)
		}
		if ok, unknown := stageMatches(spec, node, depth, "bind-symbol"); !ok {
			raise(report, StatusUnknown, unknown.Reason, unknown)
		}
		return []IRNode{{Kind: node.Kind, Stage: depth, SymbolID: symbol.ID, Spelling: graph.effective[symbol.ID], OriginStack: symbolStack(report, symbol.ID)}}, nil
	case "reference":
		reference, ok := graph.references[node.Reference]
		if !ok {
			return nil, fmt.Errorf("syntax reference %q is not selected by the case", node.Reference)
		}
		if ok, unknown := stageMatches(spec, node, depth, "reference-symbol"); !ok {
			raise(report, StatusUnknown, unknown.Reason, unknown)
		}
		target := graph.symbols[reference.TargetSymbol]
		return []IRNode{{Kind: node.Kind, Stage: depth, SymbolID: target.ID, ReferenceID: reference.ID, Spelling: graph.effective[target.ID], OriginStack: referenceStack(report, reference.ID)}}, nil
	case "literal":
		return []IRNode{{Kind: node.Kind, Stage: depth, Value: node.Value, OriginStack: []ProofStep{{Kind: "literal-origin", ID: node.Origin}}}}, nil
	default:
		return nil, fmt.Errorf("unsupported syntax node kind %q", node.Kind)
	}
}

func stageMatches(spec Spec, node SyntaxNode, depth int, step string) (bool, *UnknownRecord) {
	if node.Stage == nil {
		return false, unknownFor(spec, "expand", step, "syntax stage is missing", "missing-stage", "declare the syntax stage and replay the case", "stage-proof")
	}
	if *node.Stage != depth {
		return false, unknownFor(spec, "expand", step, fmt.Sprintf("syntax stage %d is ambiguous at expansion depth %d", *node.Stage, depth), "ambiguous-stage", "align syntax stage with expansion depth and replay the case", "stage-proof")
	}
	return true, nil
}

func stageValue(stage *int) int {
	if stage == nil {
		return -1
	}
	return *stage
}

func symbolStack(report *Report, id string) []ProofStep {
	for _, symbol := range report.Symbols {
		if symbol.ID == id {
			return symbol.OriginProofPath
		}
	}
	return nil
}

func referenceStack(report *Report, id string) []ProofStep {
	for _, reference := range report.References {
		if reference.ID == id {
			return reference.OriginProofPath
		}
	}
	return nil
}

func stableIdentity(spec Spec, symbol Symbol) string {
	payload := strings.Join([]string{symbol.DeclarationOrigin, symbol.ExpansionOrigin, symbol.ID}, spec.Semantics.Identity.Delimiter)
	sum := sha256.Sum256([]byte(payload))
	return spec.Semantics.Identity.Prefix + hex.EncodeToString(sum[:])
}

func unknownFor(spec Spec, stage, step, reason, unknownClass, nextOperation, blockedBy string) *UnknownRecord {
	if stage == "" {
		stage = spec.Semantics.Unknown.Stage
	}
	if step == "" {
		step = spec.Semantics.Unknown.Step
	}
	if reason == "" {
		reason = spec.Semantics.Unknown.Reason
	}
	if unknownClass == "" {
		unknownClass = spec.Semantics.Unknown.UnknownClass
	}
	if nextOperation == "" {
		nextOperation = spec.Semantics.Unknown.NextOperation
	}
	if blockedBy == "" {
		blockedBy = spec.Semantics.Unknown.BlockedBy
	}
	return &UnknownRecord{Stage: stage, Step: step, Reason: reason, UnknownClass: unknownClass, NextOperation: nextOperation, BlockedBy: blockedBy}
}

func unknownReason(record *UnknownRecord, fallback string) string {
	if record != nil && record.Reason != "" {
		return record.Reason
	}
	return fallback
}

func unknownDecision(spec Spec, record *UnknownRecord, fallback string) string {
	if record == nil {
		return fallback
	}
	switch record.UnknownClass {
	case "missing-stage", "ambiguous-stage":
		return spec.Semantics.CaptureJudgment.MissingStage
	case "missing-grant":
		return spec.Semantics.CaptureJudgment.MissingGrant
	default:
		return spec.Semantics.CaptureJudgment.MissingOrigin
	}
}

func raise(report *Report, candidate Status, reason string, unknown *UnknownRecord) {
	if rank(candidate) > rank(report.Status) {
		report.Status = candidate
		report.Reason = reason
	}
	if candidate == StatusUnknown && report.Unknown == nil {
		report.Unknown = unknown
	}
}

func rank(status Status) int {
	switch status {
	case StatusRefuted:
		return 3
	case StatusUnknown:
		return 2
	case StatusClosed:
		return 1
	default:
		return 0
	}
}

func EncodeReport(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
