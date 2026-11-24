package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOnlinePlays(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	s.SyncDevices(ctx, []DiscoveredDevice{{DeviceID: "d1", Name: "x"}}, now)
	for i, x := range [][2]string{
		{`{"decision":"online","title":"晴天","artist":"周杰伦"}`, OutcomeConfirmed},
		{`{"decision":"local","title":"b"}`, OutcomeConfirmed},
		{`{"decision":"online","title":"c"}`, OutcomeFailed},
		{`broken`, OutcomeFetched},
	} {
		c := Command{DeviceID: "d1", RequestKey: string(rune('a' + i)), Query: "q", IntentKind: "song"}
		if _, err := s.AddCommand(ctx, &c); err != nil {
			t.Fatal(err)
		}
		s.SetCommandRoute(ctx, c.ID, []byte(x[0]))
		r := TakeoverRun{DeviceID: "d1", CommandID: c.ID, Kind: "command", Source: "x", StartedMS: 5}
		s.StartRun(ctx, &r)
		r.Outcome = x[1]
		if err := s.SaveRun(ctx, &r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.OnlinePlays(ctx)
	if err != nil || len(got) != 1 || got[0] != (OnlinePlay{"晴天", "周杰伦", 5}) {
		t.Fatal(got, err)
	}
}
