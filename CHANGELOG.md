# Changelog

## Unreleased

### Added

- Add `plumdash`, a web dashboard showing the state of each check and its
  recent results, with a Docker image at `ghcr.io/csmith/goplum/dashboard`
- Include each check's recent results in the API's `Check` message
- Add a `link` check setting, sent with alerts to point at a runbook or
  dashboard. It supports `{name}` and `{type}` placeholders, and can be set in
  `defaults`, a group's `defaults`, or on a check
- Add an `ms` duration unit for milliseconds
- Add a `network` setting to `http.get` and `http.healthcheck` checks to
  connect only over IPv4 (`tcp4`) or IPv6 (`tcp6`)
- Support `alerts = ["-"]` for a check that sends no alerts, such as to override
  the default alerts. It never matches an alert named `-`

### Changed

- Write goplum's logs as structured `key=value` pairs, with new `--log.level`
  and `--log.format` flags to set the minimum level or switch to JSON
- Reject a `network.portscan` `address` with an IPv6 address in brackets, as
  the check passed without scanning anything
- Reject `http.get`, `http.healthcheck` and `http.webhook` URLs that aren't
  absolute `http` or `https` URLs, instead of failing every run
- Reject malformed `prometheus` plugin `headers` in config instead of failing
  every check
- Running checks are now cancelled on shutdown, and goplum waits up
  to 5 seconds for alerts being sent to finish before saving state
- Docker images are now built with the upstream `golang` image
- Reject numbers without a unit for duration settings, as they were read as
  nanoseconds; `interval = 30` must now be written `interval = 30s`
- Reject durations ending in a number without a unit, such as `1h30`, which
  was read as `1h`, and durations too long to represent, which wrapped around
  to negative values
- Reject `http.get` `min_status_code` and `max_status_code` values outside
  100 to 599, or with the minimum above the maximum, instead of failing every
  run
- Reject an `interval` or `timeout` of zero, as the check was run repeatedly
  and in parallel, or failed straight away

### Fixed

- Fix goplum sometimes crashing when `plumctl results` disconnects, and stop
  slow API clients streaming results from delaying checks
- Fix `plumctl` crashing on every command that talks to the API
- Fix negative numbers being rejected in config files, so `pushover.message`
  priorities of `-1` and `-2` can be set
- Fix `plumctl unsuspend` saying "suspend" in its error message
- Exit with a non-zero status when a `plumctl` command fails
- Fix `plumctl results` reporting an error when the server ends the stream
- Fix the API sending check results' `check_time` fact without a value; it is
  now sent as an integer number of nanoseconds
- Shorten `pushover.message` alerts to Pushover's 1024 character limit, instead
  of the alert being rejected
- Remove the claim that `pushover.message` alerts are disabled after an error
  response; every alert is still attempted
- Fix `smtp.send` alerts having malformed headers when a check's detail spans
  multiple lines; the subject is now kept to a single line of up to 200
  characters
- Shorten `discord.message`, `slack.message` and `twilio.sms` alerts to fit each
  service's message limit, instead of the alert being rejected or cut off
- Fix a crash instead of an error message when the config path is
  a directory, or the config has a syntax error near the end of a file
- Fix a check's `alerts` changing the `alerts` that later checks in the same
  group get from the group's `defaults`
- Fix group `defaults` being ignored when they set a zero value, such as
  `reminder = 0s` or `link = ""`
- Fix alerts suppressed by one group's `alert_limit` counting towards the
  limits of the check's other groups, and a group listed twice on a check
  counting each alert twice
- Fix checks listing several `alerts` also sending to alerts whose names merely
  start with the first name, end with the last, or contain one in the middle
- Fail `http.healthcheck` checks when the response's `status` is missing or
  not recognised, instead of giving an indeterminate result
- Include the HTTP status in `http.healthcheck` failures when the response
  can't be decoded, e.g. for a `502 Bad Gateway` error page
- Fail `http.healthcheck` checks on 4xx and 5xx responses, even if the body's
  `status` is `pass`
- Stop the heartbeat plugin logging heartbeat IDs, which act as the password
  for sending a heartbeat
- Close heartbeat plugin connections that are idle or too slow to send their
  request, instead of keeping them open forever
- Fix resumed checks sometimes waiting up to a minute before running again
- Fix config errors and check positions reporting the wrong line after
  comments
- Ignore saved state from the future, such as after the clock was corrected,
  which stopped checks running until the clock caught up

## 1.2.0 - 2026-11-04 

### Added

- The config file can now `include` other files. Defaults may only
  be specified in the main config file, but everything else can be
  split up ([issue #8](https://github.com/csmith/goplum/issues/8))
- A new `prometheus` plugin provides a `prometheus.query` check
  that runs PromQL queries

### Changed

- Potentially sensitive fields from alert configurations and
  HTTP check URLs are no longer passed to alerts
- Config file errors now include the file and line
- Each plugin may only have a single configuration block;
  previously multiple were silently merged with undefined
  precedence
- A lot of configuration mistakes are now caught at startup
  and raise errors, instead of failing checks or silently
  being ignored
- Interrupted portscans are now treated as an indeterminate
  result not good.
- Alert when a check first settles as failing, instead of
  sending a "still failing" reminder

### Fixed

- Fix a crash at startup when an alert or check type has no plugin prefix
- Webhook and SMTP alerts now have a 20 second timeout instead of blocking
  checks indefinitely
- Fix `network.portscan` hanging when every port in the range is open
- Fix SNMP checks leaking a UDP socket on every execution
- Fix `snmp.string` panicking on every string value
- Fix `snmp.int` passing when an OID returns no value instead of a number
- Allow priority -2 in `pushover.message` alerts
- Fix HTTP checks leaking a connection when the status code is unexpected
- Fix garbled `plumctl` error messages
- Fix HTTP checks panicking when the URL cannot be parsed
- Fix check history only keeping 9 results, so a threshold of 10 was never
  reached

## 1.1.1 - 2026-10-20

### Fixed

- Require Go 1.27.3 and update the base image to dockerbase 1.20261020.1,
  for fixes to the eMudhra intermediate CA compromise
- Update gRPC to fix HTTP/2 vulnerabilities

## 1.1.0 - 2026-04-25

### Features

* The http.webhook alert method can now be configured to send arbitrary
  headers (e.g. auth tokens).
* The http.webhook alert method can now be configured with a secret;
  if specified it will send an X-Goplum-Signature header with a SHA256
  digest of the payload.

## 1.0.0 - 2026-02-26

### Breaking changes

* Removed support for runtime plugins. Plugins are now selected at compile
  time using build tags. The `plugins` option no longer exists. This should
  be a transparent change for users of the Docker image.
* Go package named changed from `github.com/csmith/goplum` to
  `chameth.com/goplum`.

### Features

* Add support for groups. These can provide default settings to checks,
  and rate limiting for alerts. Checks can be members of any number of groups,
  and alerts will be suppressed if any goes over the rate limit.
* Add support for reminders. If set, a failing check will repeatedly emit
  alerts with the given period.

### Other changes

* Various dependency updates
* Converted documentation to markdown

## 0.8.0 - 2025-06-13

### Changes

* Various dependency updates
* Dockerfile now uses `ghcr.io/greboid/dockerbase/nonroot` as a base

## 0.7.0 - 2023-06-01

### Features

* The number of runners used for concurrent checks is now configurable
  via a command-line option.

### Changes

* Update to Go 1.20
* Various dependency updates

## 0.6.0 - 2021-09-13

### Features

* Checks may now return "facts" as part of their results, for example server
  response times or HTTP codes. These are not handled by Goplum itself but are
  passed on to alerts and API clients.
  ([#34](https://github.com/csmith/goplum/issues/34))
  * All results will get a `check_time` fact by default, containing the time
    the check took to execute.
* Added `portscan` check to the `network` plugin, which fails if unpermitted
  ports are found to be open.

### Changes

* Checks can implement the `LongRunning` interface if they intentionally run
  over long periods of time. This allows them to extend the default timeout
  based on their own unique configuration.
* Arrays in goplum's config file can now contain types other than strings.
  See the [syntax guide](docs/syntax.md) for more details.
* Improved formatting of error messages when an unexpected token is found
  in the config file.
* Update to Go 1.17
* Official container images are how hosted on GitHub (`ghcr.io/csmith/goplum`)
  rather than DockerHub. Changed base images to those from
  [csmith/dockerfiles](https://github.com/csmith/dockerfiles).

## 0.5.0 - 2021-02-26

### Features

* Added `smtp` plugin to send alerts by e-mail
  ([#4](https://github.com/csmith/goplum/issues/4))
* Added `msteams` plugin for sending messages to
  Microsoft Teams.
* Added `discord` plugin to send alerts to Discord
  ([#16](https://github.com/csmith/goplum/issues/16))
* Added `snmp` plugin to check values from SNMP
  ([#26](https://github.com/csmith/goplum/issues/26))
* The `http.get` check now allows you to specify a range of
  acceptable status codes, which lets you check that a URL
  returns an error.

### Changes

* Docker images now include a `/notices` directory containing
  copyright information for all compiled code.
* The `Config` field in the `AlertDetails` struct passed to
  alerts is now correctly populated.
* The gRPC API and the `plumctl` client now require TLS 1.3
  or greater.
* Goplum is now compiled with Golang 1.16.
* Goplum is now stricter about validating its configuration
  on startup:
  * Checks can no longer have invalid alerts (i.e., an
    `alerts` property that doesn't match any configured alert).
    ([#36](https://github.com/csmith/goplum/issues/36))
  * Only one "defaults" block may exist in the configuration file.
    ([#37](https://github.com/csmith/goplum/issues/37))

## 0.4.0 - 2020-10-22

### Features

* Verbose logging can now be suppressed with the quiet flag
  ([#29](https://github.com/csmith/goplum/issues/29))
* GoPlum now exposes a gRPC API to allow for custom tooling
  and integration with other services.
  See the [API docs](docs/api.md) for further information.
  ([#30](https://github.com/csmith/goplum/issues/30))
* Checks can now be suspended and resumed (via the API), for
  e.g. planned maintenance
  ([#31](https://github.com/csmith/goplum/issues/31))
* Added `plumctl` command line tool that uses the API to
  interact with a GoPlum instance.
  See the [plumctl docs](docs/plumctl.md) for further
  information.
* The configuration file now supports plugin-specific
  config via `plugin <identifier> {}` blocks.
  ([#33](https://github.com/csmith/goplum/issues/33))
* Added a `heartbeat` plugin to enable monitoring or periodic/offline
  tasks such as cron jobs. See the
  [heartbeat documentation](plugins/heartbeat) for more information.
  ([#32](https://github.com/csmith/goplum/issues/32))

### Changes

* Small improvement to error messages for invalid config keys
* GoPlum now errors if checks or alerts have duplicate names
  (this was previously documented but not enforced)
* Plugins can now implement the Validator interface to check
  their own configuration in the same way as Alerts and Checks
* Checks can now implement the Stateful interface to backup
  and restore their internal state

## 0.3.0 - 2020-09-27

### Features

* The `http.get` check can now make sure content *isn't* present
  ([#28](https://github.com/csmith/goplum/issues/28))
* The `http.get` check now supports basic authentication
  ([#7](https://github.com/csmith/goplum/issues/7))
* Added a timeout setting for checks, and updated bundled plugins
  to respect it ([#10](https://github.com/csmith/goplum/issues/10))
* Check state is now persisted across restarts
  ([#14](https://github.com/csmith/goplum/issues/14))
* Added `twilio.call` alert for announcing alerts using TTS
  over a phone call.
* Added `http.healthcheck` check for monitoring healthcheck endpoints.
  ([#2](https://github.com/csmith/goplum/issues/2))

### Changes

* Added boolean support to configuration files
  * The following are now reserved keywords: `yes`, `no`, `true`, `false`, `on`, `off`
* Keywords in configuration files are now case-insensitive
* The "network" argument in `network.connect` is now actually optional,
  per its documentation.

## 0.2.0 - 2020-09-24

### Features

* Switched to a custom config format instead of JSON
* Added `exec.command` check ([#9](https://github.com/csmith/goplum/issues/9))
* Added `pushover.message` alert ([#23](https://github.com/csmith/goplum/issues/23))
* Added `network.connect` check ([#1](https://github.com/csmith/goplum/issues/1))
* The configuration path is now configurable via a flag or env var
  ([#13](https://github.com/csmith/goplum/issues/13))

### Changes

* Checks are now executed in parallel
* Fixed potential resource leak in several checks/alerts using HTTP requests
* Fixed timing issues if a check took a long time to execute
* Fixed issue with connection reuse when multiple http.get checks ran
  against the same host ([#21](https://github.com/csmith/goplum/issues/21))

## 0.1.0 - 2020-09-17

_Initial release._
