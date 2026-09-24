package util

import (
	"crypto/rand"
	"math/big"
	"strings"
)

func Captcha(length int8) string {

	if length <= 0 {
		return ""
	}
	var sb strings.Builder
	for i := int8(0); i < length; i++ {
		digit, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return ""
		}
		sb.WriteByte(byte('0' + digit.Int64()))
	}

	return sb.String()
}
