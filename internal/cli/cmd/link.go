package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zk-org/zk/internal/cli"
	"github.com/zk-org/zk/internal/core"
	"github.com/zk-org/zk/internal/util/errors"
	"github.com/zk-org/zk/internal/util/opt"
	"github.com/zk-org/zk/internal/util/strings"
)

// Link manages bookmarks in the notebook.
type Link struct {
	Add  LinkAdd  `cmd group:"cmd" help:"Add a new bookmark by creating a note containing the link."`
	List LinkList `cmd group:"cmd" default:"withargs" help:"List bookmarks extracted from notes."`
}

// LinkAdd creates a note file containing a markdown link.
type LinkAdd struct {
	Bookmark  string   `arg required help:"Bookmark in markdown link syntax: [Title](URL)."`
	Tags      []string `short:t sep:"," help:"Tags for the created note."`
	Directory string   `arg optional default:"." help:"Directory in which to create the note."`
}

func (cmd *LinkAdd) Run(container *cli.Container) error {
	title, url, err := core.ParseMarkdownLink(cmd.Bookmark)
	if err != nil {
		return err
	}

	notebook, err := container.CurrentNotebook()
	if err != nil {
		return err
	}

	// Build the note file content with frontmatter.
	var body string
	body = "---\ntitle: \"" + title + "\"\n"
	if len(cmd.Tags) > 0 {
		body += "tags:\n"
		for _, tag := range cmd.Tags {
			body += "  - " + tag + "\n"
		}
	}
	body += "---\n\n"
	body += fmt.Sprintf("[%s](%s)\n", title, url)

	// Use NewNote to generate the filename and index, passing the full
	// content including frontmatter. The content is used as-is when no
	// body template is configured (which renders the content via {{content}}).
	note, err := notebook.NewNote(core.NewNoteOpts{
		Title:     opt.NewNotEmptyString(title),
		Content:   body,
		Directory: opt.NewNotEmptyString(cmd.Directory),
		Date:      time.Now(),
	})
	if err != nil {
		return err
	}

	// If no body template was configured, NewNote's NullTemplate produces
	// empty content. Overwrite the file with our full content.
	notePath := filepath.Join(notebook.Path, note.Path)
	if note.RawContent == "" {
		err = container.FS.Write(notePath, []byte(body))
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "Bookmark note created: %s\n", notePath)
	return nil
}

// LinkList lists bookmarks matching the given criteria.
type LinkList struct {
	Tag        string `short:t help:"Filter bookmarks by tag."`
	Match      string `short:m help:"Search bookmarks by title or URL."`
	Format     string `group:format short:f placeholder:TEMPLATE help:"Pretty print the list using a custom template or one of the predefined formats: short, full, json, jsonl."`
	Header     string `group:format help:"Arbitrary text printed at the start of the list."`
	Footer     string `group:format default:\n help:"Arbitrary text printed at the end of the list."`
	Delimiter  string "group:format short:d default:\\n help:\"Print bookmarks delimited by the given separator.\""
	Delimiter0 bool   "group:format short:0 name:delimiter0 help:\"Print bookmarks delimited by ASCII NUL characters. This is useful when used in conjunction with `xargs -0`.\""
	NoPager    bool   `group:format short:P help:"Do not pipe output into a pager."`
	Quiet      bool   `group:format short:q help:"Do not print the total number of bookmarks found."`
}

func (cmd *LinkList) Run(container *cli.Container) error {
	cmd.Header = strings.ExpandWhitespaceLiterals(cmd.Header)
	cmd.Footer = strings.ExpandWhitespaceLiterals(cmd.Footer)
	cmd.Delimiter = strings.ExpandWhitespaceLiterals(cmd.Delimiter)

	if cmd.Delimiter0 {
		if cmd.Delimiter != "\n" {
			return errors.New("--delimiter and --delimiter0 can't be used together")
		}
		if cmd.Header != "" {
			return errors.New("--header and --delimiter0 can't be used together")
		}
		if cmd.Footer != "\n" {
			return errors.New("--footer and --delimiter0 can't be used together")
		}

		cmd.Delimiter = "\x00"
		cmd.Footer = "\x00"
	}

	if cmd.Format == "json" || cmd.Format == "jsonl" {
		if cmd.Header != "" {
			return errors.New("--header can't be used with JSON format")
		}
		if cmd.Footer != "\n" {
			return errors.New("--footer can't be used with JSON format")
		}
		if cmd.Delimiter != "\n" {
			return errors.New("--delimiter can't be used with JSON format")
		}

		switch cmd.Format {
		case "json":
			cmd.Delimiter = ","
			cmd.Header = "["
			cmd.Footer = "]\n"
		case "jsonl":
			cmd.Footer = "\n"
		}
	}

	notebook, err := container.CurrentNotebook()
	if err != nil {
		return err
	}

	format, err := notebook.NewBookmarkFormatter(cmd.bookmarkTemplate())
	if err != nil {
		return err
	}

	opts := core.BookmarkFindOpts{
		Tag:   cmd.Tag,
		Match: cmd.Match,
	}

	bookmarks, err := notebook.FindBookmarks(opts)
	if err != nil {
		return err
	}

	count := len(bookmarks)
	if count > 0 {
		err = container.Paginate(cmd.NoPager, func(out io.Writer) error {
			if cmd.Header != "" {
				fmt.Fprint(out, cmd.Header)
			}
			for i, bookmark := range bookmarks {
				if i > 0 {
					fmt.Fprint(out, cmd.Delimiter)
				}

				ft, err := format(bookmark)
				if err != nil {
					return err
				}
				fmt.Fprint(out, ft)
			}
			if cmd.Footer != "" {
				fmt.Fprint(out, cmd.Footer)
			}

			return nil
		})
	}

	if err == nil && !cmd.Quiet {
		fmt.Fprintf(os.Stderr, "\nFound %d %s\n", count, strings.Pluralize("bookmark", count))
	}

	return err
}

func (cmd *LinkList) bookmarkTemplate() string {
	format := cmd.Format
	if format == "" {
		format = "full"
	}

	templ, ok := defaultBookmarkFormats[format]
	if !ok {
		templ = strings.ExpandWhitespaceLiterals(format)
	}

	return templ
}

var defaultBookmarkFormats = map[string]string{
	"json":  `{{json .}}`,
	"jsonl": `{{json .}}`,
	"short": `{{style "title" title}} {{style "understate" url}}`,
	"full": `{{style "title" title}} {{style "understate" url}}{{#if tags}}
    {{#each tags}}{{#unless @first}} {{/unless}}{{style "term" this}}{{/each}}{{/if}}{{#if sources}}
    {{#each sources}}{{#unless @first}}, {{/unless}}{{style "path" this}}{{/each}}{{/if}}`,
}
