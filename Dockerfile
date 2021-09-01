# Step 1: compile

FROM golang:1.17 AS build
WORKDIR /go/src/app
COPY . .

# Build all plugins
RUN for plugin in $(ls plugins); do go build -o $plugin.so -buildmode=plugin ./plugins/$plugin/cmd; done

# Build the main application
RUN go install  ./cmd/goplum

# Generate licence information
RUN go run github.com/google/go-licenses@latest save ./... --save_path=/notices

# Step 2: execute

FROM gcr.io/distroless/base:nonroot@sha256:a74f307185001c69bc362a40dbab7b67d410a872678132b187774fa21718fa13
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
