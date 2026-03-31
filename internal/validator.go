package internal

import (
	"net/url"

	"github.com/go-playground/validator/v10"
)

type InputValidator struct {
	v *validator.Validate
}

func NewInputValidator() *InputValidator {
	v := validator.New()

	_ = v.RegisterValidation("httpurl", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if s == "" {
			return true
		}

		u, err := url.Parse(s)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
	})

	return &InputValidator{v: v}
}

func (ev *InputValidator) Validate(i any) error {
	return ev.v.Struct(i)
}
