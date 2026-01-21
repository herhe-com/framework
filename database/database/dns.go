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

func postgreDSN(username, password, host, port, db, sslmode, timezone string) string {

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host,
		username,
		url.QueryEscape(password),
		db,
		port,
		sslmode,
		timezone)
}
