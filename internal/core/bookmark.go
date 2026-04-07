package core

import (
	"fmt"
	"regexp"
	"time"
)

// BookmarkID represents the unique serial number of a bookmark.
type BookmarkID int64

func (id BookmarkID) IsValid() bool {
	return id > 0
}

// Bookmark holds a bookmark extracted from a note's external links.
type Bookmark struct {
	ID      BookmarkID
	Title   string
	URL     string
	Tags    []string
	Sources []string
	Created time.Time
}

// BookmarkFindOpts holds filtering options for listing bookmarks.
type BookmarkFindOpts struct {
	Tag   string
	Match string
}

// BookmarkRepository provides read access to bookmarks.
type BookmarkRepository interface {
	FindAll(opts BookmarkFindOpts) ([]Bookmark, error)
}

var markdownLinkRegex = regexp.MustCompile(`^\[(.+?)\]\((.+?)\)$`)

// ParseMarkdownLink extracts the title and URL from a markdown link string.
func ParseMarkdownLink(input string) (title string, url string, err error) {
	matches := markdownLinkRegex.FindStringSubmatch(input)
	if matches == nil {
		return "", "", fmt.Errorf("invalid markdown link syntax, expected [Title](URL): %s", input)
	}
	return matches[1], matches[2], nil
}
