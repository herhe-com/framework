package validation

import (
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTranslation "github.com/go-playground/validator/v10/translations/zh"
	"github.com/herhe-com/framework/facades"
	"github.com/hertz-contrib/binding/go_playground"
	"reflect"
)

var (
	trans ut.Translator
)

func NewApplication() {

	valid := go_playground.NewValidator()

	valid.SetValidateTag("valid")

	//注册翻译器
	chinese := zh.New()
	uni := ut.New(chinese, chinese)

	trans, _ = uni.GetTranslator("zh")

	////获取 CloudWeGo 的校验器
	vd := valid.Engine().(*validator.Validate)

	vd.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("label")
	})

	registerRules(vd)

	registerTranslation(vd)

	//注册翻译器
	_ = zhTranslation.RegisterDefaultTranslations(vd, trans)

	facades.Validator = valid
}

func registerRules(vd *validator.Validate) {

	_ = vd.RegisterValidation("captcha", captcha)
	_ = vd.RegisterValidation("idCard", idCard)
	_ = vd.RegisterValidation("mobile", mobile)
	_ = vd.RegisterValidation("dirs", dirs)
	_ = vd.RegisterValidation("username", username)
	_ = vd.RegisterValidation("password", password)
	_ = vd.RegisterValidation("snowflake", snowflake)
}

func registerTranslation(vd *validator.Validate) {

	_ = vd.RegisterTranslation("idCard", trans, func(ut ut.Translator) error {
		return ut.Add("idCard", "身份证号格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("idCard")
		return t
	})

	_ = vd.RegisterTranslation("captcha", trans, func(ut ut.Translator) error {
		return ut.Add("captcha", "The Captcha format is incorrect", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("captcha")
		return t
	})

	_ = vd.RegisterTranslation("mobile", trans, func(ut ut.Translator) error {
		return ut.Add("mobile", "手机号格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("mobile")
		return t
	})

	_ = vd.RegisterTranslation("dirs", trans, func(ut ut.Translator) error {
		return ut.Add("dirs", "文件夹格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("dirs")
		return t
	})

	_ = vd.RegisterTranslation("username", trans, func(ut ut.Translator) error {
		return ut.Add("username", "请输入 6-32 位的英文字母数字以及 -_ 等字符", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("username")
		return t
	})

	_ = vd.RegisterTranslation("password", trans, func(ut ut.Translator) error {
		return ut.Add("password", "请输入 6-32 位的英文字母数字以及 -_@$&%! 等特殊字符", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("password")
		return t
	})

	_ = vd.RegisterTranslation("snowflake", trans, func(ut ut.Translator) error {
		return ut.Add("snowflake", "雪花 ID 格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("snowflake")
		return t
	})
}
