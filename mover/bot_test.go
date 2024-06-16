package mover

import (
	"context"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type fakeDiscordSession struct {
	id string
}

func (f *fakeDiscordSession) GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
	return nil, nil
}

func (f *fakeDiscordSession) StateGuild(guildID string) (*discordgo.Guild, error) {
	return nil, nil
}

func (f *fakeDiscordSession) GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	return nil, nil
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
	// need helpers to create a fake discord guild & voice channel
	m := New(&Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "nightphase",
		DayPhaseCategory:        "dayphase",
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
		if err := m.checkUserIsStoryTeller(ctx, d, i); (err != nil) != tc.wantErr {
			t.Errorf("checkUserIsStoryTeller() returned unexpected error %v, want error: %t", err, tc.wantErr)
		}
	}
}

func TestPrepareDayMoves(t *testing.T) {
	t.Skip("TODO: Implement rest of discord session fake.")
}

func TestPrepareNightMoves(t *testing.T) {
	t.Skip("TODO: Implement rest of discord session fake.")
}

func TestExecuteMovementPlan(t *testing.T) {
	t.Skip("TODO: Implement rest of discord session fake.")
}
