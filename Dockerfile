FROM golang:1.27.4 AS build
WORKDIR /go/src/app
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    set -eux; \
    CGO_ENABLED=0 go install ./cmd/goplum; \
    mkdir /notices; \
    go tool gocredits -skip-missing . > /notices/CREDITS;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261022.0
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
