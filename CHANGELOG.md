# Changelog

## unreleased

### Breaking changes

* Removed support for runtime plugins. Plugins are now selected at compile
  time using build tags. The `plugins` option no longer exists. This should
  be a transparent change for users of the Docker image.

### Features

* Add support for groups. These can provide default settings to checks,
  and rate limiting for alerts. Checks can be members of any number of groups,
  and alerts will be suppressed if any goes over the rate limit.
* Add support for reminders. If set, a failing check will repeatedly emit
  alerts with the given period.

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
  See the [syntax guide](docs/syntax.adoc) for more details.
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
  See the [API docs](docs/api.adoc) for further information.
  ([#30](https://github.com/csmith/goplum/issues/30))
* Checks can now be suspended and resumed (via the API), for
  e.g. planned maintenance
  ([#31](https://github.com/csmith/goplum/issues/31))
* Added `plumctl` command line tool that uses the API to
  interact with a GoPlum instance.
  See the [plumctl docs](docs/plumctl.adoc) for further
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
