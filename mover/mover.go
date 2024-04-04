// Package mover implements the botc discord movement bot.
package mover

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type Mover struct {
	cfg *Config
}

func New(cfg *Config) *Mover {
	return &Mover{cfg: cfg}
}

func (m *Mover) RunForever(ctx context.Context) error {
	var token string
	dg, err := discordgo.New("Bot " + token)
	fmt.Printf("%v %v\n", dg, err)
	return nil
}
