package routes

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/stjudewashere/seonaut/internal/services"
)

// credentialsView is the data of the sign up, sign in and verification pages.
// ErrorMessage and InfoMessage hold translation keys.
type credentialsView struct {
	Email        string
	Error        bool
	ErrorMessage string
	InfoMessage  string
}

func (h *userHandler) renderCredentials(w http.ResponseWriter, tpl, title string, status int, data *credentialsView) {
	pageView := &PageView{
		Lang:      h.Container.Config.UIConfig.Language,
		Theme:     h.Container.Config.UIConfig.Theme,
		PageTitle: title,
		Data:      data,
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	h.Renderer.RenderTemplate(w, tpl, pageView, pageView.Lang)
}

// signupGetHandler displays the sign up form.
func (h *userHandler) signupGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderCredentials(w, "signup", "SIGNUP_VIEW_PAGE_TITLE", http.StatusOK, &credentialsView{})
}

// signupPostHandler creates an account. The request goes through the per-IP limit
// and Turnstile before any work is done. When e-mail verification is enabled the
// account stays inactive and the visitor is told to check their inbox; otherwise
// they are signed in right away, as upstream does.
func (h *userHandler) signupPostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	ip := h.ClientIP.IP(r)

	fail := func(status int, key string) {
		h.renderCredentials(w, "signup", "SIGNUP_VIEW_PAGE_TITLE", status, &credentialsView{
			Email: email, Error: true, ErrorMessage: key,
		})
	}

	if !h.SignupLimiter.Allow(ip) {
		fail(http.StatusTooManyRequests, "ERROR_TOO_MANY_ATTEMPTS")
		return
	}

	if err := h.Turnstile.Verify(r.Context(), r.FormValue("cf-turnstile-response"), ip); err != nil {
		fail(http.StatusForbidden, "ERROR_HUMAN_CHECK")
		return
	}

	u, err := h.UserService.SignUp(email, password, h.Config.UIConfig.Language, h.Config.UIConfig.Theme)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPassword):
			fail(http.StatusOK, "ERROR_INVALID_PASSWORD")
		case errors.Is(err, services.ErrInvalidEmail):
			fail(http.StatusOK, "ERROR_INVALID_EMAIL")
		case errors.Is(err, services.ErrVerificationMail):
			fail(http.StatusServiceUnavailable, "ERROR_MAIL_NOT_SENT")
		default:
			if !errors.Is(err, services.ErrUserExists) {
				log.Printf("sign up error: %v", err)
			}
			fail(http.StatusOK, "SIGIN_ERROR")
		}
		return
	}

	if !u.EmailVerified {
		h.renderCredentials(w, "signin", "SIGNIN_VIEW_PAGE_TITLE", http.StatusOK, &credentialsView{
			Email: email, InfoMessage: "VERIFY_EMAIL_SENT",
		})
		return
	}

	h.CookieSession.SetSession(u, w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// signinGetHandler displays the sign in form.
func (h *userHandler) signinGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderCredentials(w, "signin", "SIGNIN_VIEW_PAGE_TITLE", http.StatusOK, &credentialsView{})
}

// signinPostHandler authenticates a user. Attempts are limited per IP and per
// e-mail address, so neither a single host nor a botnet aiming at one account
// can guess passwords at speed.
func (h *userHandler) signinPostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	ip := h.ClientIP.IP(r)

	fail := func(status int, key string) {
		h.renderCredentials(w, "signin", "SIGNIN_VIEW_PAGE_TITLE", status, &credentialsView{
			Email: email, Error: true, ErrorMessage: key,
		})
	}

	if !h.SigninIPLimiter.Allow(ip) || !h.SigninEmailLimiter.Allow(strings.ToLower(email)) {
		fail(http.StatusTooManyRequests, "ERROR_TOO_MANY_ATTEMPTS")
		return
	}

	if err := h.Turnstile.Verify(r.Context(), r.FormValue("cf-turnstile-response"), ip); err != nil {
		fail(http.StatusForbidden, "ERROR_HUMAN_CHECK")
		return
	}

	u, err := h.UserService.SignIn(email, password)
	if err != nil {
		if errors.Is(err, services.ErrEmailNotVerified) {
			h.renderCredentials(w, "signin", "SIGNIN_VIEW_PAGE_TITLE", http.StatusOK, &credentialsView{
				Email: email, InfoMessage: "VERIFY_EMAIL_PENDING",
			})
			return
		}
		fail(http.StatusOK, "SIGIN_ERROR")
		return
	}

	h.CookieSession.SetSession(u, w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// verifyHandler activates an account from the link e-mailed at sign up and signs
// the user in.
func (h *userHandler) verifyHandler(w http.ResponseWriter, r *http.Request) {
	u, err := h.UserService.VerifyEmail(r.URL.Query().Get("token"))
	if err != nil {
		h.renderCredentials(w, "signin", "SIGNIN_VIEW_PAGE_TITLE", http.StatusBadRequest, &credentialsView{
			Error: true, ErrorMessage: "VERIFY_EMAIL_INVALID",
		})
		return
	}

	h.CookieSession.SetSession(u, w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
