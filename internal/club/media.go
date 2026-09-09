package club

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

type Item struct {
	URL    string `json:"media_url"`
	Type   string `json:"media_type"`
	Source string `json:"video_source,omitempty"`
}
type Media struct {
	Items []Item `json:"items"`
}

func ParseMedia(raw []byte) Media {
	m := Media{Items: []Item{}}
	_ = json.Unmarshal(raw, &m)
	if m.Items == nil {
		m.Items = []Item{}
	}
	return m
}
func HasURL(items []Item, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.TrimSpace(item.URL) == value {
			return true
		}
	}
	return false
}
func Duplicates(items []Item) bool {
	seen := map[string]bool{}
	for _, item := range items {
		value := strings.TrimSpace(item.URL)
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

var videoPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{6,20}$`)

func YouTubeID(value string) string {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return ""
	}
	id := ""
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		parts := strings.FieldsFunc(u.EscapedPath(), func(r rune) bool { return r == '/' })
		if len(parts) > 0 {
			id = parts[0]
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if strings.HasPrefix(u.Path, "/embed/") {
			parts := strings.Split(u.EscapedPath(), "/")
			if len(parts) > 2 {
				id = parts[2]
			}
		}
	}
	if !videoPattern.MatchString(id) {
		return ""
	}
	return id
}
