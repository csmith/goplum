# Heartbeat plugin

The Heartbeat plugin exposes a HTTP endpoint where it can receive "heartbeats" from external
systems. The `heartbeat.received` check then makes sure that a heartbeat has been received
within a given timeframe.

This can be used to monitor batch jobs, such as cron tasks, or just to periodically indicate
service status when the service is not directly monitorable.

The heartbeat plugin must be explicitly configured with the port, and optionally the path,
it will listen on:

```goplum
plugin heartbeat {
  # Port is required
  port = 8080

  # Path is optional, if specified all requests must start with the specified path.
  path = "/heartbeat/"
}
```

> **Tip:** The heartbeat plugin does not support TLS connections. It is strongly recommended
> that you use a reverse proxy such as Nginx, Haproxy or Caddy to perform TLS termination.

Services sending heartbeats must use a 32-character hexadecimal identifier, which
is included in the path when sending a heartbeat. For example with a configured
path of `/heartbeat/`, a service will send a HTTP GET to a URL like:

    https://example.com/heartbeat/bf7a7fa97f112bf949e6de4188d6a991

Requests do not need any specific parameters, headers or payloads.

> **Tip:** You can generate a random ID using the command `openssl rand -hex 16`

It is recommended that callers automatically retry if they do not get a `2xx` status code
in response to the heartbeat, for example with curl:

```shell
$ curl --retry 10 --retry-all-errors https://example.com/heartbeat/bf7a7fa97f112bf949e6de4188d6a991
```

## Checks

### heartbeat.received

```goplum
check heartbeat.received "cron" {
  id = "bf7a7fa97f112bf949e6de4188d6a991"
  within = 1d2h
}
```

Checks to ensure that a heartbeat with the given ID has been received within the time
period (in the example, 1 day 2 hour). Each check must have a unique heartbeat ID.

> **Tip:** Ensure the `within` window is long enough to account for the scheduling time _and_
> any execution time the job may take. For example, a daily cron job that varies in
> duration between 10 and 30 minutes will have up to a 1d20m gap between heartbeats.
>
> Also be aware that daylight savings time changes may cause a ±1h change depending
> on your configured timezone(s).

You may wish to consider setting a custom `interval`, `good_threshold` and
`failing_threshold` for this check. For a heartbeat that should be executed daily a
recommended configuration is:

```goplum
check heartbeat.received "cron" {
  id = "58139f843d770c638420a327e530834d53b691f74c8449a86474b081e22b02f3"
  within = 1d2h
  interval = 30m
  good_threshold = 1
  failing_threshold = 1
}
```

This will trigger an alert between 26 and 26½ hours after the last heartbeat was
received, and revert to a "good" status within half an hour of a subsequent
heartbeat being received.
