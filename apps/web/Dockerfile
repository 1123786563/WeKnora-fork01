# React Web/Embed candidate image. The current production UI image remains
# frontend/Dockerfile (Vue) until T25 is accepted.
# Build from the repository root so shared packages and the pnpm lockfile are
# part of the build context:
#
#   docker build -f apps/web/Dockerfile \
#     --build-arg VITE_FRONTEND_COMMIT=$(git rev-parse --short HEAD) \
#     -t weknora-ui-react:candidate .

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS builder

WORKDIR /workspace

ARG NODE_MAX_OLD_SPACE_SIZE=4096
ARG REACT_BUILD_VERSION=dev
ARG VITE_FRONTEND_COMMIT=unknown

ENV NODE_OPTIONS="--max-old-space-size=${NODE_MAX_OLD_SPACE_SIZE}"
ENV REACT_BUILD_VERSION=${REACT_BUILD_VERSION}
ENV VITE_FRONTEND_COMMIT=${VITE_FRONTEND_COMMIT}

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages ./packages
COPY apps ./apps
COPY scripts/build_react_web_bundle.sh ./scripts/build_react_web_bundle.sh

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate \
    && pnpm install --frozen-lockfile \
    && pnpm run build:react-bundle

FROM nginx:1.30.3-alpine@sha256:0d3b80406a13a767339fbe2f41406d6c7da727ab89cf8fae399e81f780f814d1

COPY --from=builder /workspace/dist/react-web/web /usr/share/nginx/html
COPY frontend/nginx.conf /etc/nginx/templates/default.conf.template
COPY frontend/nginx-api-proxy.conf /etc/nginx/api-proxy.conf
COPY frontend/nginx-http.conf /etc/nginx/conf.d/00-weknora-http.conf
COPY frontend/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV MAX_FILE_SIZE_MB=50

EXPOSE 80
ENTRYPOINT ["/docker-entrypoint.sh"]
