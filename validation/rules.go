package validation

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/herhe-com/framework/contracts/util"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// captcha 验证验证码
func captcha(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfCaptcha, fl.Field().String())
	return ok
}

// mobile 验证手机号码
func mobile(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfMobile, fl.Field().String())
	return ok
}

// idCard 验证身份证号码
func idCard(fl validator.FieldLevel) bool {
	id := fl.Field().String()

	var a1Map = map[int]int{
		0:  1,
		1:  0,
		2:  10,
		3:  9,
		4:  8,
		5:  7,
		6:  6,
		7:  5,
		8:  4,
		9:  3,
		10: 2,
	}

	var idStr = strings.ToUpper(id)
	var reg, err = regexp.Compile(`^\d{17}[\dX]$`)
	if err != nil {
		return false
	}
	if !reg.Match([]byte(idStr)) {
		return false
	}
	var sum int
	var signChar = ""
	for index, c := range idStr {
		var i = 18 - index
		if i != 1 {
			if v, err := strconv.Atoi(string(c)); err == nil {
				var weight = int(math.Pow(2, float64(i-1))) % 11
				sum += v * weight
			} else {
				return false
			}
		} else {
			signChar = string(c)
		}
	}
	var a1 = a1Map[sum%11]
	var a1Str = fmt.Sprintf("%d", a1)
	if a1 == 10 {
		a1Str = "X"
	}
	return a1Str == signChar
}

func dirs(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfDirs, fl.Field().String())
	return ok
}

func username(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfUsername, fl.Field().String())
	return ok
}

func password(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfPassword, fl.Field().String())
	return ok
}

func snowflake(fl validator.FieldLevel) bool {
	ok, _ := regexp.MatchString(util.PatternOfSnowflake, fl.Field().String())
	return ok
}
