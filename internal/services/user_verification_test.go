package services

import (
	"database/sql"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stjudewashere/seonaut/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// memUsers is an in-memory user store implementing both repositories.
type memUsers struct {
	users   map[string]*models.User
	hashes  map[int]string
	expires map[int]time.Time
	nextID  int
	deleted []int
}

func newMemUsers() *memUsers {
	return &memUsers{users: map[string]*models.User{}, hashes: map[int]string{}, expires: map[int]time.Time{}, nextID: 1}
}

func (m *memUsers) UserSignup(email, hashed, lang, theme string) (*models.User, error) {
	u := &models.User{Id: m.nextID, Email: email, Password: hashed, Lang: lang, Theme: theme, EmailVerified: true}
	m.nextID++
	m.users[email] = u
	return u, nil
}
func (m *memUsers) FindUserByEmail(email string) (*models.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, ErrUnexistingUser
	}
	c := *u
	return &c, nil
}
func (m *memUsers) UserUpdatePassword(string, string) error { return nil }
func (m *memUsers) DeleteUser(u *models.User) error {
	m.deleted = append(m.deleted, u.Id)
	delete(m.users, u.Email)
	return nil
}
func (m *memUsers) DisableUser(*models.User) error                        { return nil }
func (m *memUsers) UserUpdateSettings(*models.User, string, string) error { return nil }

func (m *memUsers) SetEmailVerification(id int, hash string, exp time.Time) error {
	for _, u := range m.users {
		if u.Id == id {
			u.EmailVerified = false
			m.hashes[id] = hash
			m.expires[id] = exp
			return nil
		}
	}
	return sql.ErrNoRows
}
func (m *memUsers) VerifyEmailByHash(hash string) (string, error) {
	for _, u := range m.users {
		if m.hashes[u.Id] == hash && time.Now().Before(m.expires[u.Id]) {
			u.EmailVerified = true
			delete(m.hashes, u.Id)
			return u.Email, nil
		}
	}
	return "", sql.ErrNoRows
}
func (m *memUsers) DeleteStaleUnverifiedUsers(time.Duration) (int64, error) { return 0, nil }

type memMailer struct {
	to, subject, body string
	sent              int
	fail              bool
}

func (m *memMailer) Send(to, subject, body string) error {
	if m.fail {
		return errors.New("smtp down")
	}
	m.to, m.subject, m.body = to, subject, body
	m.sent++
	return nil
}

type countLimiter struct{ left int }

func (c *countLimiter) Allow(string) bool {
	if c.left <= 0 {
		return false
	}
	c.left--
	return true
}

type okTranslator struct{}

func (okTranslator) LangIsSupported(string) bool { return true }

var tokenRe = regexp.MustCompile(`/verify\?token=([0-9a-f]{64})`)

func setup(t *testing.T, mailer *memMailer, limit int) (*UserService, *memUsers) {
	t.Helper()
	repo := newMemUsers()
	s := NewUserService(repo, okTranslator{})
	s.EnableEmailVerification(repo, mailer, "https://seo.example.com/", &countLimiter{left: limit})
	return s, repo
}

func TestSignupRequiresVerification(t *testing.T) {
	mailer := &memMailer{}
	s, repo := setup(t, mailer, 10)

	u, err := s.SignUp("new@example.com", "password1", "en", "light")
	if err != nil {
		t.Fatal(err)
	}
	if u.EmailVerified {
		t.Fatal("new account must start unverified")
	}
	if mailer.sent != 1 || mailer.to != "new@example.com" {
		t.Fatalf("verification mail not sent: %+v", mailer)
	}

	m := tokenRe.FindStringSubmatch(mailer.body)
	if m == nil {
		t.Fatalf("no link in body: %q", mailer.body)
	}
	if !strings.Contains(mailer.body, "https://seo.example.com/verify?token=") {
		t.Errorf("link must use the public base URL without double slash: %q", mailer.body)
	}
	if repo.hashes[u.Id] == m[1] {
		t.Fatal("raw token stored instead of its hash")
	}

	if _, err := s.SignIn("new@example.com", "password1"); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("unverified sign in: got %v", err)
	}

	// The sign in attempt sent a fresh link; only the latest link is valid.
	if _, err := s.VerifyEmail(m[1]); err == nil {
		t.Fatal("a superseded link must stop working")
	}
	m = tokenRe.FindStringSubmatch(mailer.body)

	v, err := s.VerifyEmail(m[1])
	if err != nil || !v.EmailVerified {
		t.Fatalf("verify: %v %+v", err, v)
	}
	if _, err := s.VerifyEmail(m[1]); err == nil {
		t.Fatal("a token must work only once")
	}
	if _, err := s.SignIn("new@example.com", "password1"); err != nil {
		t.Fatalf("verified sign in: %v", err)
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	s, _ := setup(t, &memMailer{}, 10)
	for _, tok := range []string{"", "abc", strings.Repeat("z", 64), strings.Repeat("a", 64)} {
		if _, err := s.VerifyEmail(tok); !errors.Is(err, ErrInvalidVerificationToken) {
			t.Errorf("token %q accepted", tok)
		}
	}
}

func TestExpiredTokenRefused(t *testing.T) {
	mailer := &memMailer{}
	s, _ := setup(t, mailer, 10)
	s.verification.now = func() time.Time { return time.Now().Add(-25 * time.Hour) }
	if _, err := s.SignUp("old@example.com", "password1", "en", "light"); err != nil {
		t.Fatal(err)
	}
	tok := tokenRe.FindStringSubmatch(mailer.body)[1]
	if _, err := s.VerifyEmail(tok); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestSignupRolledBackWhenMailFails(t *testing.T) {
	s, repo := setup(t, &memMailer{fail: true}, 10)
	if _, err := s.SignUp("x@example.com", "password1", "en", "light"); !errors.Is(err, ErrVerificationMail) {
		t.Fatalf("got %v", err)
	}
	if _, ok := repo.users["x@example.com"]; ok {
		t.Fatal("account kept although the mail was not sent")
	}
}

func TestWrongPasswordSendsNoMail(t *testing.T) {
	mailer := &memMailer{}
	s, _ := setup(t, mailer, 10)
	s.SignUp("y@example.com", "password1", "en", "light")
	before := mailer.sent
	if _, err := s.SignIn("y@example.com", "wrong-password"); !errors.Is(err, ErrIncorrectPassword) {
		t.Fatalf("got %v", err)
	}
	if mailer.sent != before {
		t.Fatal("a wrong password triggered a verification mail")
	}
}

func TestVerificationMailsAreLimited(t *testing.T) {
	mailer := &memMailer{}
	s, _ := setup(t, mailer, 2)
	s.SignUp("z@example.com", "password1", "en", "light")
	for i := 0; i < 5; i++ {
		s.SignIn("z@example.com", "password1")
	}
	if mailer.sent != 2 {
		t.Fatalf("sent %d mails, limit is 2", mailer.sent)
	}
}

func TestVerificationDisabledKeepsUpstreamBehaviour(t *testing.T) {
	repo := newMemUsers()
	s := NewUserService(repo, okTranslator{})
	u, err := s.SignUp("a@example.com", "password1", "en", "light")
	if err != nil || !u.EmailVerified {
		t.Fatalf("got %v %+v", err, u)
	}
	if _, err := s.SignIn("a@example.com", "password1"); err != nil {
		t.Fatal(err)
	}
}

func TestBuildMessageRefusesHeaderInjection(t *testing.T) {
	from, _ := mail.ParseAddress("SEO <seo@example.com>")
	for _, to := range []string{"a@example.com\r\nBcc: evil@example.com", "not an address"} {
		if _, _, err := buildMessage(from, to, "s", "b", time.Now()); err == nil {
			t.Errorf("recipient %q accepted", to)
		}
	}
	msg, rcpt, err := buildMessage(from, "a@example.com", "Confirm", "line1\nline2", time.Now())
	if err != nil || rcpt != "a@example.com" {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "line1\r\nline2") || !strings.Contains(string(msg), "To: <a@example.com>") {
		t.Errorf("bad message %q", msg)
	}
}

// Sanity check: bcrypt hashes produced by SignUp are verifiable.
func TestSignupHashesPassword(t *testing.T) {
	s, repo := setup(t, &memMailer{}, 10)
	s.SignUp("h@example.com", "password1", "en", "light")
	if bcrypt.CompareHashAndPassword([]byte(repo.users["h@example.com"].Password), []byte("password1")) != nil {
		t.Fatal("password not hashed with bcrypt")
	}
}
