package console

import "github.com/spf13/cobra"

type Console struct {
	Cmd      string
	Name     string
	Summary  string
	Consoles []Console
	Run      func(cmd *cobra.Command, args []string)
	Tags     func(cmd *cobra.Command)
}

type SelectOfCli struct {
	Key   string
	Label string
}
