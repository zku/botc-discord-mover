package mover

import "fmt"

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
  "MovementDeadlineSeconds": 15
}
*/
type Config struct {
	Tokens                  []string
	NightPhaseCategory      string
	DayPhaseCategory        string
	TownSquare              string
	StoryTellerRole         string
	MovementDeadlineSeconds int
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
	}

	return nil
}
