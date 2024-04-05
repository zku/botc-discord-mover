package mover

import (
	"fmt"
	"strings"
)

// movementPlan contains the required moves for this guild to enter the desired phase.
type movementPlan struct {
	// moves maps user IDs to channel IDs.
	moves map[string]string
	guild string
}

func (p *movementPlan) String() string {
	if len(p.moves) == 0 {
		return fmt.Sprintf("No movements required for guild %s", p.guild)
	}

	var parts []string
	for user, channel := range p.moves {
		parts = append(parts, fmt.Sprintf("[Move user %s to channel %s]", user, channel))
	}

	return fmt.Sprintf("Moving members of guild %s: %s", p.guild, strings.Join(parts, ", "))
}
