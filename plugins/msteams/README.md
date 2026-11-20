# Microsoft Teams plugin

The msteams plugin provides alerts that send messages to Microsoft Teams channels and chats.

## Alerts

### msteams.message

```goplum
alert msteams.message "example" {
  url = "https://.../workflows/.../triggers/manual/paths/invoke?..."
  title = "Service alert"
}
```

Sends an Adaptive Card to a Teams webhook created with the Workflows app. Create one
from the channel's "Workflows" menu using a template such as "Send webhook alerts to a
channel", then copy the webhook URL it gives you. See
[the Microsoft documentation](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook)
for details.

The `title` parameter is optional, and defaults to "Goplum alert".
