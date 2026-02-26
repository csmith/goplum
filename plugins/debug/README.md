# Debug plugin

The debug plugin provides checks and alerts for use when developing or testing Goplum.
These probably aren't of interest to you if you just want to run Goplum.

## Checks

### debug.random

```goplum
check debug.random "example" {
  percent_good = 0.8
}
```

Passes or fails at random. If the `percent_good` parameter is specified then checks will pass with
that probability (i.e. a value of 0.8 means a check has an 80% chance to pass).

## Alerts

### debug.sysout

```goplum
alert debug.sysout "example" {}
```

Prints alerts to system out, prefixed with 'DEBUG ALERT'.
