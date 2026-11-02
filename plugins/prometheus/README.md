# Prometheus plugin

The Prometheus plugin runs PromQL queries against a Prometheus-compatible API
(such as Prometheus itself or VictoriaMetrics), so services can be checked using
metrics that are already being collected.

The plugin must be configured with the URL of the API, and can optionally
authenticate, send extra headers, and use custom TLS settings:

```goplum
plugin prometheus {
  # URL is required
  url = "https://prometheus.example.com/"

  # Basic authentication
  auth {
    username = "goplum"
    password = "hunter2"
  }

  # Or a bearer token
  bearer_token = "abc123"

  # Additional headers, such as a tenant ID for Mimir or Cortex
  headers = ["X-Scope-OrgID: tenant-1"]

  tls {
    # CA certificate(s) to verify the server with, instead of the system's
    ca_file = "/etc/goplum/prometheus-ca.pem"

    # Client certificate and key, for mutual TLS
    cert_file = "/etc/goplum/client.pem"
    key_file = "/etc/goplum/client-key.pem"
  }
}
```

Only one of `auth` and `bearer_token` may be specified. Headers are given in
`"Name: Value"` format, and a header may be repeated to send multiple values.
`cert_file` and `key_file` must be specified together, and all TLS files must
be PEM-encoded.

## Checks

### prometheus.query

```goplum
check prometheus.query "example" {
  query = "up{job='example'}"
}
```

Runs an instant query, which should describe the healthy state of the service.
The check passes if the query returns at least one series and every value is
non-zero. It fails if:

- the query returns no series, so a metric that has disappeared fails without
  needing `absent()`;
- any series has a value of zero (or `NaN`); or
- the query is invalid, or the API can't be reached.

This means a plain `up{job='example'}` works as expected: `up` remains present
with a value of zero when a scrape fails. Comparisons also work, as they filter
out series that don't match:

```goplum
check prometheus.query "backup ran" {
  query = "time() - backup_last_success_timestamp_seconds < 86400"
  interval = 10m
}
```

When a query matches several series, a series that vanishes won't be noticed
as long as others remain. To make sure all of them are present, count them:

```goplum
check prometheus.query "all hosts reporting" {
  query = "count(up{job='node'} == 1) == 3"
}
```

PromQL accepts single-quoted label values, so queries don't need to escape
Goplum's double quotes.
