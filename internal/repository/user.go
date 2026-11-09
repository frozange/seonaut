package repository

import (
	"database/sql"
	"time"

	"github.com/stjudewashere/seonaut/internal/models"
)

type UserRepository struct {
	DB *sql.DB
}

// UserSignup inserts a new user with the provided email and password into the database.
// It returns the inserted user and an error if the user could not be inserted.
func (ds *UserRepository) UserSignup(email, password, lang, theme string) (*models.User, error) {
	query := `INSERT INTO users (email, password, lang, theme, created) VALUES (?, ?, ?, ?, NOW())`
	stmt, _ := ds.DB.Prepare(query)
	defer stmt.Close()

	_, err := stmt.Exec(email, password, lang, theme)
	if err != nil {
		return nil, err
	}

	// Retrieve and return the inserted user
	return ds.FindUserByEmail(email)
}

// FindUserByEmail returns the user that matches the email address. It also returns
// an error if the user does not exist. The user must not be in 'deleting' state.
func (ds *UserRepository) FindUserByEmail(email string) (*models.User, error) {
	u := &models.User{}
	query := `
		SELECT
			id,
			email,
			password,
			lang,
			theme,
			email_verified
		FROM users
		WHERE email = ? AND deleting = 0`

	row := ds.DB.QueryRow(query, email)
	err := row.Scan(&u.Id, &u.Email, &u.Password, &u.Lang, &u.Theme, &u.EmailVerified)
	if err != nil {
		return u, err
	}

	return u, nil
}

// UserUpdatePassword sets a new password for the user matching the email address.
func (ds *UserRepository) UserUpdatePassword(email, hashedPassword string) error {
	query := `
		UPDATE users
		SET password = ?
		WHERE email = ?
	`

	_, err := ds.DB.Exec(query, hashedPassword, email)

	return err
}

// UserUpdateLang updates the user's language and theme settings.
func (ds *UserRepository) UserUpdateSettings(user *models.User, lang, theme string) error {
	query := `
		UPDATE users
		SET lang = ?, THEME = ?
		WHERE id = ?
	`

	_, err := ds.DB.Exec(query, lang, theme, user.Id)

	return err
}

// DisableUser sets the user in 'deleting' state, which makes it inactive.
func (ds *UserRepository) DisableUser(user *models.User) error {
	query := `UPDATE users SET deleting=1 WHERE id = ?`
	_, err := ds.DB.Exec(query, user.Id)

	return err
}

// DeleteUser removes the user from the database.
func (ds *UserRepository) DeleteUser(user *models.User) error {
	query := "DELETE FROM users WHERE id = ?"
	_, err := ds.DB.Exec(query, user.Id)

	return err
}

// SetEmailVerification marks the user as unverified and stores the hash of the
// verification token with its expiry. The token itself is never stored.
func (ds *UserRepository) SetEmailVerification(userID int, tokenHash string, expires time.Time) error {
	query := `
		UPDATE users
		SET email_verified = 0, verification_hash = ?, verification_expires = ?
		WHERE id = ? AND deleting = 0
	`
	_, err := ds.DB.Exec(query, tokenHash, expires.UTC(), userID)

	return err
}

// VerifyEmailByHash marks as verified the user holding a valid, unexpired token
// hash and returns its email. It returns sql.ErrNoRows when no user matches.
func (ds *UserRepository) VerifyEmailByHash(tokenHash string) (string, error) {
	var (
		id    int
		email string
	)
	query := `
		SELECT id, email
		FROM users
		WHERE verification_hash = ? AND verification_expires > UTC_TIMESTAMP() AND deleting = 0`
	if err := ds.DB.QueryRow(query, tokenHash).Scan(&id, &email); err != nil {
		return "", err
	}

	res, err := ds.DB.Exec(`
		UPDATE users
		SET email_verified = 1, verification_hash = NULL, verification_expires = NULL
		WHERE id = ? AND verification_hash = ?`, id, tokenHash)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return "", sql.ErrNoRows
	}

	return email, nil
}

// DeleteStaleUnverifiedUsers removes accounts whose verification link expired more
// than grace ago. They were never usable, so nothing else references them.
func (ds *UserRepository) DeleteStaleUnverifiedUsers(grace time.Duration) (int64, error) {
	res, err := ds.DB.Exec(`
		DELETE FROM users
		WHERE email_verified = 0 AND verification_expires < ?`, time.Now().UTC().Add(-grace))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
