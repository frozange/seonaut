package models

type User struct {
	Id       int
	Email    string
	Password string
	Lang     string
	Theme    string

	// EmailVerified is false until the owner follows the link sent at sign up.
	// Accounts created before e-mail verification existed are verified.
	EmailVerified bool
}
