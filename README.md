# Gooo hygienic origin resolver

This repository is an executable language slice for preventing name capture in
metaprogramming. The authoritative semantics live in
[`origin-resolver.gooo`](.gooo/origin-resolver.gooo): it declares declaration
origins, expansion origins, scope edges, introduced symbols, reference symbols,
status precedence, the denominator, and the generation plan.

Go is limited to parsing the metacode, executing its declared judgments, and
emitting structured Go syntax. It does not perform arbitrary string replacement.

## What is covered

The contract contains executable cases for:

- normal nested expansion with deterministic alpha-renaming;
- intended capture explicitly targeting a caller symbol;
- unintended capture preserved as `REFUTED`;
- missing origin proof preserved as `UNKNOWN` with all six required fields; and
- replay with stable symbol identities, names, and capture decisions.

Every resolved symbol and reference carries an origin proof path and capture
decision. `REFUTED > UNKNOWN > CLOSED` is read from the `.gooo` contract.

## CI-only verification

GitHub Actions uses Go 1.27 to compile the packages, build both executors, run
the tests, replay the declared cases, emit a generated capture-free Go example,
and build that generated file. The uploaded evidence includes exact integer
inventory, physical-line, generated-artifact, runtime, peak-RSS, and test-count
measurements. Root `README.md` is excluded from inventory by the bootstrap
contract.

The repository began from `gooo-repository-bootstrap` v0.1.1. The bootstrap
commit is the only direct-main exception; functional changes are PR-only.

## Commands

The commands below are intended for CI or a caller-owned output directory:

```text
go run ./cmd/gooo-origin resolve --output /caller/output/origin.json
go run ./cmd/gooo-origin emit --scenario normal-nested-expansion --output /caller/output/generated.go
```

Reports and generated files must be outside the target repository. Local
execution is not evidence of closure; GitHub Actions is the verification source.
