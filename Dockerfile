# Step 1: compile

FROM golang:1.16 AS build
WORKDIR /go/src/app
COPY . .

# Build all plugins
RUN for plugin in $(ls plugins); do go build -o $plugin.so -buildmode=plugin ./plugins/$plugin/cmd; done

# Build the main application
RUN go install  ./cmd/goplum

# Generate licence information
RUN go get github.com/google/go-licenses && go-licenses save ./... --save_path=/notices

# Step 2: execute

FROM gcr.io/distroless/base:nonroot@sha256:38778ff7aa549bf6904c9d1c68bfe5946e96cac91dc32ba1f58e83bb9c9e6abe
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
