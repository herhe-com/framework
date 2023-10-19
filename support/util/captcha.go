package util

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func Captcha(length int8) string {

	numeric := [10]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	l := len(numeric)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	var i int8
	var sb strings.Builder

	for i = 0; i < length; i++ {
		_, _ = fmt.Fprintf(&sb, "%d", numeric[r.Intn(l)])
	}

	return sb.String()
}
