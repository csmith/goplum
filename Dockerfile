# Step 1: compile

FROM golang:1.15 AS build
WORKDIR /go/src/app
COPY . .

# Build all plugins
RUN for plugin in $(ls plugins); do go build -o $plugin.so -buildmode=plugin ./plugins/$plugin/cmd; done

# Build the main application
RUN go install  ./cmd/goplum


# Step 2: execute

FROM gcr.io/distroless/base:nonroot
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
ENTRYPOINT ["/goplum"]
