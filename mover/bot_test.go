package mover

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-cmp/cmp"
)

type fakeDiscordSession struct {
	id               string
	userToChannelMap map[string]string
}

func (f *fakeDiscordSession) GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	if f.id != guildID {
		return nil, fmt.Errorf("unknown guild: %v", guildID)
	}

	return []*discordgo.Channel{
		{Name: "day phase", ID: "day phase", ParentID: "root", Type: discordgo.ChannelTypeGuildCategory},
		{Name: "townsquare", ID: "townsquare", ParentID: "day phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "inn", ID: "inn", ParentID: "day phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "hotel", ID: "hotel", ParentID: "day phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "barber", ID: "barber", ParentID: "day phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "night phase", ID: "night phase", ParentID: "root", Type: discordgo.ChannelTypeGuildCategory},
		{Name: "cottage1", ID: "cottage1", ParentID: "night phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "cottage2", ID: "cottage2", ParentID: "night phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "cottage3", ID: "cottage3", ParentID: "night phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "cottage4", ID: "cottage4", ParentID: "night phase", Type: discordgo.ChannelTypeGuildVoice},
		{Name: "cottage5", ID: "cottage5", ParentID: "night phase", Type: discordgo.ChannelTypeGuildVoice},
	}, nil
}

func (f *fakeDiscordSession) StateGuild(guildID string) (*discordgo.Guild, error) {
	if f.id != guildID {
		return nil, fmt.Errorf("unknown guild: %v", guildID)
	}

	return &discordgo.Guild{
		VoiceStates: []*discordgo.VoiceState{
			{UserID: "user1", ChannelID: "townsquare"},
			{UserID: "user2", ChannelID: "inn"},
			{UserID: "user3", ChannelID: "barber"},
			{UserID: "storyteller", ChannelID: "barber"},
			{UserID: "storyteller2", ChannelID: "library"},
		},
	}, nil
}

func (f *fakeDiscordSession) GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	if f.id != guildID {
		return nil, fmt.Errorf("unknown guild: %v", guildID)
	}

	return []*discordgo.Member{
		{User: &discordgo.User{ID: "user1"}, Roles: []string{"role1"}},
		{User: &discordgo.User{ID: "user2"}, Roles: []string{"role1"}},
		{User: &discordgo.User{ID: "user3"}, Roles: []string{"role1"}},
		{User: &discordgo.User{ID: "storyteller"}, Roles: []string{"role1", "storyteller"}},
		{User: &discordgo.User{ID: "storyteller2"}, Roles: []string{"role1", "storyteller"}},
	}, nil
}

func (f *fakeDiscordSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse, options ...discordgo.RequestOption) error {
	return nil
}

func (f *fakeDiscordSession) GuildRoles(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) {
	if f.id == guildID {
		return []*discordgo.Role{
			{ID: "role1", Name: "role1"},
			{ID: "role2", Name: "role2"},
			{ID: "role3", Name: "role3"},
			{ID: "storyteller", Name: "storyteller"},
		}, nil
	}

	return nil, fmt.Errorf("unknown guild: %v", guildID)
}

func TestUserIsStoryTeller(t *testing.T) {
	m := New(&Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "night phase",
		DayPhaseCategory:        "day phase",
		TownSquare:              "townsquare",
		StoryTellerRole:         "storyteller",
		MovementDeadlineSeconds: 15,
		PerRequestSeconds:       5,
		MaxConcurrentRequests:   1,
	})

	d := &fakeDiscordSession{
		id: "guild",
	}

	for _, tc := range []struct {
		roles   []string
		wantErr bool
	}{
		{
			roles:   []string{"foo", "bar", "storyteller", "baz"},
			wantErr: false,
		},
		{
			roles:   []string{"storyteller"},
			wantErr: false,
		},
		{
			roles:   []string{"nobody"},
			wantErr: true,
		},
	} {
		ctx := context.Background()
		i := &discordgo.InteractionCreate{
			Interaction: &discordgo.Interaction{
				GuildID: "guild",
				Member: &discordgo.Member{
					Roles: tc.roles,
					User: &discordgo.User{
						Username: "user",
					},
				},
			},
		}
		if err := m.checkUserIsStoryTeller(ctx, d, i.GuildID, i.Member); (err != nil) != tc.wantErr {
			t.Errorf("checkUserIsStoryTeller() returned unexpected error %v, want error: %t", err, tc.wantErr)
		}
	}
}

func TestPrepareDayMoves(t *testing.T) {
	b := &Bot{
		ch: make(chan *movementPlan, 1),
		cfg: &Config{
			Tokens:                  []string{"a", "b", "c"},
			NightPhaseCategory:      "night phase",
			DayPhaseCategory:        "day phase",
			TownSquare:              "townsquare",
			StoryTellerRole:         "storyteller",
			MovementDeadlineSeconds: 15,
			PerRequestSeconds:       5,
			MaxConcurrentRequests:   3,
		},
	}

	d := &fakeDiscordSession{
		id: "guild",
	}

	ctx := context.Background()
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			GuildID: "guild",
		},
	}

	if err := b.prepareDayMoves(ctx, d, i); err != nil {
		t.Fatalf("Cannot prepare day moves: %v", err)
	}

	want := map[string]string{
		"user2":        "townsquare",
		"user3":        "townsquare",
		"storyteller":  "townsquare",
		"storyteller2": "townsquare",
	}

	select {
	case plan := <-b.ch:
		got := plan.moves
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Movement plan mismatch (-want, +got):%s\n", diff)
		}
	default:
		t.Fatal("Expected to receive plan, got nothing.")
	}
}

func TestPrepareNightMoves(t *testing.T) {
	b := &Bot{
		ch: make(chan *movementPlan, 1),
		cfg: &Config{
			Tokens:                  []string{"a", "b", "c"},
			NightPhaseCategory:      "night phase",
			DayPhaseCategory:        "day phase",
			TownSquare:              "townsquare",
			StoryTellerRole:         "storyteller",
			MovementDeadlineSeconds: 15,
			PerRequestSeconds:       5,
			MaxConcurrentRequests:   3,
		},
	}

	d := &fakeDiscordSession{
		id: "guild",
	}

	ctx := context.Background()
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			GuildID: "guild",
		},
	}

	if err := b.prepareNightMoves(ctx, d, i); err != nil {
		t.Fatalf("Cannot prepare day moves: %v", err)
	}

	select {
	case plan := <-b.ch:
		t.Logf("Movement plan: %#v", plan.moves)
		if len(plan.moves) != 5 {
			t.Fatalf("Expected 5 movements, got %#v", plan.moves)
		}

		var storyTellerCottageID string
		fullCottageIDs := make(map[string]bool)
		for user, channel := range plan.moves {
			if !strings.HasPrefix(channel, "cottage") {
				t.Fatalf("Expected all players to move to cottages, received move %s -> %s instead", user, channel)
			}
			if strings.HasPrefix(user, "storyteller") {
				if storyTellerCottageID == "" {
					storyTellerCottageID = channel
				} else {
					// Check that all STs are in the same cottage. The only time this would not happen is if
					// multiple STs are already in different cottages during the night when a new night move
					// is initiated.
					if channel != storyTellerCottageID {
						t.Fatalf("Expected story teller %s to be in the story teller cottage %s, but instead is moved to %s", user, storyTellerCottageID, channel)
					}
				}
			} else {
				if fullCottageIDs[channel] {
					t.Fatalf("Attempted to move 2 non-story tellers to the same cottage %s", channel)
				}
				fullCottageIDs[channel] = true
			}
		}

	default:
		t.Fatal("Expected to receive plan, got nothing.")
	}
}
