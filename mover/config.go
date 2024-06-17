package mover

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config for the mover.
// Example config file:
/*
{
  "Tokens": [
    "<token 1>",
    "<token 2>"
  ],
  "NightPhaseCategory": "Night Phase",
  "DayPhaseCategory": "Day Phase",
  "TownSquare": "Town Square",
  "StoryTellerRole": "Storyteller",
  "MovementDeadlineSeconds": 15,
  "PerRequestSeconds": 5,
  "MaxConcurrentRequests": 3
}
*/
// The config can also be loaded from the following environment variables:
// BOTC_TOKENS (comma separated tokens)
// BOTC_NIGHT_PHASE_CATEGORY
// BOTC_DAY_PHASE_CATEGORY
// BOTC_TOWN_SQUARE
// BOTC_STORY_TELLER_ROLE
// BOTC_MOVEMENT_DEADLINE_SECONDS (default 15)
// BOTC_PER_REQUEST_SECONDS (default 5)
// BOTC_MAX_CONCURRENT_REQUESTS (default 3)
type Config struct {
	Tokens                  []string
	NightPhaseCategory      string
	DayPhaseCategory        string
	TownSquare              string
	StoryTellerRole         string
	MovementDeadlineSeconds int
	PerRequestSeconds       int
	MaxConcurrentRequests   int
}

// ConfigFromEnv loads a config from environment variables with reasonable defaults.
func ConfigFromEnv() (*Config, error) {
	cfg := &Config{
		MovementDeadlineSeconds: 15,
		PerRequestSeconds:       5,
		MaxConcurrentRequests:   3,
	}

	if v, ok := os.LookupEnv("BOTC_TOKENS"); ok {
		cfg.Tokens = strings.Split(v, ",")
	}
	if v, ok := os.LookupEnv("BOTC_NIGHT_PHASE_CATEGORY"); ok {
		cfg.NightPhaseCategory = v
	}
	if v, ok := os.LookupEnv("BOTC_DAY_PHASE_CATEGORY"); ok {
		cfg.DayPhaseCategory = v
	}
	if v, ok := os.LookupEnv("BOTC_TOWN_SQUARE"); ok {
		cfg.TownSquare = v
	}
	if v, ok := os.LookupEnv("BOTC_STORY_TELLER_ROLE"); ok {
		cfg.StoryTellerRole = v
	}
	if v, ok := os.LookupEnv("BOTC_MOVEMENT_DEADLINE_SECONDS"); ok {
		if d, err := strconv.Atoi(v); err != nil {
			return nil, err
		} else {
			cfg.MovementDeadlineSeconds = d
		}
	}
	if v, ok := os.LookupEnv("BOTC_PER_REQUEST_SECONDS"); ok {
		if d, err := strconv.Atoi(v); err != nil {
			return nil, err
		} else {
			cfg.PerRequestSeconds = d
		}
	}
	if v, ok := os.LookupEnv("BOTC_MAX_CONCURRENT_REQUESTS"); ok {
		if d, err := strconv.Atoi(v); err != nil {
			return nil, err
		} else {
			cfg.MaxConcurrentRequests = d
		}
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	switch {
	case len(c.Tokens) == 0:
		return fmt.Errorf("no discord bot tokens specified")
	case c.NightPhaseCategory == "":
		return fmt.Errorf("night phase voice channel category is empty")
	case c.DayPhaseCategory == "":
		return fmt.Errorf("day phase voice channel category is empty")
	case c.TownSquare == "":
		return fmt.Errorf("town square voice channel name is empty")
	case c.MovementDeadlineSeconds <= 0:
		return fmt.Errorf("invalid deadline %d (must be >0) for movement operations", c.MovementDeadlineSeconds)
	case c.PerRequestSeconds <= 0:
		return fmt.Errorf("invalid deadline %d (must be >0) for requests", c.PerRequestSeconds)
	case c.MaxConcurrentRequests <= 0:
		return fmt.Errorf("invalid max number of concurrent requests %d (must be >0) ", c.MaxConcurrentRequests)
	}

	return nil
}
