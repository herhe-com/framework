package validation

import (
	"fmt"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/herhe-com/framework/contracts/global"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type rule struct {
	tag          string
	pattern      string
	valid        func(fl validator.FieldLevel) bool
	translation  string
	translations map[string]string
}

func register(vd *validator.Validate, trans ut.Translator, language string) (err error) {

	for _, item := range rules() {

		fn := item.valid

		if item.pattern != "" {
			fn = func(fl validator.FieldLevel) bool {
				ok, _ := regexp.MatchString(item.pattern, fl.Field().String())
				return ok
			}
		}

		if err = vd.RegisterValidation(item.tag, fn); err != nil {
			return err
		}

		text := item.translation

		if txt, ok := item.translations[language]; ok {
			text = txt
		}

		if err = vd.RegisterTranslation(item.tag, trans,
			func(ut ut.Translator) error {
				return ut.Add(item.tag, text, true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(fe.Tag(), fe.Field(), fe.Param())
				return t
			}); err != nil {
			return err
		}
	}

	return nil
}

func rules() []rule {

	return []rule{
		{
			tag:         "captcha",
			pattern:     global.PatternOfCaptcha,
			translation: "{0} must be a valid CAPTCHA",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的验证码",
				"ja": "{0}は正しい認証コードでなければならない",
			},
		},
		{
			tag:         "pinyin",
			pattern:     global.PatternOfPinyin,
			translation: "{0} must be a valid pinyin",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的拼音",
			},
		},
		{
			tag:         "mobile",
			pattern:     global.PatternOfMobile,
			translation: "{0} must be a valid mobile phone number",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的手机号",
			},
		},
		{
			tag:         "dirs",
			pattern:     global.PatternOfDirs,
			translation: "{0} must be a valid dir address",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的文件夹",
			},
		},
		{
			tag:         "username",
			pattern:     global.PatternOfUsername,
			translation: "{0} must be a valid username",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的用户名",
			},
		},
		{
			tag:         "password",
			pattern:     global.PatternOfPassword,
			translation: "{0} must be a valid password",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的密码",
			},
		},
		{
			tag:         "snowflake",
			pattern:     global.PatternOfSnowflake,
			translation: "{0} must be a valid snowflake",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的雪花 Organization",
			},
		},
		{
			tag: "idCard",
			valid: func(fl validator.FieldLevel) bool {

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
			},
			translation: "{0} must be a valid Organization Card",
			translations: map[string]string{
				"zh": "{0}必须是一个有效的身份证号",
			},
		},
	}
}
