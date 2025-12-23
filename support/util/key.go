package util

import (
	"fmt"
	"strings"

	"github.com/herhe-com/framework/facades"
)

func Keys(args ...any) string {

	name := facades.Cfg.GetString("app.name")

	names := make([]string, 0)

	names = append(names, name)

	for _, item := range args {
		names = append(names, fmt.Sprintf("%v", item))
	}

	return strings.Join(names, ":")
}
