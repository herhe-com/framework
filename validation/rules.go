package validation

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/herhe-com/framework/contracts/global"
	"github.com/herhe-com/framework/contracts/validation"
	"github.com/herhe-com/framework/facades"
)

func register(vd *validator.Validate, trans ut.Translator, language string) (err error) {

	rule := rules()

	if items, ok := facades.Cfg.Get("validation.rules", nil).([]validation.Rule); ok && len(items) > 0 {
		rule = append(rule, items...)
	}

	for _, item := range rule {

		fn := item.Valid

		if item.Pattern != "" {
			fn = func(fl validator.FieldLevel) bool {
				ok, _ := regexp.MatchString(item.Pattern, fl.Field().String())
				return ok
			}
		}

		if err = vd.RegisterValidation(item.Tag, fn); err != nil {
			return err
		}

		text := item.Translation

		if txt, ok := item.Translations[language]; ok {
			text = txt
		}

		if err = vd.RegisterTranslation(item.Tag, trans,
			func(ut ut.Translator) error {
				return ut.Add(item.Tag, text, true)
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

func rules() []validation.Rule {

	return []validation.Rule{
		{
			Tag:         "captcha",
			Pattern:     global.PatternOfCaptcha,
			Translation: "{0} must be a valid CAPTCHA",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的验证码",
				"ja": "{0}は正しい認証コードでなければならない",
			},
		},
		{
			Tag:         "pinyin",
			Pattern:     global.PatternOfPinyin,
			Translation: "{0} must be a valid pinyin",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的拼音",
			},
		},
		{
			Tag:         "mobile",
			Pattern:     global.PatternOfMobile,
			Translation: "{0} must be a valid mobile phone number",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的手机号",
			},
		},
		{
			Tag:         "dirs",
			Pattern:     global.PatternOfDirs,
			Translation: "{0} must be a valid dir address",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的文件夹",
			},
		},
		{
			Tag:         "username",
			Pattern:     global.PatternOfUsername,
			Translation: "{0} must be a valid username",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的用户名",
			},
		},
		{
			Tag:         "password",
			Pattern:     global.PatternOfPassword,
			Translation: "{0} must be a valid password",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的密码",
			},
		},
		{
			Tag:         "snowflake",
			Pattern:     global.PatternOfSnowflake,
			Translation: "{0} must be a valid snowflake",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的雪花 Organization",
			},
		},
		{
			Tag: "idCard",
			Valid: func(fl validator.FieldLevel) bool {

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
			Translation: "{0} must be a valid Organization Card",
			Translations: map[string]string{
				"zh": "{0}必须是一个有效的身份证号",
			},
		},
	}
}
