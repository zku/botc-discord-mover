package mover

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	maxAsyncRequests   = 3
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

type sessionProvider interface {
	Next() *discordgo.Session
}

// Execute executes all movements required to enter a new phase.
// Movements are load-balanced across all configured bots provided by the session provider.
// The whole phase transition must not take longer than the configured MovementDeadlineSeconds.
func (p *movementPlan) Execute(ctx context.Context, sp sessionProvider) error {
	tasks := make(chan string, len(p.moves))
	for user := range p.moves {
		tasks <- user
	}

	results := make(chan error)
	for i := 0; i < maxAsyncRequests; i++ {
		go func() {
			for user := range tasks {
				results <- executeSingleMove(ctx, p.guild, user, p.moves[user], sp)
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

func executeSingleMove(ctx context.Context, guild, user, channel string, sp sessionProvider) error {
	for i := 0; i < maxAttemptsPerUser; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			session := sp.Next()
			log.Printf("Using session %s to move %s to %s (attempt %d).", session.State.User.Username, user, channel, i+1)
			if err := session.GuildMemberMove(guild, user, &channel, discordgo.WithContext(ctx)); err != nil {
				log.Printf("Attempt %d to move %s to %s failed: %v", i+1, user, channel, err)
				time.Sleep(50 * time.Millisecond)
			} else {
				return nil
			}
		}
	}

	return fmt.Errorf("could not move user %s after %d attempts", user, maxAttemptsPerUser)
}
