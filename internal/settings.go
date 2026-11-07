package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

func DecodeSettings(src *map[string]any, targets ...any) error {
	unused := make(map[string]int)

	for i := range targets {
		dst := targets[i]

		metadata := mapstructure.Metadata{}
		dec, err := mapstructure.NewDecoder(
			&mapstructure.DecoderConfig{
				WeaklyTypedInput: true,
				TagName:          "config",
				Metadata:         &metadata,
				Result:           dst,
				DecodeHook:       requireDurationUnits,
			},
		)
		if err != nil {
			return err
		}

		if err := dec.Decode(*src); err != nil {
			return err
		}

		for _, val := range metadata.Unused {
			unused[val]++
		}
	}

	for key, count := range unused {
		if count == len(targets) {
			return fmt.Errorf("unknown configuration key: %s", key)
		}
	}

	return nil
}

// requireDurationUnits stops plain numbers being decoded into durations, where they'd be read as nanoseconds.
func requireDurationUnits(from, to reflect.Type, data any) (any, error) {
	if to == reflect.TypeFor[time.Duration]() && from != to {
		return nil, fmt.Errorf("expected a duration with units, such as 30s, got %v", data)
	}
	return data, nil
}

// ValidateHTTPURL checks a URL setting is an absolute http or https URL. Errors don't include the URL, as it may
// contain credentials.
func ValidateHTTPURL(raw string) error {
	if len(raw) == 0 {
		return errors.New("missing required argument: url")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", WithoutURL(err))
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("invalid url: must be an absolute http or https URL")
	}

	return nil
}

// WithoutURL strips the URL from a url.Error, as it may contain credentials.
func WithoutURL(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}
	return err
}

const headerNameChars = "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// AddHeaders parses "Name: Value" entries into dst. It rejects anything net/http would refuse to send, so entries
// that pass here can't fail later for being malformed. Errors give the entry's position instead of its content.
func AddHeaders(dst http.Header, headers []string) error {
	for i, h := range headers {
		name, value, ok := strings.Cut(h, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		invalidValue := strings.ContainsFunc(
			value,
			func(r rune) bool {
				return (r < ' ' && r != '\t') || r == 0x7f
			},
		)
		if !ok || len(name) == 0 || strings.Trim(name, headerNameChars) != "" || invalidValue {
			return fmt.Errorf("invalid header at position %d, expected format \"Name: Value\"", i+1)
		}

		dst.Add(name, value)
	}

	return nil
}
