package internal

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

func DecodeSettings(src *map[string]interface{}, targets ...interface{}) error {
	unused := make(map[string]int)

	for i := range targets {
		dst := targets[i]

		metadata := mapstructure.Metadata{}
		dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			TagName:          "config",
			Metadata:         &metadata,
			Result:           dst,
		})
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
