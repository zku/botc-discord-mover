// Package mover implements the botc discord movement bot.
package mover

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Mover struct {
	cfg      *Config
	sessions []*discordgo.Session
	ch       chan (*movementPlan)
}

func New(cfg *Config) *Mover {
	return &Mover{cfg: cfg, ch: make(chan (*movementPlan))}
}

const (
	buttonNight = "buttonNight"
	buttonDay   = "buttonDay"

	slashCommandButtons = "buttons"
)

func (m *Mover) onButtonPressed(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	switch i.MessageComponentData().CustomID {
	case buttonNight:
		return m.prepareNightMoves(s, i)
	case buttonDay:
		return m.prepareDayMoves(s, i)
	}

	return fmt.Errorf("unknown button pressed: %#v", i.MessageComponentData())
}

func (m *Mover) onSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	data := i.ApplicationCommandData()
	if data.Name != slashCommandButtons {
		return fmt.Errorf("unknown slash command: %s", data.Name)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Emoji:    &discordgo.ComponentEmoji{Name: "☀️"},
							Label:    "Day: Return to Town Square",
							CustomID: buttonDay,
							Style:    discordgo.PrimaryButton,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Emoji:    &discordgo.ComponentEmoji{Name: "🌑"},
							Label:    "Night: Send all to Cottages",
							CustomID: buttonNight,
							Style:    discordgo.DangerButton,
						},
					},
				},
			},
		},
	})

	return nil
}

type discordVoiceState struct {
	guild            *discordgo.Guild
	userToVoiceState map[string]*discordgo.VoiceState
	members          []*discordgo.Member
	townSquare       *discordgo.Channel
	cottages         []*discordgo.Channel
}

type movementPlan struct {
	s     *discordgo.Session
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

func (m *Mover) buildDiscordVoiceState(s *discordgo.Session, guildID string) (*discordVoiceState, error) {
	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("cannot list guild channels: %w", err)
	}

	var dayCategoryChannel, nightCategoryChannel, townSquareChannel *discordgo.Channel
	for _, channel := range channels {
		switch channel.Name {
		case m.cfg.DayPhaseCategory:
			dayCategoryChannel = channel
		case m.cfg.NightPhaseCategory:
			nightCategoryChannel = channel
		case m.cfg.TownSquare:
			townSquareChannel = channel
		}
	}

	if dayCategoryChannel == nil {
		return nil, fmt.Errorf("cannot find day category %q", m.cfg.DayPhaseCategory)
	}
	if nightCategoryChannel == nil {
		return nil, fmt.Errorf("cannot find night category %q", m.cfg.NightPhaseCategory)
	}
	if townSquareChannel == nil {
		return nil, fmt.Errorf("cannot find Town Square %q", m.cfg.TownSquare)
	}
	if townSquareChannel.ParentID != dayCategoryChannel.ID {
		return nil, fmt.Errorf("town square is not under day phase")
	}

	var nightCottages []*discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice && channel.ParentID == nightCategoryChannel.ID {
			nightCottages = append(nightCottages, channel)
		}
	}

	guild, err := s.State.Guild(guildID)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch guild state: %w", err)
	}
	userToVoiceState := make(map[string]*discordgo.VoiceState)
	for _, vs := range guild.VoiceStates {
		userToVoiceState[vs.UserID] = vs
	}
	members, err := s.GuildMembers(guildID, "", 1000)
	if err != nil {
		return nil, fmt.Errorf("cannot list guild members: %w", err)
	}

	// TODO: should we randomize member and voice channel order?

	return &discordVoiceState{
		guild:            guild,
		userToVoiceState: userToVoiceState,
		members:          members,
		townSquare:       townSquareChannel,
		cottages:         nightCottages,
	}, nil
}

func (m *Mover) prepareNightMoves(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	log.Println("Moving to night.")

	vs, err := m.buildDiscordVoiceState(s, i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot build voice state: %w", err)
	}

	log.Printf("Found all relevant channels and %d cottages for the night phase.", len(vs.cottages))

	nightCottageChannelIDs := make(map[string]bool)
	for _, cottage := range vs.cottages {
		nightCottageChannelIDs[cottage.ID] = true
	}

	// Anyone who isn't already in a private night time cottage needs to move.
	fullCottageIDs := make(map[string]bool)
	var userNeedsMove []string
	for _, member := range vs.members {
		userVoiceState := vs.userToVoiceState[member.User.ID]
		if userVoiceState != nil && userVoiceState.ChannelID != "" {
			// This member is in a voice channel.
			// If they are in a cottage, we have to mark it as full already. (Should rarely happen?)
			if nightCottageChannelIDs[userVoiceState.ChannelID] {
				fullCottageIDs[userVoiceState.ChannelID] = true
			} else {
				// Otherwise, they need to move.
				userNeedsMove = append(userNeedsMove, member.User.ID)
			}
		}
	}

	if len(userNeedsMove) > len(nightCottageChannelIDs)-len(fullCottageIDs) {
		return fmt.Errorf("not enough cottages available, need %d user movements but only have %d empty cottages", len(userNeedsMove), len(nightCottageChannelIDs)-len(fullCottageIDs))
	}

	// Build the movement plan.
	plan := make(map[string]string)
	for _, user := range userNeedsMove {
		for _, cottage := range vs.cottages {
			if fullCottageIDs[cottage.ID] {
				continue // This cottage is already full.
			}
			plan[user] = cottage.ID
			fullCottageIDs[cottage.ID] = true
		}
	}

	select {
	case m.ch <- &movementPlan{s: s, moves: plan, guild: i.GuildID}:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
	default:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Another movement is already in progress, please wait.",
			},
		})
		return fmt.Errorf("cannot push movement plan to queue, busy")
	}

	return nil
}

func (m *Mover) prepareDayMoves(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	log.Println("Moving to day.")

	vs, err := m.buildDiscordVoiceState(s, i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot build voice state: %w", err)
	}

	log.Printf("Found all relevant channels for the day phase.")

	// Anyone who isn't already in Town Square needs to move.
	plan := make(map[string]string)
	for _, member := range vs.members {
		userVoiceState := vs.userToVoiceState[member.User.ID]
		if userVoiceState != nil && userVoiceState.ChannelID != "" {
			// This member is in a voice channel.
			// If they are not in Town Square, they need to move.
			if userVoiceState.ChannelID != vs.townSquare.ID {
				plan[member.User.ID] = vs.townSquare.ID
			}
		}
	}

	select {
	case m.ch <- &movementPlan{s: s, moves: plan, guild: i.GuildID}:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
	default:
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: "Another movement is already in progress, please wait.",
			},
		})
		return fmt.Errorf("cannot push movement plan to queue, busy")
	}

	return nil
}

func (m *Mover) checkUserIsStoryTeller(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if i.Member == nil {
		return fmt.Errorf("action not invoked from guild channel")
	}

	// Fetch all guild roles.
	allRoles, err := s.GuildRoles(i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot fetch guild roles: %w", err)
	}
	roleToName := make(map[string]string)
	for _, role := range allRoles {
		roleToName[role.ID] = role.Name
	}

	// User must be a story teller.
	for _, role := range i.Member.Roles {
		if m.cfg.StoryTellerRole == roleToName[role] {
			// Found it.
			return nil
		}
	}

	return fmt.Errorf("user %v (%v) is not a story teller", i.Member.User.Username, i.Member.DisplayName())
}

var moveAttempts int64

func (m *Mover) executeMovementPlan(plan *movementPlan) error {
	finishedMoves := make(map[string]bool)

	for deadline := time.Now().Add(time.Duration(m.cfg.MovementDeadlineSeconds) * time.Second); time.Now().Before(deadline); {
		for user, channel := range plan.moves {
			if finishedMoves[user] {
				continue
			}

			counter := int(atomic.LoadInt64(&moveAttempts))
			session := m.sessions[counter%len(m.sessions)]
			log.Printf("Using session %s (overall movement attempt %d) to move %s to %s.", session.State.User.Username, counter, user, channel)
			if err := session.GuildMemberMove(plan.guild, user, &channel); err != nil {
				return err
			}

			atomic.AddInt64(&moveAttempts, 1)
			finishedMoves[user] = true
			time.Sleep(50 * time.Millisecond)
		}

		if len(finishedMoves) == len(plan.moves) {
			log.Println("Successfully finished executing movement plan.")
			return nil
		}
	}

	return fmt.Errorf("movement plan deadline exceeded, finished %d of %d moves", len(finishedMoves), len(plan.moves))
}

func (m *Mover) handleMovementPlans() {
	for plan := range m.ch {
		log.Printf("Received new movement plan: %v", plan)
		m.executeMovementPlan(plan)
	}
}

func (m *Mover) RunForever(ctx context.Context) error {
	// Establish all bot sessions.
	for _, token := range m.cfg.Tokens {
		dg, err := discordgo.New("Bot " + token)
		if err != nil {
			return fmt.Errorf("cannot create discordgo session: %w", err)
		}
		defer dg.Close()

		dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentGuilds | discordgo.IntentGuildMessages | discordgo.IntentMessageContent | discordgo.IntentGuildMembers | discordgo.IntentsGuildVoiceStates)
		if err := dg.Open(); err != nil {
			return fmt.Errorf("cannot open session: %w", err)
		}

		m.sessions = append(m.sessions, dg)
	}
	log.Printf("Loaded %d discord session(s).", len(m.sessions))

	// Only session 1 will listen to commands from users. Other sessions
	// only act according to session 1.

	// Create the /buttons slash command.
	if _, err := m.sessions[0].ApplicationCommandCreate(m.sessions[0].State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "buttons",
		Description: "Show day/night action buttons.",
	}); err != nil {
		return fmt.Errorf("cannot create application command: %w", err)
	}

	defer close(m.ch)
	go m.handleMovementPlans()

	// Listen for commands.
	m.sessions[0].AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if err := m.checkUserIsStoryTeller(s, i); err != nil {
			log.Printf("Invalid user: %v", err)
			return
		}

		log.Printf("Received command from %s (%s).", i.Member.User.Username, i.Member.DisplayName())

		switch i.Type {
		case discordgo.InteractionMessageComponent:
			// Handle button press.
			if err := m.onButtonPressed(s, i); err != nil {
				log.Printf("Cannot handle button press: %v", err)
				return
			}
		case discordgo.InteractionApplicationCommand:
			// Handle slash command.
			if err := m.onSlashCommand(s, i); err != nil {
				log.Printf("Cannot handle slash command: %v", err)
				return
			}
		}
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	return nil
}
