package course

import (
	"kaderisasi/admin/internal/domain"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const MaxPDFBytes = 20 << 20

type Input struct {
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	MinimumLevel int32  `json:"minimum_level"`
	Status       string `json:"status"`
}
type LessonInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	YoutubeURL  string `json:"youtube_url"`
}
type OrderInput struct {
	LessonIDs []int32 `json:"lesson_ids"`
}

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func YouTubeID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || videoIDPattern.MatchString(value) {
		return value, nil
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" {
		return "", domain.Fail(422, "INVALID_YOUTUBE_URL")
	}
	host := strings.ToLower(u.Hostname())
	id := ""
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if host == "youtu.be" && len(parts) == 1 {
		id = parts[0]
	}
	if slices.Contains([]string{"youtube.com", "www.youtube.com", "m.youtube.com"}, host) {
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		}
		if len(parts) == 2 && slices.Contains([]string{"embed", "shorts", "live"}, parts[0]) {
			id = parts[1]
		}
	}
	if !videoIDPattern.MatchString(id) {
		return "", domain.Fail(422, "INVALID_YOUTUBE_URL")
	}
	return id, nil
}

func (in *Input) Validate(create bool) error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 255 || utf8.RuneCountInString(in.Summary) > 2000 || len(in.Description) > 100000 {
		return domain.Fail(422, "INVALID_COURSE_CONTENT")
	}
	if !slices.Contains([]int32{0, 3, 6, 10}, in.MinimumLevel) {
		return domain.Fail(422, "INVALID_MINIMUM_LEVEL")
	}
	if create && in.Status == "" {
		in.Status = "draft"
	}
	if !slices.Contains([]string{"draft", "published", "archived"}, in.Status) || (create && in.Status != "draft") {
		return domain.Fail(422, "INVALID_COURSE_STATUS")
	}
	return nil
}
func (in *LessonInput) Validate() (string, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 255 || len(in.Description) > 100000 {
		return "", domain.Fail(422, "INVALID_LESSON_CONTENT")
	}
	return YouTubeID(in.YoutubeURL)
}
