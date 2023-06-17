package facades

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/spf13/cobra"
)

var Server *server.Hertz

var Console *cobra.Command
