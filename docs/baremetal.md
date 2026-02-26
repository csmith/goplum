# Installing Goplum without Docker

## Compiling

You must have [Go 1.24](https://golang.org/) or newer installed.

```shell
go build -o goplum ./cmd/goplum
```

This will produce a `goplum` binary with all plugins included. You can disable
individual plugins with build tags, e.g. to exclude the Discord and Slack plugins:

```shell
go build -tags "nodiscord,noslack" -o goplum ./cmd/goplum
```

## Paths

By default, Goplum will look for a config file named `goplum.conf` in its working directory.
It will also try to persist data to `/tmp/goplum.tomb` when stopping, and read the same file
when it starts up. These are good defaults for Docker and for local development, but probably
aren't good for a system-wide install.

These can be configured using either flags or environment variables. See
[the flags documentation](flags.md) for more information.

```shell
# Flags
goplum --config /path/to/goplum.conf --tombstone /var/run/goplum.tomb

# Environment
CONFIG=/path/to/goplum.conf TOMBSTONE=/var/run/goplum.tomb goplum
```

## Execution

The goplum binary does not fork, and does not require any special privileges, so it should
be easy enough to integrate with most init systems.

For example, with systemd:

```systemd
[Unit]
Description=Goplum
After=network.target

[Service]
Type=simple
Restart=always
User=goplum
Group=goplum
WorkingDirectory=/tmp
ExecStart=/usr/bin/goplum --config /etc/goplum.conf --tombstone /var/run/goplum.tomb

[Install]
WantedBy=multi-user.target
```

This assumes that:

 * The `goplum` binary is in `/usr/bin/goplum`
 * The config file is at `/etc/goplum.conf`
 * The tombstone will be created at `/var/run/goplum.tomb`
 * You have created a `goplum` user and group.
 * The `goplum` user can read and write to `/var/run/goplum.tomb`
