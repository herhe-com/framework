package validation

import (
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTranslation "github.com/go-playground/validator/v10/translations/zh"
	"github.com/herhe-com/framework/facades"
	"reflect"
)

var (
	trans ut.Translator
)

func NewApplication() {

	//注册校验器
	facades.Validator = validator.New()

	//注册翻译器
	chinese := zh.New()
	uni := ut.New(chinese, chinese)

	trans, _ = uni.GetTranslator("zh")

	////获取 CloudWeGo 的校验器
	//valid = binding.Validator.Engine().(*validator.Validate)

	facades.Validator.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("label")
	})

	registerRules()

	registerTranslation()

	//注册翻译器
	_ = zhTranslation.RegisterDefaultTranslations(facades.Validator, trans)
}

func registerRules() {

	_ = facades.Validator.RegisterValidation("idCard", idCard)
	_ = facades.Validator.RegisterValidation("mobile", mobile)
	_ = facades.Validator.RegisterValidation("dirs", dirs)
	_ = facades.Validator.RegisterValidation("username", username)
	_ = facades.Validator.RegisterValidation("password", password)
	_ = facades.Validator.RegisterValidation("snowflake", snowflake)
}

func registerTranslation() {

	_ = facades.Validator.RegisterTranslation("idCard", trans, func(ut ut.Translator) error {
		return ut.Add("idCard", "身份证号格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("idCard")
		return t
	})

	_ = facades.Validator.RegisterTranslation("mobile", trans, func(ut ut.Translator) error {
		return ut.Add("mobile", "手机号格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("mobile")
		return t
	})

	_ = facades.Validator.RegisterTranslation("dirs", trans, func(ut ut.Translator) error {
		return ut.Add("dirs", "文件夹格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("dirs")
		return t
	})

	_ = facades.Validator.RegisterTranslation("username", trans, func(ut ut.Translator) error {
		return ut.Add("username", "请输入 6-32 位的英文字母数字以及 -_ 等字符", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("username")
		return t
	})

	_ = facades.Validator.RegisterTranslation("password", trans, func(ut ut.Translator) error {
		return ut.Add("password", "请输入 6-32 位的英文字母数字以及 -_@$&%! 等特殊字符", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("password")
		return t
	})

	_ = facades.Validator.RegisterTranslation("snowflake", trans, func(ut ut.Translator) error {
		return ut.Add("snowflake", "雪花 ID 格式错误", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("snowflake")
		return t
	})
}
