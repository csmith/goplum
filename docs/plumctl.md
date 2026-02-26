# plumctl

`plumctl` is a command-line tool to remote control a GoPlum instance using the
[GoPlum API](api.md).

## Getting started

To install the latest version:

```
$ go install chameth.com/goplum/cmd/plumctl
```

Then to create a configuration file:

```
$ plumctl init plum.example.com:7586
Config created in /home/user/.config/plumctl.
You must provide your CA certificate, client certificate and client private key:

    CA cert: /home/user/.config/plumctl/ca.crt
Client cert: /home/user/.config/plumctl/client.crt
 Client key: /home/user/.config/plumctl/client.key

You can adjust these paths in the configuration file.
```

As described, you must provide the certificate from your Certificate Authority
and a cert and key pair for plumctl to use when connecting to the API. For more
information on how to generate these see the [API documentation](api.md).

## Commands

### plumctl init \<server:port\>

Creates a new configuration file pointing at the given API endpoint. GoPlum
automatically determines an appropriate configuration file location and will
output it.

### plumctl checks

Lists all checks configured in GoPlum.

Checks that are suspended, not passing, or haven't yet settled are marked
as such in the output.

### plumctl results

Streams check results as they happen. Each line will show the result of
one check that was executed.

### plumctl suspend \<check\>

Suspends the check with the specified name. The check won't execute again
until it is unsuspended.

### plumctl unsuspend \<check\>

Unsuspends a previously suspended check. The check will be queued for
execution as normal.
