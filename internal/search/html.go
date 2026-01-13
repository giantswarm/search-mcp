package search

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// ConvertHTMLToMarkdown converts HTML content to clean Markdown
func ConvertHTMLToMarkdown(htmlContent, sourceURL string) (string, error) {
	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Remove unwanted elements
	doc.Find("aside.td-sidebar").Remove()
	doc.Find("script").Remove()

	// Get cleaned HTML
	cleanedHTML, err := doc.Html()
	if err != nil {
		return "", fmt.Errorf("failed to get cleaned HTML: %w", err)
	}

	// Convert to Markdown
	markdownContent, err := md.ConvertString(cleanedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to convert to markdown: %w", err)
	}

	// Clean up excessive whitespace
	cleaned := cleanupWhitespace(markdownContent)

	// Add header with source URL
	result := fmt.Sprintf("# Content from %s\n\n%s", sourceURL, cleaned)

	return result, nil
}

// cleanupWhitespace removes excessive blank lines from text
func cleanupWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		isEmpty := len(line) == 0

		// Skip multiple consecutive empty lines
		if isEmpty && prevEmpty {
			continue
		}

		cleaned = append(cleaned, line)
		prevEmpty = isEmpty
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
