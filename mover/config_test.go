package mover

import "testing"

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
