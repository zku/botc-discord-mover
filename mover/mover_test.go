package mover

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-cmp/cmp"
)

func TestNext(t *testing.T) {
	m := &simpleGuildMemberMover{
		sessions: []*discordgo.Session{
			{Token: "a"}, {Token: "b"}, {Token: "c"},
		},
	}

	var got []string
	for i := 0; i < 7; i++ {
		s := m.next()
		got = append(got, s.Token)
	}

	want := []string{"a", "b", "c", "a", "b", "c", "a"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected order of sessions received from mover (-want, +got):%s\n", diff)
	}
}
