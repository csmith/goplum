![Goplum](.images/banner.png?raw=true)

Goplum is an extensible monitoring and alerting daemon designed for
personal infrastructure and small businesses. It can monitor
websites and APIs, and send alerts by a variety of means if they go down.

## Table of Contents

- [Features](#features)
- [Getting started](#getting-started)
- [Usage](#usage)
- [Advanced topics](#advanced-topics)
- [Licence and credits](#licence-and-credits)

## Features

**Highly extensible**: Goplum supports plugins written in Go
to define new monitoring rules and alert types, and it has an API
for integration with other services and tools.

**Get alerts anywhere**: Goplum supports a variety of ways to
alert you out-of-the-box:

| | | |
|---|---|---|
| ![Discord logo](.images/alerts/discord.png) Discord | ![Mail icon](.images/alerts/mail.png) E-mail | ![Microsoft Teams icon](.images/alerts/msteams.png) Microsoft Teams |
| ![Phone icon](.images/alerts/phone.png) Phone call (via Twilio) | ![Pushover logo](.images/alerts/pushover.png) Pushover | ![Slack logo](.images/alerts/slack.png) Slack |
| ![SMS icon](.images/alerts/sms.png) SMS (via Twilio) | ![Webhook logo](.images/alerts/webhook.png) Webhook | |

**Lightweight**: Goplum has a small resource footprint, and all
checks are purpose-written in Go. No need to worry about chains
of interdependent scripts being executed.

**Heartbeat monitoring**: Have an offline service or a cron job
that you want to monitor? Have it send a heartbeat to Goplum
periodically and get alerted if it stops.

**Simple to get started**: If you're set up to run services in
containers, you can get Goplum up and running in a couple of minutes.

**Alert storm prevention**: Group related checks together and set
limits on how many alerts can be sent from a group within a time
window, preventing notification overload when multiple services fail.

## Getting started

### Basic configuration

Goplum works by running a number of _checks_ (which test to see
if a service is working or not), and when they change state running
an _alert_ that notifies you about the problem.

Checks and alerts are both defined in Goplum's config file. A
minimal example looks like this:

```
check http.get "example.com" {
  url = "https://example.com/"
}

alert twilio.sms "Text Bob" {
  sid = "sid"
  token = "token"
  from = "+01 867 5309"
  to = "+01 867 5309"
}
```

1. Goplum's configuration consists of "blocks". The contents
   of the blocks are placed within braces (`{}`). This is
   a "check block"; these will likely make up the bulk of your
   configuration.
   - `http.get` is the type of check we want to execute. The
     `http` part indicates it comes from the HTTP plugin, while
     the `get` part is the type of check.
   - All checks (and alerts) have a unique name, in this case
     we've called it "example.com". If a check starts to fail,
     the alert you receive will contain the check name.
2. Parameters for the check are specified as `key = value`
   pairs within the body of the check. The documentation for
   each check and alert will explain what parameters are available,
   and whether they're required or not.
3. Like checks, alerts have both a type and a name. Here we're
   using the `sms` alert from the `twilio` plugin, and we've
   named it `Text Bob`.
4. The `twilio.sms` alert has a number of required parameters
   that define the account you wish to use and the phone numbers
   involved. These are all just given as `key = value` pairs.

This simple example will try to retrieve https://example.com/
every thirty seconds. If it fails three times in a row, a text
message will be sent using Twilio. Then if it consistently starts
passing again another message will be sent saying it has recovered.
Don't worry - these numbers are all configurable: see the
[Default Settings](#default-settings) section.

In this example we used the `http.get` check and the `twilio.sms`
alert. See the [Available checks and alerts](#available-checks-and-alerts) section for details
of the other types available by default.

There is a complete [syntax guide](docs/syntax.md) available
in the `docs` folder if you need to look up a specific aspect of
the configuration syntax.

### Docker

The easiest way to run Goplum is using Docker. Goplum doesn't require
any privileges, settings, or ports exposed to get a basic setup
running. It just needs the configuration file, and optionally a
persistent file it can use to persist data across restarts:

Running it via the command line:

```shell
# Create a configuration file
vi goplum.config

# Make a 'tombstone' file that Goplum's unprivileged user can write
touch goplum.tomb
chown 65532:65532 goplum.tomb

# Start goplum
docker run -d --restart always \
   -v $(PWD)/goplum.conf:/goplum.conf:ro \
   -v $(PWD)/goplum.tomb:/tmp/goplum.tomb \
   ghcr.io/csmith/goplum
```

Or using Docker Compose:

```yaml
version: "3.8"

services:
  goplum:
    image: ghcr.io/csmith/goplum
    volumes:
      - ./goplum.conf:/goplum.conf
      - ./goplum.tomb:/tmp/goplum.tomb
    restart: always
```

The `latest` tag points to the latest stable release of Goplum, if
you wish to run the very latest build from this repository you can
use the `dev` tag.

### Without Docker

While Docker is the easiest way to run Goplum, it's not that hard to run it
directly on a host without containerisation. See the
[installing without Docker](docs/baremetal.md) guide for more information.

## Usage

### Available checks and alerts

All checks and alerts in Goplum are implemented as plugins. The following are maintained in
this repository and are available by default in the Docker image. Each plugin has its own
documentation, that explains how its checks and alerts need to be configured.

| Plugin | checks | alerts |
|---|---|---|
| [discord](plugins/discord) | - | message |
| [http](plugins/http) | get, healthcheck | webhook |
| [network](plugins/network) | connect, portscan | - |
| [heartbeat](plugins/heartbeat) | received | - |
| [msteams](plugins/msteams) | - | message |
| [pushover](plugins/pushover) | - | message |
| [slack](plugins/slack) | - | message |
| [smtp](plugins/smtp) | - | send |
| [snmp](plugins/snmp) | int, string | - |
| [twilio](plugins/twilio) | - | call, sms |
| [debug](plugins/debug) | random | sysout |
| [exec](plugins/exec) | command | - |

The `docs` folder contains [an example configuration file](docs/example.conf)
that contains an example of every check and alert fully configured.

### Settling and thresholds

When Goplum first starts, it is not aware of the current state of your services.
To avoid immediately sending alerts when the state is determined, Goplum waits for
each check to **settle** into a state, and then only alerts when that state
subsequently changes.

Goplum uses **thresholds** to decide how many times a check result must happen in
a row before it's considered settled. By default, this the threshold is two "good"
results or two "failing" results, but this can be changed - see [Default Settings](#default-settings).

For example:

```
 Goplum                    Failing            Recovery
 starts                     Alert               Alert
   ↓                          ↓                   ↓
    ✓ ✓ ✓ ✓ ✓ ✓ ✓ 🗙 ✓ ✓ ✓ 🗙 🗙 🗙 🗙 🗙 ✓ 🗙 ✓ 🗙 ✓ ✓ ✓ ✓ ✓ ✓ ✓ ✓ …
       ↑                      ↑                   ↑
  State settles          State becomes       State becomes
    as "good"              "failing"            "good"
```

### Default Settings

All checks have a number of additional settings to control how they work. These can be
specified for each check, or changed globally by putting them in the "defaults" section.
If they're not specified then Goplum's built-in defaults will be used.

| Setting | Description | Default |
|---|---|---|
| `interval` | Length of time between each run of the check. | `30s` |
| `timeout` | Maximum length of time the check can run for before it's terminated. | `20s` |
| `alerts` | A list of alert names to trigger when the service changes state. Supports '\*' as a wildcard. | `["*"]` |
| `groups` | A list of group names this check belongs to. | `[]` |
| `failing_threshold` | The number of checks that must fail in a row before a failure alert is raised. | `2` |
| `good_threshold` | The number of checks that must pass in a row before a recovery alert is raised. | `2` |
| `reminder` | If set, a reminder alert will be sent periodically while a check remains in a failing state. A value of `0` disables reminders. The actual interval between reminders will be rounded up to the next multiple of the check interval. | `0` (disabled) |

For example, to change the `interval` and `timeout` for all checks:

```goplum
defaults {
  interval = 2m
  timeout = 30s
}
```

Or to specify a custom timeout and alerts for one check:

```goplum
check http.get "get" {
  url = "https://www.example.com/"
  timeout = 60s
  alerts = ["Text Bob"]
}
```

### Groups and Alert Storm Prevention

When multiple services fail simultaneously (e.g., when a server goes down),
you might receive dozens of alerts at once. Groups help prevent this alert
storm by limiting how many alerts can be sent within a time window.

To create a group:

```goplum
group "webservices" {
  alert_limit = 3           # Maximum 3 alerts from this group
  alert_window = 10m        # Within a 10 minute window

  defaults {
    interval = 30s          # Default settings for checks in this group
    timeout = 10s
  }
}
```

Then add checks to the group:

```goplum
check http.get "website" {
  url = "https://example.com/"
  groups = ["webservices"]
}

# ... other checks ...

check http.get "api" {
  url = "https://api.example.com/"
  groups = ["webservices"]
}
```

With this configuration, if all the websites fail when their server crashes,
you'll receive at most 3 alerts in any 10-minute period.

Checks can belong to multiple groups, and groups can have their own default
settings that override the global defaults but can be overridden by individual
check settings.

## Advanced topics

### Selecting plugins

By default all plugins in the source tree will be built when compiling Goplum.
You can exclude plugins by setting the appropriate tag when building; for example,
to exclude the Discord and Slack plugin:

```shell
go build -tags "nodiscord,noslack" ./cmd/plugins
```

### gRPC API

In addition to allowing plugins to define new checks and alerts, GoPlum provides a gRPC
API to enable development of custom tooling and facilitate use cases not supported by
GoPlum itself (e.g. persisting check history indefinitely). The API is currently in
development; more information can be found in the [API documentation](docs/api.md).

### plumctl command-line tool

Goplum comes with `plumctl`, a command-line interface to inspect the state of Goplum
as well as perform certain operations such as pausing and resuming a check. `plumctl`
uses the [gRPC API](#grpc-api). For more information see the
[plumctl documentation](docs/plumctl.md).

## Licence and credits

Goplum is licensed under the MIT licence. A full copy of the licence is available in
the [LICENCE](LICENCE) file.

Some icons in this README are modifications of the Material Design icons created by Google
and released under the [Apache 2.0 licence](https://www.apache.org/licenses/LICENSE-2.0.html).

Goplum makes use of a number of third-party libraries. See the [go.mod](go.mod) file
for a list of direct dependencies. Users of the docker image will find a copy of the
relevant licence and notice files under the `/notices` directory in the image.
