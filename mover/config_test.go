package mover

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		cfg     *Config
		wantErr bool
	}{
		{
			desc: "valid config",
			cfg: &Config{
				Tokens:                  []string{"a", "b", "c"},
				NightPhaseCategory:      "nightphase",
				DayPhaseCategory:        "dayphase",
				TownSquare:              "townsquare",
				StoryTellerRole:         "storyteller",
				MovementDeadlineSeconds: 15,
				PerRequestSeconds:       5,
				MaxConcurrentRequests:   1,
			},
			wantErr: false,
		},
		{
			desc: "missing tokens",
			cfg: &Config{
				NightPhaseCategory:      "nightphase",
				DayPhaseCategory:        "dayphase",
				TownSquare:              "townsquare",
				StoryTellerRole:         "storyteller",
				MovementDeadlineSeconds: 15,
				PerRequestSeconds:       5,
				MaxConcurrentRequests:   1,
			},
			wantErr: true,
		},
		{
			desc: "invalid movement deadline",
			cfg: &Config{
				Tokens:                  []string{"a", "b", "c"},
				NightPhaseCategory:      "nightphase",
				DayPhaseCategory:        "dayphase",
				TownSquare:              "townsquare",
				StoryTellerRole:         "storyteller",
				MovementDeadlineSeconds: 0,
				PerRequestSeconds:       5,
				MaxConcurrentRequests:   1,
			},
			wantErr: true,
		},
	} {
		if err := tc.cfg.Validate(); (err != nil) != tc.wantErr {
			t.Errorf("%s: Validate() returned unexpected error %v, want error: %t", tc.desc, err, tc.wantErr)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("BOTC_TOKENS", "a,b,c")
	t.Setenv("BOTC_NIGHT_PHASE_CATEGORY", "nightphase")
	t.Setenv("BOTC_DAY_PHASE_CATEGORY", "dayphase")
	t.Setenv("BOTC_TOWN_SQUARE", "townsquare")
	t.Setenv("BOTC_STORY_TELLER_ROLE", "storyteller")
	t.Setenv("BOTC_MOVEMENT_DEADLINE_SECONDS", "15")
	t.Setenv("BOTC_PER_REQUEST_SECONDS", "5")
	t.Setenv("BOTC_MAX_CONCURRENT_REQUESTS", "3")

	got, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("Cannot load config from environment variables: %v", err)
	}

	want := &Config{
		Tokens:                  []string{"a", "b", "c"},
		NightPhaseCategory:      "nightphase",
		DayPhaseCategory:        "dayphase",
		TownSquare:              "townsquare",
		StoryTellerRole:         "storyteller",
		MovementDeadlineSeconds: 15,
		PerRequestSeconds:       5,
		MaxConcurrentRequests:   3,
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Config loaded from environment variables mismatch (-want, +got):%s\n", diff)
	}
}
