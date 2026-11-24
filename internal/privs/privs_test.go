package privs

import (
	"os"
	"testing"
)

func TestIdentity(t *testing.T) {
	for _, c := range []struct {
		u, g   string
		wu, wg int
		ok     bool
	}{{"", "", 10001, 10001, true}, {"1000", "", 1000, 10001, true}, {"1000", "1001", 1000, 1001, true},
		{"0", "", 0, 0, false}, {"-1", "", 0, 0, false}, {"x", "", 0, 0, false}, {"1000", "0", 0, 0, false}, {"4294967295", "", 0, 0, false}} {
		u, g, err := Identity(func(k string) string {
			if k == "PUID" {
				return c.u
			}
			return c.g
		})
		if (err == nil) != c.ok || c.ok && (u != c.wu || g != c.wg) {
			t.Errorf("%+v: %d %d %v", c, u, g, err)
		}
	}
}

func TestOrdinaryIdentityIsLeftAlone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root")
	}
	if err := Drop(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
