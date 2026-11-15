FROM golang:1.27.4 AS build
WORKDIR /go/src/app
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    set -eux; \
    CGO_ENABLED=0 go install ./cmd/plumdash; \
    mkdir /notices; \
    go tool gocredits -skip-missing . > /notices/CREDITS;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261020.1
WORKDIR /
COPY --from=build /go/bin/plumdash /plumdash
COPY --from=build /notices /notices
ENTRYPOINT ["/plumdash"]
