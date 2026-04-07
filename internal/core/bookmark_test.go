package core

import (
	"testing"

	"github.com/zk-org/zk/internal/util/test/assert"
)

func TestParseMarkdownLink(t *testing.T) {
	title, url, err := ParseMarkdownLink("[Go docs](https://go.dev)")
	assert.Nil(t, err)
	assert.Equal(t, title, "Go docs")
	assert.Equal(t, url, "https://go.dev")
}

func TestParseMarkdownLinkWithQueryParams(t *testing.T) {
	title, url, err := ParseMarkdownLink("[Search](https://example.com/search?q=test&lang=en)")
	assert.Nil(t, err)
	assert.Equal(t, title, "Search")
	assert.Equal(t, url, "https://example.com/search?q=test&lang=en")
}

func TestParseMarkdownLinkInvalid(t *testing.T) {
	_, _, err := ParseMarkdownLink("not a link")
	assert.NotNil(t, err)
}

func TestParseMarkdownLinkEmpty(t *testing.T) {
	_, _, err := ParseMarkdownLink("")
	assert.NotNil(t, err)
}

func TestParseMarkdownLinkBareURL(t *testing.T) {
	_, _, err := ParseMarkdownLink("https://example.com")
	assert.NotNil(t, err)
}
