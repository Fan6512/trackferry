# Builds from source on one machine. Published images use build/Dockerfile.release instead.
# The runtime image is scratch: one static binary.
# No "# syntax=" line: a NAS cannot always reach Docker Hub for the frontend image.

FROM node:22.22.0-alpine AS web
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --registry=${NPM_REGISTRY} --no-audit --no-fund
COPY web ./
# vite writes to ../internal/webui/dist
RUN mkdir -p /src/internal/webui && npm run build

FROM golang:1.27.3-alpine AS build
ARG GOPROXY=https://goproxy.cn,direct
ARG VERSION=
ARG COMMIT=unknown
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum VERSION ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN go build -ldflags "-s -w -X main.version=${VERSION:-$(cat VERSION)} -X main.commit=${COMMIT}" -o /out/trackferry ./cmd/trackferry && mkdir -p /out/run

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/trackferry /trackferry
# The shared volume takes this directory's owner.
COPY --from=build --chown=10001:10001 /out/run /run/trackferry
USER 10001:10001
ENV TRACKFERRY_CONFIG=/data/config.yaml TZ=Asia/Shanghai
EXPOSE 18561
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/trackferry", "healthcheck"]
ENTRYPOINT ["/trackferry"]
CMD ["serve"]
