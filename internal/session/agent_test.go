package session

import (
	"testing"

	"wind-agent/internal/domain"
)

func TestUserContent(t *testing.T) {
	cases := []struct {
		name string
		in   domain.Inbound
		want string
	}{
		{
			name: "private",
			in:   domain.Inbound{Content: "你好"},
			want: "你好",
		},
		{
			name: "group sender",
			in:   domain.Inbound{Content: "你好", Speaker: "U1"},
			want: "[U1] 说：你好",
		},
		{
			name: "group sender with name and mentions",
			in: domain.Inbound{
				Content:     "你好 @[U2]（小明）",
				Speaker:     "U1",
				SpeakerName: "静静",
				Mentions:    []domain.Mention{{ID: "U2", Name: "小明"}, {ID: "U3"}},
			},
			want: "[U1]（静静） 说：你好 @[U2]（小明）\n[本轮@: U2（小明）, U3]",
		},
	}
	for _, c := range cases {
		if got := userContent(c.in); got != c.want {
			t.Fatalf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
