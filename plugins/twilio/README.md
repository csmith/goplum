# Twilio plugin

The twilio plugin provides alerts that use the Twilio API.

To use the Twilio alerts you must have a funded Twilio account, and configure the
SID, Token, and From/To phone numbers.

## Alerts

### twilio.call

```goplum
alert twilio.call "example" {
  sid = "twilio sid"
  token = "twilio token"
  from = "+01 867 5309"
  to = "+01 867 5309"
}
```

Initiates a phone call using the Twilio API. When the call is answered the alert will be spoken
using text-to-speech.

### twilio.sms

```goplum
alert twilio.sms "example" {
  sid = "twilio sid"
  token = "twilio token"
  from = "+01 867 5309"
  to = "+01 867 5309"
}
```

Sends SMS alerts using the Twilio API.
