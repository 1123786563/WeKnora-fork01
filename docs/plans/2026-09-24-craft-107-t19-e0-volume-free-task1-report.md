# Craft #107 T19 E0 Volume-Free Task 1 Report

Status: **DONE**

## Scope and result

Built a Linux/arm64 test derivative from the exact pinned OpenCode filesystem and embedded the reviewed source recorder. The final image is `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615`. Its inspected config has no `Volumes`, and preserves UID `10001:10001`, PATH/HOME/XDG env, entrypoint, command, working directory, and exposed port. OpenCode remains version 1.18.4 with pinned binary SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`.

The base RED inspect confirms all four participants inherit two writable anonymous XDG volumes and violate the fixed mount shape. Final derivative D/A/helper/C inspections show UID `10001:10001` and the requested mounts: D/A each have one separate writable `/ledger` volume, helper has no mounts, and C has exactly the named config/data XDG volumes. All participants’ image IDs equal the inspected derivative ID. Separate D/A ledger volumes were pre-chowned by bounded `--rm`, `--network none`, `--cap-drop ALL`, `--cap-add CHOWN` provisioning commands; each participant then wrote a file as UID 10001. Provisioner commands and raw volume/container/network inspections are retained; all disposable base and derivative probe containers, volumes and networks were removed, with absence checks retained.

The first baked recorder destination under `/probe` was inaccessible to UID 10001 because of its inherited directory permissions. I moved it to `/usr/local/bin/craft_source_recorder.py`; the final image passes both same-UID SHA-256 and Python CLI help checks. The runner remains unchanged and must not adopt the derivative before its separate Task2 review.

Probe networks were Docker-internal to keep the test egress-free. This verifies participant image, UID and mount topology; it does not validate the full private/external network attachment matrix or any egress behavior.

## Commands and results

- `docker image inspect sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` — pass; exact source config saved.
- `docker run --rm --network none --entrypoint opencode <base> --version` — pass; `1.18.4`.
- `docker run --rm --network none --entrypoint sha256sum <base> /usr/local/bin/opencode` — pass; matched pinned SHA-256.
- `docker build --platform linux/arm64 --progress plain --file docs/testing/craft/egress-probe/Dockerfile.volume-free --tag craft-e0-volume-free:task1 docs/testing/craft/egress-probe` — pass; final build log saved.
- `docker run --rm --network none --entrypoint sha256sum craft-e0-volume-free:task1 /usr/local/bin/craft_source_recorder.py` — pass as image-default UID `10001:10001`; matched source SHA-256 `d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463`.
- `docker run --rm --network none --entrypoint python3 craft-e0-volume-free:task1 /usr/local/bin/craft_source_recorder.py --help` — pass as image-default UID `10001:10001`.
- Disposable base and derivative Docker inspect probes — pass; raw records and cleanup absence outputs are saved under `docs/testing/craft/egress-probe/volume-free/`.
- `python3 -m unittest docs/testing/craft/egress-probe/test_volume_free_image.py` — 4 tests pass, including actual base RED inspect, final image config/binary/recorder identity, participant image IDs and UID, mount acceptance, and malformed topology rejection.
- `python3 -m py_compile docs/testing/craft/egress-probe/test_volume_free_image.py` — pass.
- `git diff --check -- docs/testing/craft/egress-probe/Dockerfile.volume-free docs/testing/craft/egress-probe/test_volume_free_image.py` — pass.

No commit, push, publication, production change, verifier change, runner change, or paid egress was performed.
