package mover

import "fmt"

type Config struct {
	Tokens             []string
	NightPhaseCategory string
	DayPhaseCategory   string
	TownSquare         string
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
	}

	return nil
}
