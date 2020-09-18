package goplum

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_UsesHardcodedCheckDefaults(t *testing.T) {
	config, err := LoadConfig(strings.NewReader(`
		{
			"checks": [
				{
					"name": "check1",
					"type": "test.test",
					"interval": 0
				},
				{
					"name": "check2",
					"type": "test.test",
					"alerts": [],
					"good_threshold": 0,
					"failing_threshold": 0
				}
			]
		}
	`))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	assert.Equal(t, 2, len(config.Checks), "Should have parsed two checks from the config")
	assert.Equal(t, DefaultSettings, config.Checks[0].CheckSettings, "Check 0 should have default settings")
	assert.Equal(t, DefaultSettings, config.Checks[1].CheckSettings, "Check 1 should have default settings")
}

func TestLoadConfig_UsesConfigDefaults(t *testing.T) {
	config, err := LoadConfig(strings.NewReader(`
		{
			"defaults": {
				"interval": "120s",
				"alerts": ["foo"],
				"good_threshold": 5,
				"failing_threshold": 6
			},
			"checks": [
				{
					"name": "check1",
					"type": "test.test",
					"interval": 0
				},
				{
					"name": "check2",
					"type": "test.test",
					"alerts": [],
					"good_threshold": 0,
					"failing_threshold": 0
				}
			]
		}
	`))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := CheckSettings{
		Alerts:           []string{"foo"},
		Interval:         Duration(time.Second * 120),
		GoodThreshold:    5,
		FailingThreshold: 6,
	}

	assert.Equal(t, 2, len(config.Checks), "Should have parsed two checks from the config")
	assert.Equal(t, expected, config.Checks[0].CheckSettings, "Check 0 should have default settings")
	assert.Equal(t, expected, config.Checks[1].CheckSettings, "Check 1 should have default settings")
}

func TestLoadConfig_MixedDefaults(t *testing.T) {
	config, err := LoadConfig(strings.NewReader(`
		{
			"defaults": {
				"alerts": ["foo"],
				"good_threshold": 5
			},
			"checks": [
				{
					"name": "check1",
					"type": "test.test",
					"interval": "120s"
				},
				{
					"name": "check2",
					"type": "test.test",
					"alerts": ["bar"],
					"good_threshold": 7
				}
			]
		}
	`))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	assert.Equal(t, 2, len(config.Checks), "Should have parsed two checks from the config")

	assert.Equal(t, CheckSettings{
		Alerts:           []string{"foo"},
		Interval:         Duration(time.Second * 120),
		GoodThreshold:    5,
		FailingThreshold: DefaultSettings.FailingThreshold,
	}, config.Checks[0].CheckSettings, "Check 0 should have merged settings")

	assert.Equal(t, CheckSettings{
		Alerts:           []string{"bar"},
		Interval:         DefaultSettings.Interval,
		GoodThreshold:    7,
		FailingThreshold: DefaultSettings.FailingThreshold,
	}, config.Checks[1].CheckSettings, "Check 1 should have merged settings")
}
