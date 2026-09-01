# Executable model

The `.gooo` contract is the source of meaning. Its `semantics` object declares
the identity inputs, alpha-renaming policy, capture judgments, status
precedence, UNKNOWN record, denominator, and generation plan. The symbol tables
then supply the origin graph consumed by the executor.

Stable identity is the declared SHA-256 of the ordered triple
`declaration_origin | expansion_origin | symbol_id`. A fresh introduced binder
whose original spelling is visible in an ancestor scope receives the declared
alpha suffix. References to that introduced identity receive the same effective
spelling through the symbol table. A caller reference marked intentional keeps
its caller target. A fixed binder that causes a different actual target is
preserved as `REFUTED`.

Each symbol and reference report contains a structured proof path through its
origin, expansion chain, scope edges, and target identity. Missing proof data is
not guessed: it is `UNKNOWN` and carries `stage`, `step`, `reason`,
`unknown_class`, `next_operation`, and `blocked_by`.

The emitter consumes resolved symbol evidence and writes Go syntax with quoted
literal values and validated identifiers. It never rewrites source text by
substring replacement.
