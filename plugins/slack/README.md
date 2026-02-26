# Slack plugin

The slack plugin provides alerts that send messages to Slack channels.

## Alerts

### slack.message

```goplum
alert slack.message "example" {
  url = "https://hooks.slack.com/services/XXXXXXXXX/00000000000/abcdefghijklmnopqrstuvwxyz"
}
```

Sends a Slack message via a Slack incoming webhook URL. To enable incoming webhooks you will need
to create a Slack app in your workspace, enable the "Incoming Webhooks" feature, and then create
a webhook for the channel you want messages to be displayed in.
