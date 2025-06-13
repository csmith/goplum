# Step 1: compile

FROM reg.c5h.io/golang AS build
WORKDIR /go/src/app
COPY . .

# Build all plugins
RUN for plugin in $(ls plugins); do go build -o $plugin.so -buildmode=plugin ./plugins/$plugin/cmd; done

# Build the main application
RUN go install  ./cmd/goplum

# Generate licence information
RUN go run github.com/google/go-licenses@latest save ./... --save_path=/notices

# Step 2: execute

FROM ghcr.io/greboid/dockerbase/nonroot:1.20250326.0
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
