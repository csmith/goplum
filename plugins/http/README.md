# HTTP plugin

The HTTP plugin provides checks for HTTP services, and HTTP-based alerts.

## Checks

### http.get

```goplum
check http.get "example" {
  url = "https://www.example.com/"

  content = "Example Domain"
  content_expected = true

  min_status_code = 200
  max_status_code = 399

  certificate_validity = 10d

  auth {
    username = "acidburn"
    password = "HackThePlanet"
  }
}
```

Sends an HTTP GET request to the given URL.

The `min_status_code` and `max_status_code` parameter specify the allowed range for the
response's HTTP status code. These default to `100` and `399` (i.e., any non-error response).
If you wish to test a URL denies access to unauthenticated users, for example, you can use
`min_status_code=401` `max_status_code=403`; to check for a single status code the two parameters
can be the same, e.g. `min_status_code=418` `max_status_code=418`

If the `content` parameter is specified then the response body is checked for the exact string.
By default the string must be present, and the check will fail if it is not. If `content_expected`
is set to `false` then the string must NOT be present, and the check will fail if it is.

If the `certificate_validity` parameter is specified, then the connection must have
been made over TLS, and the returned certificate must be valid for at least the given duration
from now. (An expired or untrusted certificate will cause a failure regardless of this setting.)

If the `auth` settings are provided, they will be sent in a Basic authentication header. Note
that basic authentication isn't encrypted, so shouldn't be used over an insecure connection.

### http.healthcheck

```goplum
check http.healthcheck "example" {
  url = "https://www.example.com/health"
  check_components = true
  auth {
    username = "acidburn"
    password = "HackThePlanet"
  }
}
```

Retrieves the current status from a HTTP healthcheck endpoint. The endpoint is expected
to return JSON in a manner compatible with
[draft-inadarei-api-health-check-04](https://tools.ietf.org/id/draft-inadarei-api-health-check-04.html).

If the `check_components` setting is enabled, the state of each component/dependency
reported in the healthcheck response will also be verified. This means if the overall service
status is `pass` but a component is `fail` then the Goplum check will fail.

If the `auth` settings are provided, they will be sent in a Basic authentication header. Note
that basic authentication isn't encrypted, so shouldn't be used over an insecure connection.

## Alerts

### http.webhook

```goplum
alert http.webhook "example" {
  url = "https://www.example.com/incoming"
}
```

Sends alerts as a POST request to the given webhook URL with a JSON payload:

```json
{
  "text": "Check 'Testing' is now good, was failing.",
  "name": "Testing",
  "type": "debug.random",
  "config": {
    "percent_good": 0.8
  },
  "last_result": {
    "state": "failing",
    "time": "2020-09-17T17:55:02.224973486+01:00",
    "detail": "Random value 0.813640 greater than percent_good 0.800000"
  },
  "previous_state": "failing",
  "new_state": "good"
}
```
