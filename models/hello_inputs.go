package models

type HelloInput struct {
	Name string `json:"name" query:"name" validate:"omitempty,max=60"`
}
