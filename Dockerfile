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

FROM gcr.io/distroless/base:nonroot@sha256:19d927c16ddb5415d5f6f529dbbeb13c460b84b304b97af886998d3fcf18ac81
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
