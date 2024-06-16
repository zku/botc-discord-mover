package mover

import (
	"context"
	"fmt"
	"strings"
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
		},
	}, nil
}

func (f *fakeDiscordSession) GuildMembers(guildID string, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	if f.id != guildID {
		return nil, fmt.Errorf("unknown guild: %v", guildID)
	}

	return []*discordgo.Member{
		{User: &discordgo.User{ID: "user1"}},
		{User: &discordgo.User{ID: "user2"}},
		{User: &discordgo.User{ID: "user3"}},
		{User: &discordgo.User{ID: "storyteller"}},
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
		if err := m.checkUserIsStoryTeller(ctx, d, i); (err != nil) != tc.wantErr {
			t.Errorf("checkUserIsStoryTeller() returned unexpected error %v, want error: %t", err, tc.wantErr)
		}
	}
}

type fakeMover struct {
	*fakeDiscordSession
	failures         map[string]int
	numTotalFailures int
	mu               sync.Mutex
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

	// Emulate some move failures (e.g. http request times out).
	if f.numTotalFailures < 10 && f.failures[user] < maxAttemptsPerUser-1 {
		f.failures[user]++
		f.numTotalFailures++
		return fmt.Errorf("(expected test failure)")
	}

	f.userToChannelMap[user] = channel
	return nil
}

func TestExecuteMovementPlan(t *testing.T) {
	m := New(&Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "night phase",
		DayPhaseCategory:        "day phase",
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

	fm := &fakeMover{
		fakeDiscordSession: d,
		failures:           make(map[string]int),
	}

	ctx := context.Background()
	if err := plan.Execute(ctx, m.cfg, fm); err != nil {
		t.Fatalf("Cannot execute plan: %v", err)
	}

	got := d.userToChannelMap
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("End state is not as expected (-want, +got):\n%s", diff)
	}
}

func TestPrepareDayMoves(t *testing.T) {
	m := &Mover{
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

	if err := m.prepareDayMoves(ctx, d, i); err != nil {
		t.Fatalf("Cannot prepare day moves: %v", err)
	}

	want := map[string]string{
		"user2":       "townsquare",
		"user3":       "townsquare",
		"storyteller": "townsquare",
	}

	select {
	case plan := <-m.ch:
		got := plan.moves
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Movement plan mismatch (-want, +got):%s\n", diff)
		}
	default:
		t.Fatal("Expected to receive plan, got nothing.")
	}
}

func TestPrepareNightMoves(t *testing.T) {
	m := &Mover{
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

	if err := m.prepareNightMoves(ctx, d, i); err != nil {
		t.Fatalf("Cannot prepare day moves: %v", err)
	}

	select {
	case plan := <-m.ch:
		if len(plan.moves) != 4 {
			t.Fatalf("Expected 4 movements, got %#v", plan.moves)
		}
		for user, channel := range plan.moves {
			if !strings.HasPrefix(channel, "cottage") {
				t.Fatalf("Expected all players to move to cottages, received move %s -> %s instead", user, channel)
			}
		}
	default:
		t.Fatal("Expected to receive plan, got nothing.")
	}
}
