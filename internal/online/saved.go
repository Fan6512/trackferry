package online

import "time"

// Saved is a found song in a form that can be stored.
type Saved struct {
	Source     string `json:"source"`
	ID         string `json:"id"`
	Ref        string `json:"ref,omitempty"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	DurationMS int64  `json:"duration_ms"` // of the audio
	ListedMS   int64  `json:"listed_ms"`   // as the platform lists it
}

// Save returns what identifies f. Its address is left out: addresses expire.
func Save(f *Found) Saved {
	c := f.Candidate
	return Saved{Source: c.Source, ID: c.ID, Ref: c.ref, Title: c.Title, Artist: c.Artist,
		DurationMS: f.Stream.Duration.Milliseconds(), ListedMS: c.Duration.Milliseconds()}
}

// Found is the saved song; its Opener asks the platform for an address first.
func (s Saved) Found() *Found {
	return &Found{Candidate: Candidate{Source: s.Source, ID: s.ID, ref: s.Ref, Title: s.Title, Artist: s.Artist,
		Duration: time.Duration(s.ListedMS) * time.Millisecond, DurationMS: s.ListedMS},
		Stream: Stream{Duration: time.Duration(s.DurationMS) * time.Millisecond, Type: "audio/mpeg"}}
}

// Enabled reports whether a platform is switched on.
func (s *Service) Enabled(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.srcs {
		if e.src.Name() == name {
			return e.enabled
		}
	}
	return false
}
