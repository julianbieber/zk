package sqlite

import (
	"path/filepath"
	"regexp"
	"strings"

	"fmt"

	"github.com/zk-org/zk/internal/core"
	"github.com/zk-org/zk/internal/util"
	"github.com/zk-org/zk/internal/util/paths"
	strutil "github.com/zk-org/zk/internal/util/strings"
)

// NoteIndex persists note indexing results in the SQLite database.
// It implements the port core.NoteIndex and acts as a facade to the DAOs.
type NoteIndex struct {
	notebookPath string
	db           *DB
	dao          *dao
	logger       util.Logger
	extension    string
}

type dao struct {
	notes       *NoteDAO
	links       *LinkDAO
	collections *CollectionDAO
	metadata    *MetadataDAO
}

func NewNoteIndex(notebookPath string, db *DB, logger util.Logger, extension string) *NoteIndex {
	return &NoteIndex{
		notebookPath: notebookPath,
		db:           db,
		logger:       logger,
		extension:    extension,
	}
}

// Find implements core.NoteIndex.
func (ni *NoteIndex) Find(opts core.NoteFindOpts) (notes []core.ContextualNote, err error) {
	err = ni.commit(func(dao *dao) error {
		notes, err = dao.notes.Find(opts)
		return err
	})
	return
}

// FindMinimal implements core.NoteIndex.
func (ni *NoteIndex) FindMinimal(opts core.NoteFindOpts) (notes []core.MinimalNote, err error) {
	err = ni.commit(func(dao *dao) error {
		notes, err = dao.notes.FindMinimal(opts)
		return err
	})
	return
}

// findLinkMatch finds note IDs which match the given href string.
// This implements logic similar to NoteIndex.linkMatchesPath.
func (ni *NoteIndex) findLinkMatch(dao *dao, baseDir string, href string, linkType core.LinkType) (core.NoteID, error) {
	if href == "" || strutil.IsURL(href) {
		return 0, nil
	}

	// Remove any anchor, if any.
	href = strings.SplitN(href, "#", 2)[0]

	// Relative path: prefix with no slash after, or immediately followed by slash.
	relHref, err := ni.relNotebookPath(baseDir, href)
	if err != nil {
		return 0, err
	}
	id, err := ni.findByPathPrefix(dao, relHref)
	if err != nil {
		return 0, err
	}
	if id.IsValid() {
		return id, nil
	}

	if linkType == core.LinkTypeWikiLink {
		// Filename contains href.
		ids, err := dao.notes.findIDsWithStmt(dao.notes.findIDsByFilenameLikeStmt, "%"+escapeLike(href)+"%")
		if err != nil {
			return 0, err
		}
		if len(ids) > 0 {
			return ids[0], nil
		}

		// Path contains href.
		ids, err = dao.notes.findIDsWithStmt(dao.notes.findIDsByPathLikeStmt, "%"+escapeLike(href)+"%")
		if err != nil {
			return 0, err
		}
		if len(ids) > 0 {
			return ids[0], nil
		}
	}

	// Absolute path: prefix with no slash after, or followed by slash.
	return ni.findByPathPrefix(dao, href)
}

// findByPathPrefix finds a note where path has prefix href (no slash after) or href/ (directory).
func (ni *NoteIndex) findByPathPrefix(dao *dao, href string) (core.NoteID, error) {
	// Try exact match with extension first (most common case).
	id, err := dao.notes.FindIDByPath(href + "." + ni.extension)
	if err != nil {
		return 0, err
	}
	if id.IsValid() {
		return id, nil
	}

	// Try exact match without extension.
	id, err = dao.notes.FindIDByPath(href)
	if err != nil {
		return 0, err
	}
	if id.IsValid() {
		return id, nil
	}

	// Use LIKE patterns: href% but not href%/% (file prefix) OR href/% (directory).
	href = escapeLike(href)
	ids, err := dao.notes.findIDsWithStmt(
		dao.notes.findIDsByPathPrefixStmt,
		href+"%", href+"%/%", href+"/%",
	)
	if err != nil {
		return 0, err
	}
	if len(ids) > 0 {
		return ids[0], nil
	}

	return 0, nil
}

// escapeLike escapes special characters for SQL LIKE patterns.
func escapeLike(s string) string {
	return strings.NewReplacer("%", "\\%", "_", "\\_").Replace(s)
}

// FindLinksBetweenNotes implements core.NoteIndex.
func (ni *NoteIndex) FindLinksBetweenNotes(ids []core.NoteID) (links []core.ResolvedLink, err error) {
	err = ni.commit(func(dao *dao) error {
		links, err = dao.links.FindBetweenNotes(ids)
		return err
	})
	return
}

// FindCollections implements core.NoteIndex.
func (ni *NoteIndex) FindCollections(kind core.CollectionKind, sorters []core.CollectionSorter) (collections []core.Collection, err error) {
	err = ni.commit(func(dao *dao) error {
		collections, err = dao.collections.FindAll(kind, sorters)
		return err
	})
	return
}

// IndexedPaths implements core.NoteIndex.
func (ni *NoteIndex) IndexedPaths() (metadata <-chan paths.Metadata, err error) {
	err = ni.commit(func(dao *dao) error {
		metadata, err = dao.notes.Indexed()
		return err
	})
	if err != nil {
		err = fmt.Errorf("failed to get indexed notes: %w", err)
	}
	return
}

// Add implements core.NoteIndex.
func (ni *NoteIndex) Add(note core.Note) (id core.NoteID, err error) {
	err = ni.commit(func(dao *dao) error {
		id, err = dao.notes.Add(note)
		if err != nil {
			return err
		}
		note.ID = id

		err = ni.addLinks(dao, id, note.Path, note.Links)
		if err != nil {
			return err
		}

		err = ni.fixExistingLinks(dao, note.ID, note.Path)
		if err != nil {
			return err
		}

		return ni.associateTags(dao.collections, id, note.Tags)
	})

	if err != nil {
		err = fmt.Errorf("%v: failed to index the note: %w", note.Path, err)
	}
	return
}

// fixExistingLinks will go over all indexed links and update their target to
// the given id if they match the given path better than their current
// targetPath.
func (ni *NoteIndex) fixExistingLinks(dao *dao, id core.NoteID, path string) error {
	links, err := dao.links.FindInternal()
	if err != nil {
		return err
	}

	for _, link := range links {
		// To find the best match possible, shortest paths take precedence.
		// See https://github.com/zk-org/zk/issues/23
		if link.TargetPath != "" && len(link.TargetPath) < len(path) {
			continue
		}

		matches, err := ni.linkMatchesPath(link, path)
		if matches && err == nil {
			err = dao.links.SetTargetID(link.ID, id)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// linkMatchesPath returns whether the given link can be used to reach the
// given note path.
func (ni *NoteIndex) linkMatchesPath(link core.ResolvedLink, path string) (bool, error) {
	// Remove any anchor at the end of the HREF, since it's most likely
	// matching a sub-section in the note.
	href := strings.SplitN(link.Href, "#", 2)[0]

	matchString := func(pattern string, s string) bool {
		reg := regexp.MustCompile(pattern)
		return reg.MatchString(s)
	}

	matches := func(href string, allowPartialHref bool) bool {
		if href == "" {
			return false
		}
		href = regexp.QuoteMeta(href)

		if allowPartialHref {
			if matchString("^(.*/)?[^/]*"+href+"[^/]*$", path) {
				return true
			}
			if matchString(".*"+href+".*", path) {
				return true
			}
		}

		return matchString("^(?:"+href+"[^/]*|"+href+"/.+)$", path)
	}

	baseDir := filepath.Join(ni.notebookPath, filepath.Dir(link.SourcePath))
	if relHref, err := ni.relNotebookPath(baseDir, href); err != nil {
		if matches(relHref, false) {
			return true, nil
		}
	}

	allowPartialMatch := (link.Type == core.LinkTypeWikiLink)
	return matches(href, allowPartialMatch), nil
}

// relNotebookHref makes the given href (which is relative to baseDir) relative
// to the notebook root instead.
func (ni *NoteIndex) relNotebookPath(baseDir string, href string) (string, error) {
	path := filepath.Clean(filepath.Join(baseDir, href))
	path, err := filepath.Rel(ni.notebookPath, path)

	if err != nil {
		return "", fmt.Errorf("failed to make href relative to the notebook: %s: %w", href, err)
	}
	return path, nil
}

// Update implements core.NoteIndex.
func (ni *NoteIndex) Update(note core.Note) error {
	err := ni.commit(func(dao *dao) error {
		id, err := dao.notes.Update(note)
		if err != nil {
			return err
		}

		// Reset links
		err = dao.links.RemoveAll(id)
		if err != nil {
			return err
		}
		err = ni.addLinks(dao, id, note.Path, note.Links)
		if err != nil {
			return err
		}

		// Reset tags
		err = dao.collections.RemoveAssociations(id)
		if err != nil {
			return err
		}
		return ni.associateTags(dao.collections, id, note.Tags)
	})

	if err != nil {
		return fmt.Errorf("%v: failed to update note index: %w", note.Path, err)
	}
	return nil
}

func (ni *NoteIndex) associateTags(collections *CollectionDAO, noteID core.NoteID, tags []string) error {
	for _, tag := range tags {
		tagID, err := collections.FindOrCreate(core.CollectionKindTag, tag)
		if err != nil {
			return err
		}
		_, err = collections.Associate(noteID, tagID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ni *NoteIndex) addLinks(dao *dao, id core.NoteID, sourcePath string, links []core.Link) error {
	resolvedLinks, err := ni.resolveLinkNoteIDs(dao, id, sourcePath, links)
	if err != nil {
		return err
	}
	return dao.links.Add(resolvedLinks)
}

func (ni *NoteIndex) resolveLinkNoteIDs(dao *dao, sourceID core.NoteID, sourcePath string, links []core.Link) ([]core.ResolvedLink, error) {
	resolvedLinks := []core.ResolvedLink{}
	baseDir := filepath.Join(ni.notebookPath, filepath.Dir(sourcePath))

	for _, link := range links {
		targetID, err := ni.findLinkMatch(dao, baseDir, link.Href, link.Type)
		if err != nil {
			return resolvedLinks, err
		}

		resolvedLinks = append(resolvedLinks, core.ResolvedLink{
			Link:     link,
			SourceID: sourceID,
			TargetID: targetID,
		})
	}

	return resolvedLinks, nil
}

// Remove implements core.NoteIndex
func (ni *NoteIndex) Remove(path string) error {
	err := ni.commit(func(dao *dao) error {
		return dao.notes.Remove(path)
	})
	if err != nil {
		return fmt.Errorf("%v: failed to remove note from index: %w", path, err)
	}
	return nil
}

// Commit implements core.NoteIndex.
func (ni *NoteIndex) Commit(transaction func(idx core.NoteIndex) error) error {
	return ni.commit(func(dao *dao) error {
		return transaction(&NoteIndex{
			db:     ni.db,
			dao:    dao,
			logger: ni.logger,
		})
	})
}

// NeedsReindexing implements core.NoteIndex.
func (ni *NoteIndex) NeedsReindexing() (needsReindexing bool, err error) {
	err = ni.commit(func(dao *dao) error {
		res, err := dao.metadata.Get(reindexingRequiredKey)
		needsReindexing = (res == "true")
		return err
	})
	return
}

// SetNeedsReindexing implements core.NoteIndex.
func (ni *NoteIndex) SetNeedsReindexing(needsReindexing bool) error {
	return ni.commit(func(dao *dao) error {
		value := "false"
		if needsReindexing {
			value = "true"
		}

		return dao.metadata.Set(reindexingRequiredKey, value)
	})
}

func (ni *NoteIndex) commit(transaction func(dao *dao) error) error {
	if ni.dao != nil {
		return transaction(ni.dao)
	} else {
		return ni.db.WithTransaction(func(tx Transaction) error {
			dao := dao{
				notes:       NewNoteDAO(tx, ni.logger, ni.extension),
				links:       NewLinkDAO(tx, ni.logger),
				collections: NewCollectionDAO(tx, ni.logger),
				metadata:    NewMetadataDAO(tx),
			}
			return transaction(&dao)
		})
	}
}
