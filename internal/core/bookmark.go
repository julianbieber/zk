package core

import (
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

// BookmarkTagCount holds a tag name and the number of bookmarks with that tag.
type BookmarkTagCount struct {
	Name          string
	BookmarkCount int
}

// BookmarkRepository provides read access to bookmarks.
type BookmarkRepository interface {
	FindAll(opts BookmarkFindOpts) ([]Bookmark, error)
	FindTags() ([]BookmarkTagCount, error)
}
