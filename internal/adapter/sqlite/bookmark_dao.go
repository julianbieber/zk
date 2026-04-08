package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zk-org/zk/internal/core"
	"github.com/zk-org/zk/internal/util"
	"github.com/zk-org/zk/internal/util/errors"
)

// BookmarkDAO persists bookmarks extracted from notes in the SQLite database.
type BookmarkDAO struct {
	tx     Transaction
	logger util.Logger

	addBookmarkStmt        *LazyStmt
	addTagStmt             *LazyStmt
	removeBySourceNoteStmt *LazyStmt
}

func NewBookmarkDAO(tx Transaction, logger util.Logger) *BookmarkDAO {
	return &BookmarkDAO{
		tx:     tx,
		logger: logger,

		addBookmarkStmt: tx.PrepareLazy(`
			INSERT INTO bookmarks (title, url, source_note_id)
			VALUES (?, ?, ?)
			ON CONFLICT(url, source_note_id) DO UPDATE SET title = excluded.title
		`),

		addTagStmt: tx.PrepareLazy(`
			INSERT OR IGNORE INTO bookmark_tags (bookmark_id, name)
			VALUES (?, ?)
		`),

		removeBySourceNoteStmt: tx.PrepareLazy(`
			DELETE FROM bookmarks WHERE source_note_id = ?
		`),
	}
}

// Add inserts or updates a bookmark from a specific source note.
func (d *BookmarkDAO) Add(title string, url string, sourceNoteID core.NoteID, tags []string) (core.BookmarkID, error) {
	wrap := errors.Wrapperf("failed to add bookmark %s", url)

	res, err := d.addBookmarkStmt.Exec(title, url, sourceNoteID)
	if err != nil {
		return 0, wrap(err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, wrap(err)
	}

	// On conflict, LastInsertId may return 0. Look up the ID.
	if id == 0 {
		row := d.tx.QueryRow(
			`SELECT id FROM bookmarks WHERE url = ? AND source_note_id = ?`, url, sourceNoteID,
		)
		err = row.Scan(&id)
		if err != nil {
			return 0, wrap(err)
		}
	}

	for _, tag := range tags {
		_, err := d.addTagStmt.Exec(id, tag)
		if err != nil {
			return 0, wrap(err)
		}
	}

	return core.BookmarkID(id), nil
}

// RemoveBySourceNote deletes all bookmarks originating from the given note.
func (d *BookmarkDAO) RemoveBySourceNote(noteID core.NoteID) error {
	_, err := d.removeBySourceNoteStmt.Exec(noteID)
	return errors.Wrapf(err, "failed to remove bookmarks for note %d", noteID)
}

// FindAll returns bookmarks matching the given options.
// Multiple bookmarks with the same URL (from different notes) are deduplicated.
func (d *BookmarkDAO) FindAll(opts core.BookmarkFindOpts) ([]core.Bookmark, error) {
	wrap := errors.Wrapper("failed to list bookmarks")

	// Deduplicate by URL, taking the first title and aggregating all tags and source paths.
	query := `
		SELECT MIN(b.id) as id, b.url,
		       (SELECT b2.title FROM bookmarks b2 WHERE b2.url = b.url ORDER BY b2.created ASC LIMIT 1) as title,
		       (SELECT b2.created FROM bookmarks b2 WHERE b2.url = b.url ORDER BY b2.created ASC LIMIT 1) as created,
		       GROUP_CONCAT(DISTINCT bt.name) as tags,
		       GROUP_CONCAT(DISTINCT n.path) as sources
		  FROM bookmarks b
		  LEFT JOIN bookmark_tags bt ON bt.bookmark_id = b.id
		  LEFT JOIN notes n ON b.source_note_id = n.id
	`

	args := []interface{}{}
	conditions := []string{}

	if opts.Tag != "" {
		conditions = append(conditions,
			`EXISTS (SELECT 1 FROM bookmark_tags bt2
			          JOIN bookmarks b2 ON bt2.bookmark_id = b2.id
			         WHERE b2.url = b.url AND bt2.name = ?)`,
		)
		args = append(args, opts.Tag)
	}

	if opts.Match != "" {
		conditions = append(conditions, `(b.title LIKE ? OR b.url LIKE ?)`)
		pattern := fmt.Sprintf("%%%s%%", opts.Match)
		args = append(args, pattern, pattern)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY b.url ORDER BY created DESC"

	rows, err := d.tx.Query(query, args...)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()

	bookmarks := []core.Bookmark{}

	for rows.Next() {
		var id int64
		var title, url string
		var created time.Time
		var tags, sources sql.NullString

		err := rows.Scan(&id, &url, &title, &created, &tags, &sources)
		if err != nil {
			return bookmarks, wrap(err)
		}

		bookmark := core.Bookmark{
			ID:      core.BookmarkID(id),
			Title:   title,
			URL:     url,
			Created: created,
		}

		if tags.Valid && tags.String != "" {
			bookmark.Tags = strings.Split(tags.String, ",")
		}

		if sources.Valid && sources.String != "" {
			bookmark.Sources = strings.Split(sources.String, ",")
		}

		bookmarks = append(bookmarks, bookmark)
	}

	return bookmarks, nil
}

// FindTags returns all bookmark tags with their associated bookmark count.
func (d *BookmarkDAO) FindTags() ([]core.BookmarkTagCount, error) {
	wrap := errors.Wrapper("failed to list bookmark tags")

	rows, err := d.tx.Query(`
		SELECT bt.name, COUNT(DISTINCT b.url) as bookmark_count
		  FROM bookmark_tags bt
		  JOIN bookmarks b ON bt.bookmark_id = b.id
		 GROUP BY bt.name
		 ORDER BY bookmark_count DESC, bt.name ASC
	`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()

	var tags []core.BookmarkTagCount
	for rows.Next() {
		var tag core.BookmarkTagCount
		err := rows.Scan(&tag.Name, &tag.BookmarkCount)
		if err != nil {
			return tags, wrap(err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}
