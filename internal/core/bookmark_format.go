package core

import "time"

// BookmarkFormatter formats bookmarks to be printed on the screen.
type BookmarkFormatter func(bookmark Bookmark) (string, error)

func newBookmarkFormatter(template Template) (BookmarkFormatter, error) {
	return func(bookmark Bookmark) (string, error) {
		return template.Render(bookmarkFormatRenderContext{
			ID:      bookmark.ID,
			Title:   bookmark.Title,
			URL:     bookmark.URL,
			Tags:    bookmark.Tags,
			Sources: bookmark.Sources,
			Created: bookmark.Created,
		})
	}, nil
}

// bookmarkFormatRenderContext holds the variables available to the
// bookmark formatting templates.
type bookmarkFormatRenderContext struct {
	ID      BookmarkID `json:"id"`
	Title   string     `json:"title"`
	URL     string     `json:"url" handlebars:"url"`
	Tags    []string   `json:"tags"`
	Sources []string   `json:"sources"`
	Created time.Time  `json:"created"`
}

// BookmarkTagFormatter formats bookmark tag counts to be printed on the screen.
type BookmarkTagFormatter func(tag BookmarkTagCount) (string, error)

func newBookmarkTagFormatter(template Template) (BookmarkTagFormatter, error) {
	return func(tag BookmarkTagCount) (string, error) {
		return template.Render(bookmarkTagFormatRenderContext{
			Name:          tag.Name,
			BookmarkCount: tag.BookmarkCount,
		})
	}, nil
}

// bookmarkTagFormatRenderContext holds the variables available to the
// bookmark tag formatting templates.
type bookmarkTagFormatRenderContext struct {
	Name          string `json:"name"`
	BookmarkCount int    `json:"bookmarkCount" handlebars:"bookmark-count"`
}
