package sqlite

import (
	"testing"

	"github.com/zk-org/zk/internal/core"
	"github.com/zk-org/zk/internal/util"
	"github.com/zk-org/zk/internal/util/opt"
	"github.com/zk-org/zk/internal/util/test/assert"
)

func testBookmarkDAO(t *testing.T, test func(tx Transaction, dao *BookmarkDAO)) {
	testTransactionWithFixtures(t, opt.NullString, func(tx Transaction) {
		dao := NewBookmarkDAO(tx, &util.NullLogger)
		// Create a dummy note to use as source.
		_, err := tx.Exec(`
			INSERT INTO notes (id, path, sortable_path, title, body, word_count, checksum)
			VALUES (100, "test/note1.md", "testnote1.md", "Test Note 1", "body", 1, "abc")
		`)
		assert.Nil(t, err)
		_, err = tx.Exec(`
			INSERT INTO notes (id, path, sortable_path, title, body, word_count, checksum)
			VALUES (200, "test/note2.md", "testnote2.md", "Test Note 2", "body", 1, "def")
		`)
		assert.Nil(t, err)
		test(tx, dao)
	})
}

func TestBookmarkAdd(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		id, err := dao.Add("Go docs", "https://go.dev", 100, []string{"language", "reference"})
		assert.Nil(t, err)
		assert.Equal(t, id.IsValid(), true)

		assertExistTx(t, tx, "SELECT id FROM bookmarks WHERE url = 'https://go.dev' AND title = 'Go docs'")
		assertExistTx(t, tx, "SELECT id FROM bookmark_tags WHERE bookmark_id = ? AND name = 'language'", id)
		assertExistTx(t, tx, "SELECT id FROM bookmark_tags WHERE bookmark_id = ? AND name = 'reference'", id)
	})
}

func TestBookmarkAddNoTags(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		id, err := dao.Add("Example", "https://example.com", 100, nil)
		assert.Nil(t, err)
		assert.Equal(t, id.IsValid(), true)

		assertExistTx(t, tx, "SELECT id FROM bookmarks WHERE url = 'https://example.com'")
		assertNotExistTx(t, tx, "SELECT id FROM bookmark_tags WHERE bookmark_id = ?", id)
	})
}

func TestBookmarkAddDuplicateURLSameSourceUpdatesTitle(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		_, err := dao.Add("Old title", "https://go.dev", 100, nil)
		assert.Nil(t, err)

		id, err := dao.Add("New title", "https://go.dev", 100, []string{"updated"})
		assert.Nil(t, err)
		assert.Equal(t, id.IsValid(), true)

		assertExistTx(t, tx, "SELECT id FROM bookmarks WHERE url = 'https://go.dev' AND title = 'New title'")
		assertNotExistTx(t, tx, "SELECT id FROM bookmarks WHERE url = 'https://go.dev' AND title = 'Old title'")
	})
}

func TestBookmarkAddSameURLDifferentSources(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		_, err := dao.Add("Go docs", "https://go.dev", 100, []string{"lang"})
		assert.Nil(t, err)
		_, err = dao.Add("Go docs", "https://go.dev", 200, []string{"reference"})
		assert.Nil(t, err)

		// Should deduplicate in FindAll
		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, bookmarks[0].URL, "https://go.dev")
	})
}

func TestBookmarkFindAll(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, []string{"language"})
		dao.Add("Rust book", "https://doc.rust-lang.org/book/", 100, []string{"language", "book"})
		dao.Add("Example", "https://example.com", 200, nil)

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 3)
	})
}

func TestBookmarkFindAllByTag(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, []string{"language"})
		dao.Add("Rust book", "https://doc.rust-lang.org/book/", 100, []string{"language", "book"})
		dao.Add("Example", "https://example.com", 200, nil)

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{Tag: "language"})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 2)

		bookmarks, err = dao.FindAll(core.BookmarkFindOpts{Tag: "book"})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, bookmarks[0].Title, "Rust book")
	})
}

func TestBookmarkFindAllByMatch(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, nil)
		dao.Add("Rust book", "https://doc.rust-lang.org/book/", 100, nil)

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{Match: "rust"})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, bookmarks[0].Title, "Rust book")
	})
}

func TestBookmarkFindAllByMatchURL(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, nil)
		dao.Add("Rust book", "https://doc.rust-lang.org/book/", 100, nil)

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{Match: "go.dev"})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, bookmarks[0].Title, "Go docs")
	})
}

func TestBookmarkRemoveBySourceNote(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, []string{"lang"})
		dao.Add("Rust book", "https://doc.rust-lang.org/book/", 200, nil)

		err := dao.RemoveBySourceNote(100)
		assert.Nil(t, err)

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, bookmarks[0].Title, "Rust book")
	})
}

func TestBookmarkTagsAggregatedAcrossSources(t *testing.T) {
	testBookmarkDAO(t, func(tx Transaction, dao *BookmarkDAO) {
		dao.Add("Go docs", "https://go.dev", 100, []string{"language"})
		dao.Add("Go docs", "https://go.dev", 200, []string{"reference"})

		bookmarks, err := dao.FindAll(core.BookmarkFindOpts{})
		assert.Nil(t, err)
		assert.Equal(t, len(bookmarks), 1)
		assert.Equal(t, len(bookmarks[0].Tags), 2)
	})
}
