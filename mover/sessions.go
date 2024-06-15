package mover

import (
	"sync"

	"github.com/bwmarrin/discordgo"
)

type simpleSessionProvider struct {
	sessions []*discordgo.Session
	counter  int
	mu       sync.Mutex
}

func (s *simpleSessionProvider) Next() *discordgo.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.counter % len(s.sessions)
	s.counter += 1
	return s.sessions[idx]
}
