# V03 Chinese evaluation baseline

The frozen public corpus is `semantic/experiments/fixtures/questions.jsonl`.
It contains six Chinese cases: cross-document dependency chain, a technical
`depends_on` rule chain, conflict, no evidence, deletion, and revocation.
Every row fixes `document_revision: v03-frozen-1`, source text, expected
evidence, and the allowed evidence scope.

## Candidate measurement

The final `evaluate.py --backend semantica` run used Semantica 0.6.8 public
`provider_registry`, `NERExtractor(method="llm")`, and
`RelationExtractor(method="llm")` through the explicit loopback Go transport
at `127.0.0.1:18092`, fixed to host Ollama `qwen2.5:0.5b`. It retained 21 new
actual attempts: 6 NER, 6 relation, 3 cold GraphReasoner, 3 warm GraphReasoner,
and 3 failed source-validated graph retrieval stages. Together with 22 historic
calls the bounded ledger is 43 of 80. The V03 extraction profile
permits 512 output tokens and 8192 bytes; V02's 96-token/1200-byte profile is
unchanged. No truncation was observed.

The retained result rows are `semantica-results.jsonl`: precision 0.3333,
recall 0.8333, two correct answers, and zero correct abstentions. Three cases
are retained as `failed` specifically because their source-validated graph
retrieval was empty; they are not relabelled as negative results. Evidence IDs
come only from retrieved graph provenance, never from allowed scope.

The transport's initial per-row ledger retained the six query usage values
(519 total tokens) but did not preserve NER/RE usage attribution from extractor
instances. The gateway log retains 18 additional successful raw calls, but
they are not reclassified as per-row usage. This is an evidence gap, not an
estimated measurement.

## Native comparison

The final isolated native transaction is retained at
`evidence/2026-09-20/v03-native-final-2.json`: one case observed a numeric Go
Graph MatchType mapped to its runtime source, five cases are negative, the
deleted source was actually deleted before projection, and per-document
revocation is unsupported. The revoked source's canary leaked through native
fallback, so the native privacy hard gate is false. The earlier
`v03-native-final-attempt.json` retains the launcher-path failure separately.
Native SSE usage is unavailable and not estimated. Native therefore cannot
establish candidate/native parity. Its effective
configuration is different from the Semantica candidate: it is the existing Go
graph pipeline with a remote-loopback local model registration rather than
Semantica's public NER/RE API.

Every native case has cold and warm query timing plus index timing; native
usage remains unavailable. The policy is closed (`approved: false`); the
proposed numbers are not promotion approval.
