# T24 multi-architecture Docker evidence — 2026-09-11

Executed from the React migration worktree:

```text
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VITE_FRONTEND_COMMIT=0e886be \
  --output type=oci,dest=/tmp/weknora-ui-t24-multiarch-1789060967.tar \
  -f frontend/Dockerfile .
```

BuildKit completed with exit code 0 using the local `orbstack` builder. The
OCI index was inspected after export and contained these platform manifests:

| Platform | Image manifest digest |
|---|---|
| `linux/amd64` | `sha256:81021e7548378815616dd2902ae9f523bd5429e6817d39df20853ddff77e55c9` |
| `linux/arm64` | `sha256:6241fb9c9ca7fd161e85d8e8bb8b51e97566f29f7b2c0d0d4177dc927c7a980b` |

The OCI archive is 72 MB and was kept outside the repository. This proves the
two target image builds and manifest export, but not registry push, deployed
Nginx behavior, API proxying, or production rollback.
