// Package mover implements the botc discord movement bot.
package mover

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

type Mover struct {
	cfg      *Config
	sessions []*discordgo.Session
}

func New(cfg *Config) *Mover {
	return &Mover{cfg: cfg}
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
							Label:    "Day Time: Move to Town Square",
							CustomID: buttonDay,
							Style:    discordgo.PrimaryButton,
						},
						discordgo.Button{
							Emoji:    &discordgo.ComponentEmoji{Name: "🌑"},
							Label:    "Night Time: Move to Cottages",
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

func (m *Mover) prepareNightMoves(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	log.Println("Moving to night.")

	// List all night voice channels (cottages) -- can we verify they are private?
	channels, err := s.GuildChannels(i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot list guild channels: %w", err)
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
		return fmt.Errorf("cannot find day category %q", m.cfg.DayPhaseCategory)
	}
	if nightCategoryChannel == nil {
		return fmt.Errorf("cannot find night category %q", m.cfg.NightPhaseCategory)
	}
	if townSquareChannel == nil {
		return fmt.Errorf("cannot find Town Square %q", m.cfg.TownSquare)
	}

	var nightCottages []*discordgo.Channel
	nightCottageChannelIDs := make(map[string]bool)
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice && channel.ParentID == nightCategoryChannel.ID {
			nightCottages = append(nightCottages, channel)
			nightCottageChannelIDs[channel.ID] = true
		}
	}

	log.Printf("Found all relevant channels and %d cottages.", len(nightCottages))

	// List all users in all voice channels.
	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot fetch guild state: %w", err)
	}
	userToVoiceState := make(map[string]*discordgo.VoiceState)
	for _, vs := range guild.VoiceStates {
		userToVoiceState[vs.UserID] = vs
	}
	members, err := s.GuildMembers(i.GuildID, "", 1000)
	if err != nil {
		return fmt.Errorf("cannot list guild members: %w", err)
	}

	// Anyone who isn't already in a private night time cottage needs to move.
	fullCottageIDs := make(map[string]bool)
	userToVoiceChannelMap := make(map[string]string)
	var userNeedsMove []string
	for _, member := range members {
		state := userToVoiceState[member.User.ID]
		if state != nil && state.ChannelID != "" {
			// This member is in a voice channel.
			userToVoiceChannelMap[member.User.ID] = state.ChannelID
			// If they are in a cottage, we have to mark it as full already. (Should rarely happen?)
			if nightCottageChannelIDs[state.ChannelID] {
				fullCottageIDs[state.ChannelID] = true
			} else {
				userNeedsMove = append(userNeedsMove, member.User.ID)
			}
		}
	}

	log.Printf("User to voice channel map: %+v", userToVoiceChannelMap)
	log.Printf("full cottage IDs: %+v", fullCottageIDs)
	log.Printf("Users need move to cottage: %+v", userNeedsMove)

	// Build the movement plan.
	movementPlan := make(map[string]string)
	for _, user := range userNeedsMove {
		for _, cottage := range nightCottages {
			if fullCottageIDs[cottage.ID] {
				continue // this cottage is already full
			}
			movementPlan[user] = cottage.ID
			// Mark as full.
			fullCottageIDs[cottage.ID] = true
		}
	}

	log.Printf("Movement plan: %+v", movementPlan)

	// ensure number of cottages > number of users to move
	// create a movement plan, user to cottage (1 per)
	// execute the plan

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	return nil
}

func (m *Mover) prepareDayMoves(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	log.Println("Moving to day.")
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
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

	return fmt.Errorf("user %v is not a story teller", i.Member.DisplayName())
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

	// Listen for commands.
	m.sessions[0].AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if err := m.checkUserIsStoryTeller(s, i); err != nil {
			log.Printf("Invalid user: %v", err)
			return
		}

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
