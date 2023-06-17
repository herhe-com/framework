package util

const (
	PatternOfMobile    = `^1\d{10}$`
	PatternOfUsername  = `^[a-zA-Z\d\-_]{6,32}$`
	PatternOfPassword  = `^[a-zA-Z\d\-_@$&%!]{6,32}$`
	PatternOfSnowflake = `^\d{16,64}$`
	PatternOfDirs      = `^(/[\da-zA-Z_\-]{1,64}){1,8}$`
)
