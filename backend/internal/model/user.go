package model

type User struct {
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	PasswordHash string `json:"-"`
}

