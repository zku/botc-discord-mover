package mover

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
	// The real mover is much slower since it issues HTTPS requests, so this is fine.
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
	cfg := &Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "night phase",
		DayPhaseCategory:        "day phase",
		TownSquare:              "townsquare",
		StoryTellerRole:         "storyteller",
		MovementDeadlineSeconds: 15,
		PerRequestSeconds:       5,
		MaxConcurrentRequests:   3,
	}

	d := &fakeDiscordSession{
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
	if err := plan.Execute(ctx, cfg, fm); err != nil {
		t.Fatalf("Cannot execute plan: %v", err)
	}

	got := d.userToChannelMap
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("End state is not as expected (-want, +got):\n%s", diff)
	}
}
