# Staged quasiquote/splice language slice

`.gooo/origin-resolver.gooo` is the only semantic authority. Its typed JSON
declaration is parsed by `LoadSpec`; Go then performs the bounded evaluator over
the declared syntax tree and emits the IR and generated Go view.

The execution path is:

```text
.gooo declaration
  -> typed syntax parser
  -> explicit stage/depth matching
  -> nested quasiquote and ordered splice expansion
  -> origin-backed stable identity
  -> deterministic alpha-renaming of fresh visible collisions
  -> lexical target resolution and capture capability check
  -> structured IR + generated Go
  -> machine JSON + human Markdown evidence
```

The resolver is deliberately bounded by the contract's node, quote-depth,
splice, and origin-hop limits. It does not claim complete hygiene for an
unbounded language. `REFUTED` dominates `UNKNOWN`, which dominates `CLOSED`.
Missing origin, stage, or grant evidence stays `UNKNOWN` with all six fields:
`stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`.
An intentional caller capture is closed only by a grant whose reference,
target, expansion, stage, capability, and `origin-stack` evidence all match.

The contract's denominator is fixed at 12 cells/meta activities. Its proof
lanes FOUNDATION, COHERENCE, REGRESSION each contain four activities, and its
indicator lanes DRIVER, OUTCOME, GUARDRAIL each contain four activities. The
declared cases are fixed at four CLOSED, four UNKNOWN, and four REFUTED.

The design uses two public research boundaries. The
[gomacro quasiquote implementation](https://github.com/cosmos72/gomacro/blob/master/fast/quasiquote.go)
and [quasiquote notes](https://github.com/cosmos72/gomacro/blob/master/doc/quasiquote.md)
inform stage-aware traversal and ordered splice handling. The Go
[specification](https://go.dev/ref/spec),
[parser documentation](https://go.dev/pkg/go/parser/),
[AST documentation](https://go.dev/pkg/go/ast/), and
[format documentation](https://go.dev/pkg/go/format/) inform lexical scope,
syntax representation, and output formatting. The implementation is an
independent bounded static evaluator; it does not copy gomacro code.
