package club

import "testing"

func TestMediaIdentity(t *testing.T) {
	duplicate := []Item{{URL: "https://www.youtube.com/embed/video-1"}, {URL: " https://www.youtube.com/embed/video-1 "}}
	if !Duplicates(duplicate) || !HasURL(duplicate, "https://www.youtube.com/embed/video-1") {
		t.Fatal("trimmed duplicate")
	}
	distinct := []Item{{URL: "club/image-1.webp"}, {URL: "https://www.youtube.com/embed/video-1"}}
	if Duplicates(distinct) || HasURL(distinct, "club/image-2.webp") {
		t.Fatal("distinct media")
	}
	for _, input := range []string{"https://www.youtube.com/watch?v=abc_DEF-123", "https://youtu.be/abc_DEF-123?t=12", "https://www.youtube.com/embed/abc_DEF-123"} {
		if YouTubeID(input) != "abc_DEF-123" {
			t.Error(input)
		}
	}
	for _, input := range []string{"https://evil.example/youtube.com/watch?v=abc_DEF-123", "javascript:alert(1)", "https://www.youtube.com/watch?v=bad/id", "https://user:secret@youtube.com/watch?v=abc_DEF-123"} {
		if YouTubeID(input) != "" {
			t.Error(input)
		}
	}
}
