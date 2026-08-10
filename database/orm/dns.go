package orm

import (
	"net"
	"net/url"
	"time"

	"github.com/go-sql-driver/mysql"
)

func mysqlDSN(username, password, host, port, db, charset string, timeouts ...time.Duration) string {

	cfg := mysql.NewConfig()
	cfg.User = username
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = db
	cfg.AllowNativePasswords = true
	cfg.ParseTime = true
	cfg.Loc = time.Local
	if len(timeouts) > 0 {
		cfg.Timeout = timeouts[0]
	}
	if len(timeouts) > 1 {
		cfg.ReadTimeout = timeouts[1]
	}
	if len(timeouts) > 2 {
		cfg.WriteTimeout = timeouts[2]
	}

	if charset != "" {
		cfg.Params = map[string]string{
			"charset": charset,
		}
	}

	return cfg.FormatDSN()
}

func postgreDSN(username, password, host, port, db, sslmode, timezone string) string {

	values := url.Values{}
	values.Set("sslmode", sslmode)
	values.Set("TimeZone", timezone)

	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(username, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + db,
		RawQuery: values.Encode(),
	}).String()
}

func sqlserverDSN(username, password, host, port, db string) string {

	values := url.Values{}
	values.Set("database", db)

	return (&url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(username, password),
		Host:     net.JoinHostPort(host, port),
		RawQuery: values.Encode(),
	}).String()
}
