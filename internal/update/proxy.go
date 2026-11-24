package update

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// The proxy the owner set for GitHub. Nothing else in TrackFerry uses it.

var errProxy = errors.New("代理地址要写成 http://192.168.1.2:7890 或 socks5://192.168.1.2:7891")

func parseProxy(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, errProxy
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errProxy
	}
	u.Path = ""
	return u, nil
}

// LoadProxy reads the saved proxy from path, where SetProxy will keep it (it may hold a password).
func (c *Checker) LoadProxy(path string) error {
	c.proxyPath = path
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	u, err := parseProxy(string(raw))
	if err != nil {
		return err
	}
	c.useProxy(u)
	return nil
}

// SetProxy saves and uses a proxy; "" goes direct. The text Status shows, with its password
// hidden, stands for the saved one.
func (c *Checker) SetProxy(s string) error {
	u, err := parseProxy(s)
	if err != nil {
		return err
	}
	if old := c.proxy.Load(); old != nil && strings.TrimSpace(s) == old.Redacted() {
		u = old
	}
	if c.proxyPath != "" {
		if u == nil {
			if err := os.Remove(c.proxyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else {
			tmp := c.proxyPath + ".new"
			if err := os.MkdirAll(filepath.Dir(tmp), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(tmp, []byte(u.String()+"\n"), 0o600); err != nil {
				return err
			}
			if err := os.Rename(tmp, c.proxyPath); err != nil {
				return err
			}
		}
	}
	c.useProxy(u)
	return nil
}

func (c *Checker) useProxy(u *url.URL) {
	c.proxy.Store(u)
	c.Client.CloseIdleConnections()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.st.Proxy = ""
	if u != nil {
		c.st.Proxy = u.Redacted()
	}
	c.st.CheckedMS = 0 // the last answer came another way: ask again
}

func (c *Checker) transport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = func(*http.Request) (*url.URL, error) { return c.proxy.Load(), nil }
	return t
}

// unreachable says who could not be reached, without the address.
func (c *Checker) unreachable() error {
	if c.proxy.Load() != nil {
		return errors.New("经代理连不上 GitHub，检查代理地址")
	}
	return errors.New("连不上 GitHub，可以设置代理")
}
