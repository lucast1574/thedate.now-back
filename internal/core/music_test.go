package core

import "testing"

func TestMusicSourcesAndOwnership(t *testing.T) {
	key := "b6ea72ce-c49d-4f94-ae71-0a69d23dbf1e.mp3"
	cases := []struct {
		music *Music
		keys  []string
		valid bool
	}{
		{nil, nil, true},
		{&Music{Source: "mp3", Value: key}, []string{key}, true},
		{&Music{Source: "mp3", Value: key}, nil, false},
		{&Music{Source: "youtube", Value: "dQw4w9WgXcQ"}, nil, true},
		{&Music{Source: "youtube", Value: "https://evil.test/"}, nil, false},
		{&Music{Source: "spotify", Value: "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"}, nil, true},
		{&Music{Source: "spotify", Value: "https://open.spotify.com.evil.test/track/4uLU6hMCjMI75M1A2tKUQC"}, nil, false},
		{&Music{Source: "spotify", Value: "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC?redirect=evil"}, nil, false},
		{&Music{Source: "url", Value: "javascript:alert(1)"}, nil, false},
	}
	for _, tc := range cases {
		if got := ValidMusic(tc.music, tc.keys); got != tc.valid {
			t.Errorf("music %+v: got %v", tc.music, got)
		}
	}
}

func TestMP3HeaderAndID3Bounds(t *testing.T) {
	frame := []byte{0xff, 0xfb, 0x90, 0x00, 0x00}
	tagged := append([]byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0}, frame...)
	for _, data := range [][]byte{frame, tagged} {
		if !ValidMP3(data) {
			t.Fatal("MP3 header rejected")
		}
	}
	for _, data := range [][]byte{nil, []byte("<script>bad</script>"), []byte("ID3"), {'I', 'D', '3', 4, 0, 0, 0x7f, 0x7f, 0x7f, 0x7f}, {0xff, 0xfb, 0xf0, 0}, {0xff, 0xfb, 0x9c, 0}} {
		if ValidMP3(data) {
			t.Fatal("invalid MPEG header accepted")
		}
	}
}
