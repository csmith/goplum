# Groups

Groups in Goplum allow you to organize related checks and prevent alert storms by limiting the number of notifications sent from a collection of checks within a specified time window.

## Overview

Groups serve two main purposes:

1. **Alert Storm Prevention**: Limit the total number of alerts sent from a group of checks within a time window
2. **Configuration Management**: Apply common default settings to multiple related checks

## Basic Usage

### Defining Groups

```goplum
group "webservices" {
  alert_limit = 3
  alert_window = 10m
}
```

This creates a group called "webservices" that will send at most 3 alerts within any 10-minute window.

### Assigning Checks to Groups

```goplum
check http.get "api-server" {
  url = "https://api.example.com/health"
  groups = ["webservices"]
}

check http.get "web-frontend" {
  url = "https://www.example.com"
  groups = ["webservices"]
}
```

Both checks belong to the "webservices" group and share the same alert limits.

## Alert Limiting

When multiple checks in a group fail simultaneously, Goplum will:

1. Send the first N alerts normally (where N = `alert_limit`)
2. The final alert before reaching the limit includes a warning: `[GROUP ALERT LIMIT REACHED: groupname]`
3. Suppress all subsequent alerts from that group until the sliding time window allows new alerts

### Example Behavior

With `alert_limit = 2` and `alert_window = 10m`:

- **Alert 1**: "Service A is down: connection timeout"
- **Alert 2**: "Service B is down: 500 error [GROUP ALERT LIMIT REACHED: webservices]"
- **Alert 3**: *Suppressed* (logged but not sent)
- **Alert 4**: *Suppressed* (logged but not sent)

New alerts will resume once older alerts fall outside the sliding time window.

## Group Defaults

Groups can contain a nested `defaults` block to apply common settings to all checks in the group:

```goplum
group "databases" {
  alert_limit = 5
  alert_window = 15m

  defaults {
    interval = 60s
    timeout = 30s
    reminder = 10m
    alerts = ["sms", "email"]
    good_threshold = 3
    failing_threshold = 3
  }
}
```

Checks assigned to this group will inherit these default values unless explicitly overridden.

## Multiple Groups

Checks can belong to multiple groups:

```goplum
check http.get "payment-api" {
  url = "https://payments.example.com/health"
  groups = ["webservices", "critical-systems"]
}
```

When a check belongs to multiple groups:

- **Alert limiting**: If ANY group has reached its limit, the alert is suppressed
- **Defaults inheritance**: Settings are applied in order (later groups override earlier ones)

## Global Defaults Integration

Groups can be specified in the global defaults block:

```goplum
defaults {
  interval = 30s
  groups = ["monitoring"]
}

check tcp.connect "database" {
  host = "db.example.com"
  port = 5432
  # Inherits groups = ["monitoring"] from defaults
}
```

## Configuration Reference

### Group Block

```goplum
group "<name>" {
  alert_limit = <number>     # optional, max alerts per window (0 = unlimited)
  alert_window = <duration>  # optional, time window for alert limiting

  defaults {
    # Any check setting except 'groups'
  }
}
```

### Group Assignment

```goplum
# In check blocks or global defaults
groups = ["group1", "group2", ...]
```
