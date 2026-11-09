package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	// Error returned when the email is not a valid email.
	ErrInvalidEmail = errors.New("user service: invalid email")

	// Error returned when the password does not follow the password criteria.
	ErrInvalidPassword = errors.New("user service: invalid password")

	// Error returned when the user we are authenticating does not exist.
	ErrUnexistingUser = errors.New("user service: user does not exist")

	// Error returned when the password is incorrect for the user we are authenticating.
	ErrIncorrectPassword = errors.New("user service: incorrect password")

	// Error returned when trying to create a user that is already signed up.
	ErrUserExists = errors.New("user service: user already exists")

	// Error returned when trying to set a language that is not supported.
	ErrInvalidLang = errors.New("user service: lang not valid")

	// Error returned at sign in when the account e-mail has not been confirmed yet.
	ErrEmailNotVerified = errors.New("user service: email not verified")

	// Error returned when a verification link is unknown, already used or expired.
	ErrInvalidVerificationToken = errors.New("user service: invalid verification token")

	// Error returned when the verification e-mail could not be sent.
	ErrVerificationMail = errors.New("user service: verification email not sent")
)

// VerificationTokenTTL is how long a verification link stays valid.
const VerificationTokenTTL = 24 * time.Hour

const (
	LightTheme = "light"
	DarkTheme  = "dark"
)

type (
	DeleteHook func(user *models.User)

	UserServiceRepository interface {
		UserSignup(email, hashedPassword, lang, theme string) (*models.User, error)
		FindUserByEmail(email string) (*models.User, error)
		UserUpdatePassword(email, hashedPassword string) error
		DeleteUser(user *models.User) error
		DisableUser(user *models.User) error
		UserUpdateSettings(user *models.User, lang, theme string) error
	}

	TranslatorService interface {
		LangIsSupported(lang string) bool
	}

	// EmailVerificationRepository stores verification tokens, as hashes only.
	EmailVerificationRepository interface {
		SetEmailVerification(userID int, tokenHash string, expires time.Time) error
		VerifyEmailByHash(tokenHash string) (string, error)
		DeleteStaleUnverifiedUsers(grace time.Duration) (int64, error)
	}

	// MailLimiter caps how many verification e-mails an address receives.
	MailLimiter interface {
		Allow(key string) bool
	}

	emailVerification struct {
		repository EmailVerificationRepository
		mailer     Mailer
		baseURL    string
		limiter    MailLimiter
		now        func() time.Time
	}

	UserService struct {
		repository   UserServiceRepository
		translator   TranslatorService
		deleteHooks  []DeleteHook
		verification *emailVerification
	}
)

// EnableEmailVerification makes new accounts unusable until their owner follows
// the link e-mailed to them. baseURL is the public URL of the instance.
func (s *UserService) EnableEmailVerification(r EmailVerificationRepository, m Mailer, baseURL string, l MailLimiter) {
	s.verification = &emailVerification{
		repository: r,
		mailer:     m,
		baseURL:    strings.TrimRight(baseURL, "/"),
		limiter:    l,
		now:        time.Now,
	}
}

// EmailVerificationEnabled reports whether sign ups must confirm their e-mail.
func (s *UserService) EmailVerificationEnabled() bool {
	return s.verification != nil
}

// SendVerification issues a new verification token for the user and e-mails the
// link. Older links stop working because only the latest hash is stored.
func (s *UserService) SendVerification(u *models.User) error {
	v := s.verification
	if v == nil {
		return nil
	}
	if v.limiter != nil && !v.limiter.Allow(strings.ToLower(u.Email)) {
		return ErrVerificationMail
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)

	if err := v.repository.SetEmailVerification(u.Id, hashToken(token), v.now().Add(VerificationTokenTTL)); err != nil {
		return err
	}

	link := v.baseURL + "/verify?token=" + token
	body := fmt.Sprintf("Hello,\n\nPlease confirm your e-mail address to activate your account:\n\n%s\n\nThis link expires in 24 hours. If you did not create an account, ignore this message and nothing will happen.\n", link)
	if err := v.mailer.Send(u.Email, "Confirm your e-mail address", body); err != nil {
		log.Printf("verification mail to user %d failed: %v", u.Id, err)
		return ErrVerificationMail
	}

	return nil
}

// VerifyEmail activates the account matching the token and returns it.
func (s *UserService) VerifyEmail(token string) (*models.User, error) {
	v := s.verification
	token = strings.TrimSpace(token)
	if v == nil || len(token) != 64 {
		return nil, ErrInvalidVerificationToken
	}
	if _, err := hex.DecodeString(token); err != nil {
		return nil, ErrInvalidVerificationToken
	}

	email, err := v.repository.VerifyEmailByHash(hashToken(token))
	if err != nil {
		return nil, ErrInvalidVerificationToken
	}

	return s.repository.FindUserByEmail(email)
}

// PurgeUnverified deletes accounts whose verification link expired long ago.
func (s *UserService) PurgeUnverified() {
	if s.verification == nil {
		return
	}
	n, err := s.verification.repository.DeleteStaleUnverifiedUsers(VerificationTokenTTL)
	if err != nil {
		log.Printf("purge unverified users: %v", err)
		return
	}
	if n > 0 {
		log.Printf("purged %d unverified users", n)
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewUserService(r UserServiceRepository, t TranslatorService) *UserService {
	return &UserService{
		repository: r,
		translator: t,
	}
}

// SignUp validates the user email and password, if they are both valid creates a password hash
// before storing it. If succesful, it returns the new user, otherwise an error is returned.
func (s *UserService) SignUp(email, password, lang, theme string) (*models.User, error) {
	_, err := s.repository.FindUserByEmail(email)
	if err == nil {
		return nil, ErrUserExists
	}

	if !s.validPassword(password) {
		return nil, ErrInvalidPassword
	}

	_, err = mail.ParseAddress(email)
	if err != nil {
		return nil, ErrInvalidEmail
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	theme = s.getValidatedTheme(theme)

	u, err := s.repository.UserSignup(email, string(hashedPassword), lang, theme)
	if err != nil {
		return nil, err
	}

	if s.verification == nil {
		u.EmailVerified = true
		return u, nil
	}

	// The account exists from here on but stays unusable until verified. If the
	// e-mail cannot be sent the account is removed so the address can sign up again.
	u.EmailVerified = false
	if err := s.SendVerification(u); err != nil {
		s.repository.DeleteUser(u)
		return nil, err
	}

	return u, nil
}

// SignIn validates the provided email and password combination for user authentication.
// It compares the provided password with the user's hashed password.
// If the passwords do not match, it returns an error.
func (s *UserService) SignIn(email, password string) (*models.User, error) {
	u, err := s.repository.FindUserByEmail(email)
	if err != nil {
		return nil, ErrUnexistingUser
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrIncorrectPassword
	}

	// Only the owner of the password gets a fresh link, and the mail limiter caps it.
	if s.verification != nil && !u.EmailVerified {
		s.SendVerification(u)
		return nil, ErrEmailNotVerified
	}

	return u, nil
}

// UpdatePassword updates the password for the user with the given email.
// It validates the new password and generates a hashed password using bcrypt before storing it.
func (s *UserService) UpdatePassword(user *models.User, currentPassword, newPassword string) error {
	if !s.validPassword(newPassword) {
		return ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return ErrIncorrectPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = s.repository.UserUpdatePassword(user.Email, string(hashedPassword))
	if err != nil {
		return err
	}

	return nil
}

// UpdatePassword updates the password for the user with the given email.
// It validates the new password and generates a hashed password using bcrypt before storing it.
func (s *UserService) UpdateUserSettings(user *models.User, lang, theme string) error {
	if !s.translator.LangIsSupported(lang) {
		return ErrInvalidLang
	}

	theme = s.getValidatedTheme(theme)

	err := s.repository.UserUpdateSettings(user, lang, theme)
	if err != nil {
		return err
	}

	return nil
}

// Delete a User and all its associated projects and crawl data.
// Deleting the user data may take a while, and it's deleted in a
// go routine. To avoid blocking the execution the user is first disabled,
// and once the data has been deleted, the user is finally deleted.
func (s *UserService) DeleteUser(user *models.User) {
	s.repository.DisableUser(user)
	go func() {
		for _, h := range s.deleteHooks {
			h(user)
		}

		s.repository.DeleteUser(user)
	}()
}

// AddDeleteHook adds a new hook function that will be called when the user is deleted.
// This is used for user data clean up.
func (s *UserService) AddDeleteHook(hook DeleteHook) {
	s.deleteHooks = append(s.deleteHooks, hook)
}

// Validate the password to make sure it follows certain criteria.
func (s *UserService) validPassword(password string) bool {
	return len(password) > 1
}

// getValidatedTheme returns a valid theme. If the user theme is "dark" it
// returns "dark" otherwise return "light".
func (s *UserService) getValidatedTheme(theme string) string {
	if theme == DarkTheme {
		return DarkTheme
	}

	return LightTheme
}
