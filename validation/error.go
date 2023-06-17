package validation

import "github.com/go-playground/validator/v10"

func Errors(err validator.ValidationErrors) (messages map[string][]string) {

	for _, item := range err {

		if _, ok := messages[item.Field()]; !ok {
			messages[item.Field()] = make([]string, 0)
		}

		messages[item.Field()] = append(messages[item.Field()], item.Translate(trans))
	}

	return messages
}

func Error(err validator.ValidationErrors) (message string) {

	for _, item := range err {

		message = item.Translate(trans)

		break
	}

	return message
}
