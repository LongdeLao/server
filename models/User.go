package models

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"` // Now the password will be included in JSON responses
	Role     string `json:"role"`
}
