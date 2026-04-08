package sqlite

import (
	"github.com/zk-org/zk/internal/core"
	"github.com/zk-org/zk/internal/util"
)

// BookmarkStore implements core.BookmarkRepository using the SQLite database.
type BookmarkStore struct {
	db     *DB
	logger util.Logger
}

func NewBookmarkStore(db *DB, logger util.Logger) *BookmarkStore {
	return &BookmarkStore{db: db, logger: logger}
}

func (s *BookmarkStore) FindAll(opts core.BookmarkFindOpts) ([]core.Bookmark, error) {
	var bookmarks []core.Bookmark
	err := s.db.WithTransaction(func(tx Transaction) error {
		dao := NewBookmarkDAO(tx, s.logger)
		var err error
		bookmarks, err = dao.FindAll(opts)
		return err
	})
	return bookmarks, err
}

func (s *BookmarkStore) FindTags() ([]core.BookmarkTagCount, error) {
	var tags []core.BookmarkTagCount
	err := s.db.WithTransaction(func(tx Transaction) error {
		dao := NewBookmarkDAO(tx, s.logger)
		var err error
		tags, err = dao.FindTags()
		return err
	})
	return tags, err
}
