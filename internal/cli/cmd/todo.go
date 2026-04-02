package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zk-org/zk/internal/cli"
	"github.com/zk-org/zk/internal/core"
	strutil "github.com/zk-org/zk/internal/util/strings"
)

// Todo displays open TODO items (unchecked checkboxes) found in notes.
type Todo struct {
	Format  string `short:"f" placeholder:"TEMPLATE" help:"Pretty print using a custom template or a predefined format: short, full, json."`
	NoPager bool   `short:"P" help:"Do not pipe output into a pager."`
	Quiet   bool   `short:"q" help:"Do not print the total number of TODOs found."`
	cli.Filtering
}

func (cmd *Todo) Run(container *cli.Container) error {
	notebook, err := container.CurrentNotebook()
	if err != nil {
		return err
	}

	// Find notes containing unchecked checkboxes.
	findOpts, err := cmd.Filtering.NewNoteFindOpts(notebook)
	if err != nil {
		return err
	}
	findOpts.Match = append(findOpts.Match, `- [ ]`)
	findOpts.MatchStrategy = core.MatchStrategyExact

	notes, err := notebook.FindNotes(findOpts)
	if err != nil {
		return err
	}

	type todoItem struct {
		Path  string
		Title string
		Line  int
		Text  string
	}

	var items []todoItem
	for _, note := range notes {
		lines := strings.Split(note.RawContent, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ] ") || trimmed == "- [ ]" {
				text := strings.TrimPrefix(trimmed, "- [ ] ")
				items = append(items, todoItem{
					Path:  note.Path,
					Title: note.Title,
					Line:  i + 1,
					Text:  text,
				})
			}
		}
	}

	count := len(items)
	if count > 0 {
		err = container.Paginate(cmd.NoPager, func(out io.Writer) error {
			for _, item := range items {
				fmt.Fprintf(out, "[ ] %s\n    %s:%d\n", item.Text, item.Path, item.Line)
			}
			return nil
		})
	}

	if err == nil && !cmd.Quiet {
		fmt.Fprintf(os.Stderr, "\nFound %d %s\n", count, strutil.Pluralize("TODO", count))
	}

	return err
}
