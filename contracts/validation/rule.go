package validation

import "github.com/go-playground/validator/v10"

type Rule struct {
	Tag          string
	Pattern      string
	Valid        func(fl validator.FieldLevel) bool
	Translation  string
	Translations map[string]string
}
