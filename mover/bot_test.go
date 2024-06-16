package mover

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-cmp/cmp"
)

type fakeDiscordSession struct {
	id               string
	userToChannelMap map[string]string
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

type fakeMover struct {
	*fakeDiscordSession
	mu sync.Mutex
}

func (f *fakeMover) Move(ctx context.Context, guild, user, channel string) error {
	if f.id != guild {
		return fmt.Errorf("unknown guild: %v", guild)
	}

	// Not very performant, but this doesn't really matter for test code.
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.userToChannelMap[user]; !ok {
		return fmt.Errorf("unknown user: %v", user)
	}

	f.userToChannelMap[user] = channel
	return nil
}

func TestExecuteMovementPlan(t *testing.T) {
	m := New(&Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "nightphase",
		DayPhaseCategory:        "dayphase",
		TownSquare:              "townsquare",
		StoryTellerRole:         "storyteller",
		MovementDeadlineSeconds: 15,
		PerRequestSeconds:       5,
		MaxConcurrentRequests:   3,
	})

	d := &fakeDiscordSession{
		id: "guild",
		userToChannelMap: map[string]string{
			"user1": "townsquare",
			"user2": "townsquare",
		},
	}

	plan := &movementPlan{
		guild: d.id,
		moves: map[string]string{},
	}

	want := map[string]string{
		"user1": "townsquare",
		"user2": "townsquare",
	}
	for i := 3; i < 10_000; i++ {
		name := fmt.Sprintf("user%d", i)
		d.userToChannelMap[name] = "somewhere"
		plan.moves[name] = "townsquare"
		want[name] = "townsquare"
	}

	ctx := context.Background()
	if err := plan.Execute(ctx, m.cfg, &fakeMover{fakeDiscordSession: d}); err != nil {
		t.Fatalf("Cannot execute plan: %v", err)
	}

	got := d.userToChannelMap
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("End state is not as expected (-want, +got):\n%s", diff)
	}
}

func TestPrepareDayMoves(t *testing.T) {
	t.Skip("TODO: Implement rest of discord session fake.")
}

func TestPrepareNightMoves(t *testing.T) {
	t.Skip("TODO: Implement rest of discord session fake.")
}
