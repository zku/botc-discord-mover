package mover

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

const (
	// TODO: Make configurable?
	maxAttemptsPerUser = 2
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

type guildMemberMover interface {
	Move(ctx context.Context, guild, user, channel string) error
}

// Execute executes all movements required to enter a new phase.
func (p *movementPlan) Execute(ctx context.Context, cfg *Config, m guildMemberMover) error {
	tasks := make(chan string, len(p.moves))
	for user := range p.moves {
		tasks <- user
	}

	results := make(chan error)
	for i := 0; i < cfg.MaxConcurrentRequests; i++ {
		go func() {
			for user := range tasks {
				results <- executeSingleMove(ctx, p.guild, user, p.moves[user], len(p.moves), m)
			}
		}()
	}

	var err error

	for range p.moves {
		if e := <-results; e != nil {
			err = e
		}
	}

	close(results)
	close(tasks)
	return err
}

func executeSingleMove(ctx context.Context, guild, user, channel string, planSize int, m guildMemberMover) error {
	for i := 0; i < maxAttemptsPerUser; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if i == 0 {
				wait := (1000.0 / float64(planSize)) * rand.Float64()
				time.Sleep(time.Duration(wait) * time.Millisecond)
			}
			if err := m.Move(ctx, guild, user, channel); err != nil {
				log.Printf("Attempt %d to move %s to %s failed: %v", i+1, user, channel, err)
				time.Sleep(50 * time.Millisecond)
			} else {
				return nil
			}
		}
	}

	return fmt.Errorf("could not move user %s after %d attempts", user, maxAttemptsPerUser)
}
