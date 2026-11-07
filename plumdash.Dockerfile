# Step 1: compile

FROM reg.c5h.io/golang AS build
WORKDIR /go/src/app
COPY . .

# Build the dashboard
RUN go install ./cmd/plumdash

# Generate licence information
RUN go run github.com/google/go-licenses@latest save ./... --save_path=/notices

# Step 2: execute

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261020.1
WORKDIR /
COPY --from=build /go/bin/plumdash /plumdash
COPY --from=build /notices /notices
ENTRYPOINT ["/plumdash"]
