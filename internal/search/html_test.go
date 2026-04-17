package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contentSelector = ".base-content, .list-content"

// Markers that appear only in nav/footer chrome, never inside the article content.
var chromeMarkers = []string{
	"giantswarm.io/contact",
	"Giant Swarm Documentation",
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(data)
}

func TestExtractSelector_SinglePage(t *testing.T) {
	html := readFixture(t, "docs_single_page.html")

	got, err := ExtractSelector(html, contentSelector)
	if err != nil {
		t.Fatalf("ExtractSelector: %v", err)
	}

	if !strings.Contains(got, "App management") {
		t.Errorf("expected main heading %q in extracted content", "App management")
	}
	for _, marker := range chromeMarkers {
		if strings.Contains(got, marker) {
			t.Errorf("chrome marker %q leaked into extracted content", marker)
		}
	}
}

func TestExtractSelector_ListPage(t *testing.T) {
	html := readFixture(t, "docs_list_page.html")

	got, err := ExtractSelector(html, contentSelector)
	if err != nil {
		t.Fatalf("ExtractSelector: %v", err)
	}

	if !strings.Contains(got, "Fleet management") {
		t.Errorf("expected main heading %q in extracted content", "Fleet management")
	}
	for _, marker := range chromeMarkers {
		if strings.Contains(got, marker) {
			t.Errorf("chrome marker %q leaked into extracted content", marker)
		}
	}
}

func TestExtractSelector_NotFound(t *testing.T) {
	html := `<html><body><p>hello</p></body></html>`

	got, err := ExtractSelector(html, ".does-not-exist")
	if err != nil {
		t.Fatalf("ExtractSelector: %v", err)
	}
	if got != html {
		t.Errorf("expected original HTML when selector not found, got %q", got)
	}
}

func TestConvertHTMLToMarkdown_SinglePageStripsChrome(t *testing.T) {
	html := readFixture(t, "docs_single_page.html")

	extracted, err := ExtractSelector(html, contentSelector)
	if err != nil {
		t.Fatalf("ExtractSelector: %v", err)
	}

	md, err := ConvertHTMLToMarkdown(extracted, "https://docs.giantswarm.io/overview/fleet-management/app-management/")
	if err != nil {
		t.Fatalf("ConvertHTMLToMarkdown: %v", err)
	}

	if !strings.Contains(md, "App management") {
		t.Errorf("expected main heading in markdown output")
	}
	for _, marker := range chromeMarkers {
		if strings.Contains(md, marker) {
			t.Errorf("chrome marker %q leaked into markdown output", marker)
		}
	}
}

func TestConvertHTMLToMarkdown_ListPageStripsChrome(t *testing.T) {
	html := readFixture(t, "docs_list_page.html")

	extracted, err := ExtractSelector(html, contentSelector)
	if err != nil {
		t.Fatalf("ExtractSelector: %v", err)
	}

	md, err := ConvertHTMLToMarkdown(extracted, "https://docs.giantswarm.io/overview/fleet-management/")
	if err != nil {
		t.Fatalf("ConvertHTMLToMarkdown: %v", err)
	}

	if !strings.Contains(md, "Fleet management") {
		t.Errorf("expected main heading in markdown output")
	}
	for _, marker := range chromeMarkers {
		if strings.Contains(md, marker) {
			t.Errorf("chrome marker %q leaked into markdown output", marker)
		}
	}
}
