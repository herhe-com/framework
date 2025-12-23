package validation

import (
	"fmt"
	"reflect"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/ar"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/es"
	"github.com/go-playground/locales/fa"
	"github.com/go-playground/locales/fr"
	"github.com/go-playground/locales/id"
	"github.com/go-playground/locales/it"
	"github.com/go-playground/locales/ja"
	"github.com/go-playground/locales/lv"
	"github.com/go-playground/locales/nl"
	"github.com/go-playground/locales/pt"
	"github.com/go-playground/locales/pt_BR"
	"github.com/go-playground/locales/ru"
	"github.com/go-playground/locales/tr"
	"github.com/go-playground/locales/vi"
	"github.com/go-playground/locales/zh"
	"github.com/go-playground/locales/zh_Hant"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	arTranslation "github.com/go-playground/validator/v10/translations/ar"
	enTranslation "github.com/go-playground/validator/v10/translations/en"
	esTranslation "github.com/go-playground/validator/v10/translations/es"
	faTranslation "github.com/go-playground/validator/v10/translations/fa"
	frTranslation "github.com/go-playground/validator/v10/translations/fr"
	idTranslation "github.com/go-playground/validator/v10/translations/id"
	itTranslation "github.com/go-playground/validator/v10/translations/it"
	jaTranslation "github.com/go-playground/validator/v10/translations/ja"
	lvTranslation "github.com/go-playground/validator/v10/translations/lv"
	nlTranslation "github.com/go-playground/validator/v10/translations/nl"
	ptTranslation "github.com/go-playground/validator/v10/translations/pt"
	ptBRTranslation "github.com/go-playground/validator/v10/translations/pt_BR"
	ruTranslation "github.com/go-playground/validator/v10/translations/ru"
	trTranslation "github.com/go-playground/validator/v10/translations/tr"
	viTranslation "github.com/go-playground/validator/v10/translations/vi"
	zhTranslation "github.com/go-playground/validator/v10/translations/zh"
	zhTWTranslation "github.com/go-playground/validator/v10/translations/zh_tw"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"github.com/hertz-contrib/binding/go_playground"
)

var (
	trans ut.Translator
)

func NewApplication() {

	valid := go_playground.NewValidator()

	valid.SetValidateTag("validate")

	tran, language := translator()

	uni := ut.New(tran, tran)
	trans, _ = uni.GetTranslator(language)

	////获取 CloudWeGo 的校验器
	vd := valid.Engine().(*validator.Validate)

	vd.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get(facades.Cfg.GetString("validation.label", "label"))
	})

	if err := register(vd, trans, language); err != nil {
		color.Warnf("validator register error: %v", err)
	}

	//注册翻译器
	translations(vd, trans, language)

	translation(vd, trans, language)

	facades.Validator = valid
}

func translator() (translator locales.Translator, language string) {

	language = facades.Cfg.GetString("app.language")

	switch language {
	case "ar":
		translator = ar.New()
	case "es":
		translator = es.New()
	case "fa":
		translator = fa.New()
	case "fr":
		translator = fr.New()
	case "id":
		translator = id.New()
	case "it":
		translator = it.New()
	case "ja":
		translator = ja.New()
	case "lv":
		translator = lv.New()
	case "nl":
		translator = nl.New()
	case "pt":
		translator = pt.New()
	case "pt_BR":
		translator = pt_BR.New()
	case "ru":
		translator = ru.New()
	case "tr":
		translator = tr.New()
	case "vi":
		translator = vi.New()
	case "en":
		translator = en.New()
	case "zh_tw":
		translator = zh_Hant.New()
	default:
		language = "zh"
		translator = zh.New()
	}

	return translator, language
}

func translations(vd *validator.Validate, trans ut.Translator, language string) {

	switch language {
	case "ar":
		_ = arTranslation.RegisterDefaultTranslations(vd, trans)
	case "es":
		_ = esTranslation.RegisterDefaultTranslations(vd, trans)
	case "fa":
		_ = faTranslation.RegisterDefaultTranslations(vd, trans)
	case "fr":
		_ = frTranslation.RegisterDefaultTranslations(vd, trans)
	case "id":
		_ = idTranslation.RegisterDefaultTranslations(vd, trans)
	case "it":
		_ = itTranslation.RegisterDefaultTranslations(vd, trans)
	case "ja":
		_ = jaTranslation.RegisterDefaultTranslations(vd, trans)
	case "lv":
		_ = lvTranslation.RegisterDefaultTranslations(vd, trans)
	case "nl":
		_ = nlTranslation.RegisterDefaultTranslations(vd, trans)
	case "pt":
		_ = ptTranslation.RegisterDefaultTranslations(vd, trans)
	case "pt_BR":
		_ = ptBRTranslation.RegisterDefaultTranslations(vd, trans)
	case "ru":
		_ = ruTranslation.RegisterDefaultTranslations(vd, trans)
	case "tr":
		_ = trTranslation.RegisterDefaultTranslations(vd, trans)
	case "vi":
		_ = viTranslation.RegisterDefaultTranslations(vd, trans)
	case "zh":
		_ = zhTranslation.RegisterDefaultTranslations(vd, trans)
	case "zh_tw":
		_ = zhTWTranslation.RegisterDefaultTranslations(vd, trans)
	default:
		_ = enTranslation.RegisterDefaultTranslations(vd, trans)
	}
}

func translation(vd *validator.Validate, trans ut.Translator, language string) {

	myTranslations := facades.Cfg.GetMaps(fmt.Sprintf("validation.translation.%s", language))

	for key, val := range myTranslations {

		_ = vd.RegisterTranslation(key, trans, func(ut ut.Translator) error {
			return ut.Add(key, fmt.Sprintf("%v", val), true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			t, _ := ut.T(key, fe.Field())
			return t
		})
	}
}
