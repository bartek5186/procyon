package models

import "gorm.io/gorm"

type HelloMessage struct {
	gorm.Model
	Slug    string `gorm:"size:64;not null;uniqueIndex:ux_hello_slug_lang,priority:1"`
	Lang    string `gorm:"size:16;not null;uniqueIndex:ux_hello_slug_lang,priority:2;index"`
	Message string `gorm:"size:255;not null"`
}
