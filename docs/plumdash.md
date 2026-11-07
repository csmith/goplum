# plumdash

`plumdash` is a web dashboard that shows the state of each check, along with
its recent results. It connects to GoPlum using the [GoPlum API](api.md).

plumdash has no authentication and serves plain HTTP, so it should only be
exposed on a trusted network or behind a reverse proxy that adds both.

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

The dashboard is then available on port 8080. The page refreshes itself every
30 seconds.

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
