# Gooo hygienic origin resolver

This repository is an executable language slice for bounded semantic macro
expansion in nested staged quasiquote/splice forms. The authoritative semantics
live in [`origin-resolver.gooo`](.gooo/origin-resolver.gooo): it declares the
syntax nodes, stages, origins, scope edges, symbols, capture capabilities,
status precedence, denominator, and generation plan.

Go is limited to parsing the metacode, executing its declared judgments, and
emitting structured Go syntax. It does not perform arbitrary string replacement.

## What is covered

The v2 contract fixes one denominator of 12 cells/meta activities, proof lanes
FOUNDATION/COHERENCE/REGRESSION at 4/4/4, indicator lanes
DRIVER/OUTCOME/GUARDRAIL at 4/4/4, and a 4/4/4 CLOSED/UNKNOWN/REFUTED case
vector. It contains executable cases for:

- nested quasiquote and ordered two-splice expansion;
- shadowed and sibling-colliding binders with deterministic alpha-renaming;
- explicit intentional capture backed by an exact origin-stack capability;
- missing origin, stage, grant, and expansion evidence preserved as `UNKNOWN`;
- forged grants and implicit/fixed capture counterexamples preserved as `REFUTED`.

Every resolved symbol, reference, and IR node carries origin evidence. A caller
binding can be captured only by an explicit valid grant. `REFUTED > UNKNOWN >
CLOSED` is read from the `.gooo` contract, and the implementation makes no
global hygiene completeness claim.

The research boundary is intentionally narrow: quasiquote depth and splice
ordering follow the public [cosmos72/gomacro quasiquote source](https://github.com/cosmos72/gomacro/blob/master/fast/quasiquote.go)
and [quasiquote notes](https://github.com/cosmos72/gomacro/blob/master/doc/quasiquote.md);
lexical scope, parsing, AST construction, and formatting use the Go 1.27
[language specification](https://go.dev/ref/spec),
[go/parser](https://go.dev/pkg/go/parser/),
[go/ast](https://go.dev/pkg/go/ast/), and
[go/format](https://go.dev/pkg/go/format/) contracts. No source code is copied.

## CI-only verification

GitHub Actions uses Go 1.27 to compile the packages, build both executors, run
the tests, replay the declared cases, emit a generated capture-free Go example,
and build that generated file. The uploaded evidence includes exact integer
inventory, physical-line, generated-artifact, runtime, peak-RSS, and test-count
measurements. Root `README.md` is excluded from inventory by the bootstrap
contract.

The repository began from `gooo-repository-bootstrap` v0.1.1. The bootstrap
commit is the only direct-main exception; functional changes are PR-only.
The v0.1.2 release attempt is recorded as `OPERATIONAL_REFUTED` in
[`docs/release-attempt-v0.1.2.json`](docs/release-attempt-v0.1.2.json); it is
permanently burned, so the next release workflow requires fresh `v0.1.3`.

## Commands

The commands below are intended for CI or a caller-owned output directory:

```text
go run ./cmd/gooo-origin resolve --output /caller/output/origin.json --human-output /caller/output/origin.md
go run ./cmd/gooo-origin emit --scenario nested-quasiquote --output /caller/output/generated.go
```

Reports and generated files must be outside the target repository. Local
execution is not evidence of closure; GitHub Actions is the verification source.
