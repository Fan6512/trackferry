package audio

import "testing"

func TestMP3Duration(t *testing.T) {
	frame := make([]byte, 417)
	copy(frame, []byte{0xff, 0xfb, 0x90, 0x00})
	b := append(append([]byte{}, frame...), frame...)
	if d := MP3Duration(b, 160_000); d.Seconds() != 10 {
		t.Fatal(d)
	}
	if MP3Duration([]byte("<html>not audio</html>"), 1000) != 0 {
		t.Fatal("html accepted")
	}
	if n := ID3Size([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 1, 1}); n != 10+129 {
		t.Fatal(n)
	}
}
