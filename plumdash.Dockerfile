FROM golang:1.27.3 AS build
WORKDIR /go/src/app
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    set -eux; \
    CGO_ENABLED=0 go install ./cmd/plumdash; \
    go run github.com/google/go-licenses@latest save ./... --save_path=/notices;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261020.1
WORKDIR /
COPY --from=build /go/bin/plumdash /plumdash
COPY --from=build /notices /notices
ENTRYPOINT ["/plumdash"]
