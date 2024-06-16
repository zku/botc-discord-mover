package mover

import (
	"context"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type simpleGuildMemberMover struct {
	sessions []*discordgo.Session
	counter  int
	mu       sync.Mutex
}

func (m *simpleGuildMemberMover) next() *discordgo.Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.counter % len(m.sessions)
	m.counter += 1
	return m.sessions[idx]
}

func (m *simpleGuildMemberMover) Move(ctx context.Context, guild, user, channel string) error {
	s := m.next()
	log.Printf("Using session %s to move %s to %s.", s.State.User.Username, user, channel)
	return s.GuildMemberMove(guild, user, &channel, discordgo.WithContext(ctx))
}
