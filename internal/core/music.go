package core

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type Music struct {
	Source string `bson:"source" json:"source"`
	Value  string `bson:"value" json:"value"`
	Title  string `bson:"title" json:"title"`
}

var spotifyMusic = regexp.MustCompile(`^/(track|album|playlist)/[A-Za-z0-9]{22}$`)
var youtubeMusic = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func ValidMusic(m *Music, audioKeys []string) bool {
	if m == nil {
		return true
	}
	if utf8.RuneCountInString(m.Title) > 160 || len(m.Value) > 512 {
		return false
	}
	switch m.Source {
	case "mp3":
		return strings.HasSuffix(m.Value, ".mp3") && slices.Contains(audioKeys, m.Value)
	case "youtube":
		return youtubeMusic.MatchString(m.Value)
	case "spotify":
		u, err := url.Parse(m.Value)
		return err == nil && u.Scheme == "https" && u.Host == "open.spotify.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && spotifyMusic.MatchString(u.Path)
	}
	return false
}

// Check the ID3 size and MPEG frame header, never trust a filename or MIME header.
func ValidMP3(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	offset := 0
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		if data[3] < 2 || data[3] > 4 {
			return false
		}
		for _, b := range data[6:10] {
			if b&0x80 != 0 {
				return false
			}
		}
		offset = 10 + int(data[6])<<21 + int(data[7])<<14 + int(data[8])<<7 + int(data[9])
	}
	if offset > len(data)-4 {
		return false
	}
	h := data[offset : offset+4]
	return h[0] == 0xff && h[1]&0xe0 == 0xe0 && h[1]&0x18 != 0x08 && h[1]&0x06 == 0x02 && h[2]&0xf0 != 0 && h[2]&0xf0 != 0xf0 && h[2]&0x0c != 0x0c
}
