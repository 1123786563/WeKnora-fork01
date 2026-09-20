# V03 Chinese evaluation baseline

The frozen public corpus is `semantic/experiments/fixtures/questions.jsonl`.
It contains six Chinese cases: cross-document dependency chain, a technical
`depends_on` rule chain, conflict, no evidence, deletion, and revocation.
Every row fixes `document_revision: v03-frozen-1`, source text, expected
evidence, and the allowed evidence scope.

## Candidate measurement

`evaluate.py --backend semantica` used Semantica 0.6.8 public
`provider_registry`, `NERExtractor(method="llm")`, and
`RelationExtractor(method="llm")` through the explicit loopback Go transport
at `127.0.0.1:18092`, fixed to host Ollama `qwen2.5:0.5b`. It made 22 bounded
model calls: 8 NER, 8 relation, and 6 query calls. The V03 extraction profile
permits 512 output tokens and 8192 bytes; V02's 96-token/1200-byte profile is
unchanged. No truncation was observed.

The retained result rows are `semantica-results.jsonl`. Query-source scoring
was unfavorable: precision 0.1667, recall 0.4167, and zero correct abstentions.
Examples include a one-hop answer to the cross-document chain, a one-sided
conflict answer, an invented risk-control dependency, and answers to deleted
or revoked questions despite explicit evidence-insufficient instructions.
Only source quotes present in both the actual answer and its frozen source text
become `evidence_ids`; allowed scope is never copied into actual evidence.

The transport's initial per-row ledger retained the six query usage values
(519 total tokens) but did not preserve NER/RE usage attribution from extractor
instances. The gateway log retains 18 additional successful raw calls, but
they are not reclassified as per-row usage. This is an evidence gap, not an
estimated measurement.

## Native comparison

The real isolated native prerequisite is negative: it completed graph
indexing, found two source-linked Neo4j nodes, and completed SSE, but emitted
zero graph references. Its result is retained in
`semantic/experiments/native_env/evidence/v03-native-20260920.md` and has no
actual token usage. Native therefore has no retrieved evidence to score for
the frozen corpus, and cannot establish candidate/native parity. Its effective
configuration is different from the Semantica candidate: it is the existing Go
graph pipeline with a remote-loopback local model registration rather than
Semantica's public NER/RE API.

Cold/warm p50/p95 and per-case native usage remain unverified. The policy is
closed (`approved: false`); the proposed numbers are not promotion approval.
