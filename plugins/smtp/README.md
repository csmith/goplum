# SMTP plugin

The SMTP plugin provides alerts that send e-mail messages over SMTP.

## Alerts

### smtp.send

```goplum
alert smtp.send "example" {
  server = "mail.example.com:25"
  username = "goplum"
  password = "example"
  subject_prefix = "ALERT: "
  from = "alerts@example.com"
  to = "sysadmin@example.com"
}
```

Sends an e-mail message via an SMTP server. All parameters are required except
for `subject_prefix` which defaults to `"Goplum alert: "`.

If the SMTP server supports STARTTLS, the connection will switch to use TLS
prior to sending any authentication details.

If you do not run your own SMTP server, you might consider using a dedicated
service such as [mailgun](https://www.mailgun.com/) or
[AWS SES](https://aws.amazon.com/ses/), both of which you can access over SMTP.
