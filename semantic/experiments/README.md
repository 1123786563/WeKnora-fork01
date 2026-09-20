# Semantica V01: frozen minimal import-contract profile

This is a **minimal experimental profile**, not Semantica's default or full
installation. It freezes CPython `3.12.12`, Semantica `0.6.8`, and the exact
wheel SHA-256 `0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`.
The wheel is locked in `uv.lock`; its release provenance is tag `v0.6.8`, tag
object `29f3c3230cb72e6b84201a0f5f4e310929fefb47`, commit
`f73f599a22c320676a45f247247038ca1fcf40f0`.

`[tool.uv].dependency-metadata` intentionally replaces the distribution's
dependency metadata with an empty set only for `semantica==0.6.8`. The project
then explicitly locks the dependencies needed to import V01's seven public
exports: numpy, pandas, scipy, networkx, python-dateutil, PyYAML, rdflib,
requests, pydantic, pytest, and the Neo4j driver. Neo4j is pinned at `5.28.2`:
Semantica's `Neo4jStore` uses `read_transaction`/`write_transaction`, which the
6.x driver removes, while Neo4j documents 5.28 as forward compatible with
2025.x servers. This proves neither a connection nor storage acceptance.

The official wheel's non-extra `Requires-Dist` entries, copied verbatim from
its `METADATA`, are:

```text
numpy>=2.0.2
pandas>=1.3.0
scipy>=1.13.1
scikit-learn>=1.7.2; python_version >= "3.10"
umap-learn>=0.5.12
spacy>=3.4.0; python_version >= "3.10"
transformers>=4.20.0
torch>=1.13.1
sentence-transformers>=2.2.0
rdflib>=6.2.0
networkx>=2.8.0
matplotlib>=3.9.4
seaborn>=0.13.2
plotly>=6.8.0
ipywidgets>=8.0.0
requests>=2.34.2; python_version >= "3.10"
GitPython>=3.1.58
chardet>=7.4.3; python_version >= "3.10"
protobuf>=5.29.1
grpcio>=1.81.1; python_version >= "3.10"
beautifulsoup4>=4.15.0
lxml>=6.1.1
python-docx>=1.2.0
openpyxl>=3.1.5
pillow>=12.2.0; python_version >= "3.10"
librosa>=0.9.0
opencv-python>=4.13.0.92
faiss-cpu>=1.7.0
fastembed>=0.2.0
onnxruntime>=1.20.1; python_version >= "3.10"
tokenizers>=0.15.0
pydantic>=2.13.4
click>=8.4.2; python_version >= "3.10"
rich>=12.5.0
tqdm>=4.68.3
pyyaml>=6.0
toml>=0.10.0
python-dotenv>=1.2.1
loguru>=0.7.3
structlog>=22.1.0
gensim>=4.4.0
httpx<0.29.0
pyarrow>=14.0.0
```

The omitted default dependencies are every entry above except the explicitly
locked import prerequisites: scikit-learn, umap-learn, spaCy, transformers,
torch, sentence-transformers, plotting/notebook libraries, GitPython,
protobuf/grpc, document/media parsers, faiss/fastembed/onnxruntime/tokenizers,
CLI/logging/progress libraries, gensim, httpx, and pyarrow. No model weights
are downloaded. All optional extras are also omitted: provider SDKs; document
parsing; graph, vector, triplet, database, cloud, infra, monitoring, visual,
agent, split, development, explorer, GPU and `all` extras. To inspect the
remaining marker-specific and extra entries exactly as released, run:

```sh
unzip -p /tmp/semantica-0.6.8-py3-none-any.whl '*/METADATA' | rg '^Requires-Dist:'
```

Clean replay uses a new environment and never reuses `.venv312` or docreader:

```sh
curl -fsSL https://files.pythonhosted.org/packages/f5/4d/edf005139044fca3f86711d2bff96de93ae3e10ed1a6d02bc05252b610e2/semantica-0.6.8-py3-none-any.whl -o /tmp/semantica-0.6.8-py3-none-any.whl
shasum -a 256 /tmp/semantica-0.6.8-py3-none-any.whl
UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean uv sync --project semantic/experiments --frozen
UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q
SEMANTICA_WHEEL=/tmp/semantica-0.6.8-py3-none-any.whl UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean uv run --project semantic/experiments python semantic/experiments/verify_version.py --output docs/superpowers/plans/semantica/capability-evidence.json
```

The verify command writes structured JSON and exits non-zero for a missing or
mismatched wheel, unavailable distribution metadata, a wrong version, or any
failed public import. It checks installed distribution metadata and public
imports; it does not source-load Semantica from the wheel. `--wheel` may replace
`SEMANTICA_WHEEL`; with neither, it writes `wheel-not-specified` evidence and
never downloads anything.

## Retained bounded capability probe

This is an experiment only. It does not create a service, call a model, use
credentials, connect Neo4j, or validate Go authorization/deletion behavior.

Download the exact public wheel and verify its hash before probing:

```sh
curl -fsSL https://files.pythonhosted.org/packages/f5/4d/edf005139044fca3f86711d2bff96de93ae3e10ed1a6d02bc05252b610e2/semantica-0.6.8-py3-none-any.whl -o /tmp/semantica-0.6.8-py3-none-any.whl
shasum -a 256 /tmp/semantica-0.6.8-py3-none-any.whl
uv venv --python 3.12.12 semantic/experiments/.venv312
uv pip install --python semantic/experiments/.venv312/bin/python -r semantic/experiments/requirements-probe.txt
semantic/experiments/.venv312/bin/python semantic/experiments/runtime_probe.py --wheel /tmp/semantica-0.6.8-py3-none-any.whl --output docs/superpowers/plans/semantica/evidence/2026-09-20/runtime-312.json
semantic/experiments/.venv312/bin/python -m pytest semantic/experiments/test_capability_probe.py -q
```

Expected SHA-256: `0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`.
An explicit missing or mismatched `--wheel` fails before extraction or import;
omitting `--wheel` produces only `not-tested` evidence.
The probe records public-import failures as `actual-runtime` and release-wheel
source inspection as `source-only`; neither is a production acceptance result.
The current runtime evidence uses CPython 3.12.12 and the pinned probe dependencies above. CPython 3.9.6 import failure is historical diagnostic evidence only.

`capability_probe.py` can be run separately to reproduce the historical
source-only/CPython 3.9 diagnostic; it does not replace `runtime-312.json`.
