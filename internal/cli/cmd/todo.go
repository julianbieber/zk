package cmd

import "github.com/zk-org/zk/internal/cli"

// Todo displays notes marked as TODO.
type Todo struct {
	List
}

func (cmd *Todo) Run(container *cli.Container) error {
	cmd.Filtering.Todo = true
	return cmd.List.Run(container)
}
