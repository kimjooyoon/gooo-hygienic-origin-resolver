# Bootstrap provenance

The initial repository tree was copied from the immutable
`gooo-repository-bootstrap` release `v0.1.1` before functional changes.

- Source repository: `https://github.com/kimjooyoon/gooo-repository-bootstrap`
- Source tag: `v0.1.1`
- Source commit: `594b917c4b289adaf7b83e079bb526df68b339bb`
- Release asset: `gooo-evidence-0.1.1.json`
- Release asset digest: `sha256:dc204e92d219e291b9568f18affd556f226fc54114c008d00e6189ea169c9076`
- Bootstrap exception: exactly one `BOOTSTRAP_EXCEPTION` commit
- Post-bootstrap direct-main commits: required to be exactly zero; violation is `REFUTED`

Functional changes are made on a pull-request branch. Evidence outputs are
written to caller-owned runner temporary paths, never into the input checkout.
