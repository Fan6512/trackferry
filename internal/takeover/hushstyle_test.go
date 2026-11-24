package takeover

import "testing"

func TestHushStyle(t *testing.T) {
	for _, c := range []struct{ stored, hw, did, want string }{
		{"", "OH2P", "1", HushStrong}, {"normal", "OH2P", "1", HushNormal}, {"wait", "OH2P", "1", HushWait},
		{"", "LX06", "1", HushNormal}, {"strong", "LX06", "1", HushStrong},
		{"", "OH2P", "", HushNormal}, {"strong", "ZZZ", "1", HushNormal},
	} {
		if got := HushStyle(c.stored, c.hw, c.did); got != c.want {
			t.Errorf("%+v: %q", c, got)
		}
	}
}
