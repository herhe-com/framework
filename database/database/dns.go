package database

import (
	"fmt"
	"net/url"
)

func dns(username, password, host, port, prefix, db, charset string) string {

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s%s?charset=%s&parseTime=true&loc=Local",
		username,
		url.QueryEscape(password),
		host,
		port,
		prefix,
		db,
		charset)
}
