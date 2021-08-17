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

FROM gcr.io/distroless/base:nonroot@sha256:ccbc79c4fc35b92709d3987315cdb9e20b6e742546af7a7db10024641aa60572
WORKDIR /
COPY --from=build /go/bin/goplum /goplum
COPY --from=build /go/src/app/*.so /plugins/
COPY --from=build /notices /notices
ENTRYPOINT ["/goplum"]
