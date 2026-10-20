package engines

import (
	"testing"

	"ttd/internal/model"
)

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"@user":                          "https://www.tiktok.com/@user",
		"www.tiktok.com/@user/video/123": "https://www.tiktok.com/@user/video/123",
		"https://vm.tiktok.com/abc":      "https://vm.tiktok.com/abc",
		"https://example.com/x":          "https://example.com/x",
	}
	for input, want := range cases {
		if got := NormalizeURL(input); got != want {
			t.Errorf("NormalizeURL(%q) = %q, ожидалось %q", input, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"https://www.tiktok.com/@user/video/7412":   model.KindVideoPost,
		"https://www.tiktok.com/@user/photo/7412":   model.KindPhotoPost,
		"https://www.tiktok.com/@user":              model.KindProfile,
		"https://www.tiktok.com/@user/":             model.KindProfile,
		"https://www.tiktok.com/@user/stories":      model.KindStory,
		"https://www.tiktok.com/@user/stories/1":    model.KindStory,
		"https://www.tiktok.com/collection/123":     model.KindCollection,
		"https://www.tiktok.com/music/original-123": model.KindCollection,
		"https://www.tiktok.com/@user/live":         model.KindLive,
		"https://www.youtube.com/watch?v=abc":       model.KindExternal,
		"https://www.tiktok.com/":                   model.KindUnknown,
	}
	for input, want := range cases {
		if got := Classify(input); got != want {
			t.Errorf("Classify(%q) = %q, ожидалось %q", input, got, want)
		}
	}
}

func TestStoriesURL(t *testing.T) {
	if got := StoriesURL("https://www.tiktok.com/@user"); got != "https://www.tiktok.com/@user/stories" {
		t.Fatalf("StoriesURL = %q", got)
	}
	if got := StoriesURL("https://www.tiktok.com/@user/stories"); got != "https://www.tiktok.com/@user/stories" {
		t.Fatalf("StoriesURL повторно = %q", got)
	}
}

func TestExpandURLsWithStories(t *testing.T) {
	planned := ExpandURLs([]string{"https://www.tiktok.com/@user"}, true)
	if len(planned) != 2 {
		t.Fatalf("ожидалось 2 ссылки, получено %d", len(planned))
	}
	if planned[0].Kind != model.KindProfile || planned[1].Kind != model.KindStory {
		t.Fatalf("неверные виды: %+v", planned)
	}
	// Дубликаты не добавляются.
	planned = ExpandURLs([]string{"https://www.tiktok.com/@user", "https://www.tiktok.com/@user"}, true)
	if len(planned) != 2 {
		t.Fatalf("дубликаты не должны размножаться: %+v", planned)
	}
}

func TestEngineOrder(t *testing.T) {
	if got := EngineOrder(model.KindStory, "auto"); len(got) == 0 || got[0] != "gallery-dl" {
		t.Fatalf("истории должен вести gallery-dl: %v", got)
	}
	if got := EngineOrder(model.KindVideoPost, "auto"); got[0] != "yt-dlp" {
		t.Fatalf("видео должен вести yt-dlp: %v", got)
	}
	if got := EngineOrder(model.KindVideoPost, "mock"); len(got) != 1 || got[0] != "mock" {
		t.Fatalf("явный движок должен оставаться один: %v", got)
	}
}

func TestFormatSelector(t *testing.T) {
	if got := FormatSelector("best"); got != "bestvideo*+bestaudio/best" {
		t.Fatalf("best -> %q", got)
	}
	if got := FormatSelector("1080"); got != "bestvideo[height<=1080]+bestaudio/best[height<=1080]" {
		t.Fatalf("1080 -> %q", got)
	}
	if got := FormatSelector("137+140"); got != "137+140" {
		t.Fatalf("готовый селектор должен остаться: %q", got)
	}
}

func TestIsTikTokURL(t *testing.T) {
	if !IsTikTokURL("https://www.tiktok.com/@user") {
		t.Fatal("tiktok.com должен распознаваться")
	}
	if !IsTikTokURL("https://vm.tiktok.com/abc") {
		t.Fatal("vm.tiktok.com должен распознаваться")
	}
	if IsTikTokURL("https://not-tiktok.com/@user") {
		t.Fatal("посторонний домен не должен распознаваться")
	}
}
