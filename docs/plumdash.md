# plumdash

`plumdash` is a web dashboard that shows the state of each check, along with
its recent results. It connects to GoPlum using the [GoPlum API](api.md).

plumdash has no authentication and serves plain HTTP. It is meant to run
behind a reverse proxy that terminates TLS, routes by hostname, and adds
authentication. Don't expose its port directly, even on a trusted network:
plumdash doesn't check the `Host` header, so any web page opened in a browser
that can reach it could read the dashboard using DNS rebinding.

## Getting started

To install the latest version:

```
$ go install chameth.com/goplum/cmd/plumdash
```

plumdash needs the certificate from your Certificate Authority, and a cert and
key pair to use when connecting to the API. For information on how to generate
these see the [API documentation](api.md).

```
$ plumdash --server plum.example.com:7586 \
    --ca-cert ca.crt \
    --cert client.crt \
    --key client.key
```

The dashboard is then served on port 8080, for your reverse proxy to forward
to. If the proxy runs on the same host, use `--listen 127.0.0.1:8080` so the
port isn't reachable from the network. The page refreshes itself every 30
seconds.

A Docker image is also available as `ghcr.io/csmith/goplum/dashboard`. Attach
it to a network your reverse proxy is on rather than publishing its port:

```
$ docker run -d --restart always \
    --network proxy \
    -v /path/to/certs:/certs:ro \
    -e SERVER=plum.example.com:7586 \
    -e CA_CERT=/certs/ca.crt \
    -e CERT=/certs/client.crt \
    -e KEY=/certs/client.key \
    ghcr.io/csmith/goplum/dashboard
```

## Flags

Each flag can also be set as an environment variable, e.g. `SERVER` or
`CA_CERT`.

| Flag        | Default          | Description                                       |
|-------------|------------------|---------------------------------------------------|
| `--listen`  | `:8080`          | Address to serve the dashboard on                 |
| `--server`  | `localhost:7586` | Address of the GoPlum API                         |
| `--ca-cert` | `ca.crt`         | Path to the certificate authority's certificate   |
| `--cert`    | `client.crt`     | Path to the client certificate                    |
| `--key`     | `client.key`     | Path to the client private key                    |

Logging can be configured with `--log.level` and `--log.format` (`text` or
`json`).
