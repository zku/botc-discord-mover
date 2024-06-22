// Package mover implements the botc discord movement bot.
package mover

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/exp/slices"
)

// Bot is a BotC multi-bot voice channel mover.
type Bot struct {
	cfg      *Config
	sessions []*discordgo.Session
	ch       chan (*movementPlan)
}

// New creates a new BotC multi-bot voice channel mover.
// Supports moving users to individual cottages (night phases) and to Town Square (day phase).
// Actions are load-balanced across all configured bots in an attempt to reduce Discord
// throttling issues for large games (>10 players).
func New(cfg *Config) *Bot {
	return &Bot{cfg: cfg, ch: make(chan (*movementPlan))}
}

// Button IDs.
const (
	buttonNight = "buttonNight"
	buttonDay   = "buttonDay"
)

// onButtonPressed handles the 2 button presses for day/night phase movements.
func (b *Bot) onButtonPressed(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	switch i.MessageComponentData().CustomID {
	case buttonNight:
		return b.prepareNightMoves(ctx, &discordSessionWrap{s}, i)
	case buttonDay:
		return b.prepareDayMoves(ctx, &discordSessionWrap{s}, i)
	}

	return fmt.Errorf("unknown button pressed: %#v", i.MessageComponentData())
}

// Slash command IDs.
const (
	slashCommandButtons = "buttons"
)

// onSlashCommand handles the /buttons slash command and responds with the 2 button embeds.
func (b *Bot) onSlashCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
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
	}, discordgo.WithContext(ctx))

	return nil
}

// discordVoiceState contains all required discord guild and voice state information to perform
// day or night moves.
type discordVoiceState struct {
	guild            *discordgo.Guild
	userToVoiceState map[string]*discordgo.VoiceState
	members          []*discordgo.Member
	townSquare       *discordgo.Channel
	cottages         []*discordgo.Channel
}

// discordSessionWrap wraps a discordgo session to simplify unit testing.
type discordSessionWrap struct {
	*discordgo.Session
}

// StateGuild implements the session's State.Guild(). session.Guild() and session.State.Guild()
// have different semantics, and we unfortunately need to use the State's Guild() call for the
// voice channel information.
func (s *discordSessionWrap) StateGuild(guildID string) (*discordgo.Guild, error) {
	return s.State.Guild(guildID)
}

// discordSession interface used by the bot. Can be exchanged for a fake in unit tests.
type discordSession interface {
	GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error)
	StateGuild(guildID string) (*discordgo.Guild, error)
	GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error)
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error
	GuildRoles(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Role, error)
}

// buildDiscordVoiceState returns information about all mandatory voice channels and members in
// voice channels.
func (b *Bot) buildDiscordVoiceState(ctx context.Context, s discordSession, guildID string) (*discordVoiceState, error) {
	channels, err := s.GuildChannels(guildID, discordgo.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("cannot list guild channels: %w", err)
	}

	var dayCategoryChannel, nightCategoryChannel, townSquareChannel *discordgo.Channel
	for _, channel := range channels {
		switch channel.Name {
		case b.cfg.DayPhaseCategory:
			dayCategoryChannel = channel
		case b.cfg.NightPhaseCategory:
			nightCategoryChannel = channel
		case b.cfg.TownSquare:
			townSquareChannel = channel
		}
	}

	if dayCategoryChannel == nil {
		return nil, fmt.Errorf("cannot find day category %q", b.cfg.DayPhaseCategory)
	}
	if nightCategoryChannel == nil {
		return nil, fmt.Errorf("cannot find night category %q", b.cfg.NightPhaseCategory)
	}
	if townSquareChannel == nil {
		return nil, fmt.Errorf("cannot find Town Square %q", b.cfg.TownSquare)
	}
	if townSquareChannel.ParentID != dayCategoryChannel.ID {
		return nil, fmt.Errorf("town square is not under day phase")
	}

	var cottages []*discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice && channel.ParentID == nightCategoryChannel.ID {
			cottages = append(cottages, channel)
		}
	}

	// Sort the cottages according to their position and use the top N cottages.
	slices.SortFunc(cottages, func(a, b *discordgo.Channel) int {
		return a.Position - b.Position
	})

	guild, err := s.StateGuild(guildID)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch guild state: %w", err)
	}
	userToVoiceState := make(map[string]*discordgo.VoiceState)
	for _, vs := range guild.VoiceStates {
		userToVoiceState[vs.UserID] = vs
	}
	members, err := s.GuildMembers(guildID, "", 1000, discordgo.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("cannot list guild members: %w", err)
	}

	return &discordVoiceState{
		guild:            guild,
		userToVoiceState: userToVoiceState,
		members:          members,
		townSquare:       townSquareChannel,
		cottages:         cottages,
	}, nil
}

// forwardInteractionError forwards the interaction error to the button embed.
func forwardInteractionError(s *discordgo.Session, i *discordgo.InteractionCreate, err error) {
	log.Printf("Interaction error: %v", err)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: fmt.Sprintf("Interaction error: %v", err),
		},
	})
}

// prepareNightMoves prepares all necessary moves for the night phase and dispatches the plan.
func (b *Bot) prepareNightMoves(ctx context.Context, s discordSession, i *discordgo.InteractionCreate) error {
	log.Println("Moving to night.")

	vs, err := b.buildDiscordVoiceState(ctx, s, i.GuildID)
	if err != nil {
		return fmt.Errorf("cannot build voice state: %w", err)
	}

	log.Printf("Found all relevant channels and %d cottages for the night phase.", len(vs.cottages))

	nightCottageChannelIDs := make(map[string]bool)
	for _, cottage := range vs.cottages {
		nightCottageChannelIDs[cottage.ID] = true
	}

	// Anyone who isn't already in a private night time cottage needs to move.
	var storyTellerCottageID string
	fullCottageIDs := make(map[string]bool)
	var userNeedsMove []*discordgo.Member
	for _, member := range vs.members {
		userVoiceState := vs.userToVoiceState[member.User.ID]
		if userVoiceState != nil && userVoiceState.ChannelID != "" {
			// This member is in a voice channel.
			// If they are in a cottage, we have to mark it as full already.
			// This only happens if a new player joins during the night phase.
			if nightCottageChannelIDs[userVoiceState.ChannelID] {
				fullCottageIDs[userVoiceState.ChannelID] = true
				if storyTellerCottageID == "" && member.User.ID == i.Member.User.ID {
					storyTellerCottageID = userVoiceState.ChannelID
				}
			} else {
				// Otherwise, they need to move.
				userNeedsMove = append(userNeedsMove, member)
			}
		}
	}

	if len(userNeedsMove) > len(nightCottageChannelIDs)-len(fullCottageIDs) {
		return fmt.Errorf("not enough cottages available, need %d user movements but only have %d empty cottages", len(userNeedsMove), len(nightCottageChannelIDs)-len(fullCottageIDs))
	}

	// Find the story teller role ID.
	allRoles, err := s.GuildRoles(i.GuildID, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("cannot fetch guild roles: %w", err)
	}
	var storyTellerRoleID string
	for _, role := range allRoles {
		if role.Name == b.cfg.StoryTellerRole {
			storyTellerRoleID = role.ID
			break
		}
	}
	if storyTellerRoleID == "" {
		return fmt.Errorf("cannot determine story teller role ID for name %s", b.cfg.StoryTellerRole)
	}

	// Build the movement plan.
	plan := make(map[string]string)
	for _, member := range userNeedsMove {
		isStoryTeller := slices.Contains(member.Roles, storyTellerRoleID)
		if isStoryTeller && storyTellerCottageID != "" {
			// Move story tellers into the same cottage at night.
			plan[member.User.ID] = storyTellerCottageID
			continue
		}
		for _, cottage := range vs.cottages {
			if fullCottageIDs[cottage.ID] {
				continue // This cottage is already full.
			}
			plan[member.User.ID] = cottage.ID
			fullCottageIDs[cottage.ID] = true
			if isStoryTeller {
				storyTellerCottageID = cottage.ID
			}
			break
		}
	}

	if len(plan) != len(userNeedsMove) {
		return fmt.Errorf("could not find a move for every player, plan %d vs needed moves %d", len(plan), len(userNeedsMove))
	}

	select {
	case b.ch <- &movementPlan{moves: plan, guild: i.GuildID}:
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}, discordgo.WithContext(ctx))
	default:
		// NOTE: If we ever want to provide this bot as a service (vs self-hosted), we should allow
		// concurrent movement plans (for different guild IDs).
		return fmt.Errorf("existing player movement has not finished yet, please wait")
	}
}

// prepareDayMoves prepares all necessary moves for the day phase and dispatches the plan.
func (b *Bot) prepareDayMoves(ctx context.Context, s discordSession, i *discordgo.InteractionCreate) error {
	log.Println("Moving to day.")

	vs, err := b.buildDiscordVoiceState(ctx, s, i.GuildID)
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
	case b.ch <- &movementPlan{moves: plan, guild: i.GuildID}:
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		}, discordgo.WithContext(ctx))
	default:
		return fmt.Errorf("existing player movement has not finished yet, please wait")
	}
}

// checkUserIsStoryTeller returns an error iff the interaction user is not a story teller or if the
// command was not invoked in a guild channel.
func (b *Bot) checkUserIsStoryTeller(ctx context.Context, s discordSession, guildID string, member *discordgo.Member) error {
	if member == nil {
		return fmt.Errorf("action not invoked from guild channel")
	}

	// Fetch all guild roles.
	allRoles, err := s.GuildRoles(guildID, discordgo.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("cannot fetch guild roles: %w", err)
	}

	var storyTellerRoleID string
	for _, role := range allRoles {
		if role.Name == b.cfg.StoryTellerRole {
			storyTellerRoleID = role.ID
			break
		}
	}

	if storyTellerRoleID == "" {
		return fmt.Errorf("cannot find story teller role %s among %#v", b.cfg.StoryTellerRole, allRoles)
	}

	if slices.Contains(member.Roles, storyTellerRoleID) {
		// User is a story teller.
		return nil
	}

	return fmt.Errorf("user %v (%v) is not a story teller", member.User.Username, member.DisplayName())
}

// handleMovementPlans listens for and handles new movement plans. Only one plan can be executed
// at once.
func (b *Bot) handleMovementPlans() {
	m := &simpleGuildMemberMover{sessions: b.sessions}
	for plan := range b.ch {
		log.Printf("Received new movement plan: %v", plan)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(b.cfg.MovementDeadlineSeconds))
		if err := plan.Execute(ctx, b.cfg, m); err != nil {
			log.Printf("Executing movement plan failed: %v", err)
		} else {
			log.Printf("Successfully finished movement plan.")
		}
		cancel()
	}
}

// RunForever establishes all bot sessions and listens for commands until the program is
// terminated.
func (b *Bot) RunForever() error {
	// Establish all bot sessions.
	for _, token := range b.cfg.Tokens {
		dg, err := discordgo.New("Bot " + token)
		if err != nil {
			return fmt.Errorf("cannot create discordgo session: %w", err)
		}
		defer dg.Close()

		dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentGuilds | discordgo.IntentGuildMessages | discordgo.IntentMessageContent | discordgo.IntentGuildMembers | discordgo.IntentsGuildVoiceStates)
		if err := dg.Open(); err != nil {
			return fmt.Errorf("cannot open session: %w", err)
		}

		b.sessions = append(b.sessions, dg)
	}

	if l := len(b.sessions); l == 0 {
		return fmt.Errorf("no discord sessions loaded")
	} else {
		log.Printf("Loaded %d discord session(s).", l)
	}

	// Only session 1 will listen to commands from users. Other sessions
	// only act according to session 1.

	// Create the /buttons slash command.
	if _, err := b.sessions[0].ApplicationCommandCreate(b.sessions[0].State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "buttons",
		Description: "Show day/night action buttons.",
	}); err != nil {
		return fmt.Errorf("cannot create application command: %w", err)
	}

	defer close(b.ch)
	go b.handleMovementPlans()

	// Listen for commands.
	b.sessions[0].AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(b.cfg.PerRequestSeconds)*time.Second)
		defer cancel()

		if err := b.checkUserIsStoryTeller(ctx, &discordSessionWrap{s}, i.GuildID, i.Member); err != nil {
			log.Printf("Invalid user: %v", err)
			return
		}

		log.Printf("Received command from %s (%s) for guild %s.", i.Member.User.Username, i.Member.DisplayName(), i.GuildID)

		switch i.Type {
		case discordgo.InteractionMessageComponent:
			// Handle button press.
			if err := b.onButtonPressed(ctx, s, i); err != nil {
				forwardInteractionError(s, i, err)
				return
			}
		case discordgo.InteractionApplicationCommand:
			// Handle slash command.
			if err := b.onSlashCommand(ctx, s, i); err != nil {
				forwardInteractionError(s, i, err)
				return
			}
		}
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	return nil
}
