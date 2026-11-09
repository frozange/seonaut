package routes

import (
	"log"
	"net/http"

	"github.com/stjudewashere/seonaut/internal/services"
)

type userHandler struct {
	*services.Container
}

// deleteGetHandler handles the HTTP GET request for the delete user account functionality.
// it renders the delete page with the appropriate data.
// A user account can not be deleted if it is crawling a project.
func (h *userHandler) deleteGetHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	pageView := &PageView{
		Lang:      user.Lang,
		Theme:     user.Theme,
		PageTitle: "DELETE_ACCOUNT_VIEW_PAGE_TITLE",
		User:      *user,
		Data: &struct {
			Crawling bool
		}{
			Crawling: h.ProjectViewService.UserIsCrawling(user.Id),
		},
	}

	h.Renderer.RenderTemplate(w, "delete_account", pageView, user.Lang)
}

// deletePostHandler handles the POST request to delete an account.
// After deleting the user and all its associated data it destroys the session
// and redirects to the sign-in page. Users with projects still crawling can not
// be deleted and are redirected to the delete page.
func (h *userHandler) deletePostHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	crawling := h.ProjectViewService.UserIsCrawling(user.Id)
	if crawling {
		http.Redirect(w, r, "/account/delete", http.StatusSeeOther)
		return
	}

	h.UserService.DeleteUser(user)
	h.CookieSession.DestroySession(w, r)
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

// editGetHandler handles the HTTP GET request for the account edit page.
// It renders the account management form with the appropriate data.
func (h *userHandler) editGetHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	pageView := &PageView{
		Lang:      user.Lang,
		Theme:     user.Theme,
		PageTitle: "ACCOUNT_VIEW_PAGE_TITLE",
		User:      *user,
		Data: &struct {
			Error        bool
			ErrorMessage string
		}{},
	}

	h.Renderer.RenderTemplate(w, "account", pageView, user.Lang)
}

// editPostHandler handles the HTTP POST request for the account edit page.
// It allows users to change their credentials by verifying the current password and
// updating it with a new one.
// In case of error the data's Error and ErrorMessage are populated for the template.
func (h *userHandler) editPostHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	formLang := r.FormValue("language")
	formTheme := r.FormValue("theme")

	err = h.UserService.UpdateUserSettings(user, formLang, formTheme)
	if err != nil {
		errorMsg := "GENERIC_ERROR"
		switch err {
		case services.ErrInvalidLang:
			errorMsg = "INVALID_LANG_ERROR"
		default:
			log.Printf("edit post handler user id %d error: %v", user.Id, err)
		}

		pageView := &PageView{
			Lang:      user.Lang,
			Theme:     user.Theme,
			PageTitle: "ACCOUNT_VIEW_PAGE_TITLE",
			User:      *user,
			Data: &struct {
				Error        bool
				ErrorMessage string
			}{
				Error:        true,
				ErrorMessage: errorMsg,
			},
		}

		h.Renderer.RenderTemplate(w, "account", pageView, user.Lang)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// changePasswordGetHandler handles the HTTP GET request for the password change page.
// It renders the password management form with the appropriate data.
func (h *userHandler) changePasswordGetHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	pageView := &PageView{
		Lang:      user.Lang,
		Theme:     user.Theme,
		PageTitle: "ACCOUNT_VIEW_PAGE_TITLE",
		User:      *user,
		Data: &struct {
			Error        bool
			ErrorMessage string
		}{},
	}

	h.Renderer.RenderTemplate(w, "change_password", pageView, user.Lang)
}

// changePasswordPostHandler handles the HTTP POST request for the password change page.
// It allows users to change their credentials by verifying the current password and
// updating it with a new one.
// In case of error the data's Error and ErrorMessage are populated for the template.
func (h *userHandler) changePasswordPostHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	currentPassword := r.FormValue("password")
	newPassword := r.FormValue("new_password")

	err = h.UserService.UpdatePassword(user, currentPassword, newPassword)
	if err != nil {
		errorMsg := "GENERIC_ERROR"
		switch err {
		case services.ErrInvalidPassword:
			errorMsg = "NEW_PASSWORD_IS_INVALID"
		case services.ErrIncorrectPassword:
			errorMsg = "CURRENT_PASSWORD_INCORRECT"
		default:
			log.Printf("update password user id %d error: %v", user.Id, err)
		}

		pageView := &PageView{
			Lang:      user.Lang,
			Theme:     user.Theme,
			PageTitle: "ACCOUNT_VIEW_PAGE_TITLE",
			User:      *user,
			Data: &struct {
				Error        bool
				ErrorMessage string
			}{
				Error:        true,
				ErrorMessage: errorMsg,
			},
		}

		h.Renderer.RenderTemplate(w, "change_password", pageView, user.Lang)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// signoutHandler handles the user's signout request.
// It clears the session data related to authenticated user and redirects to
// the sign-in page.
func (h *userHandler) signoutHandler(w http.ResponseWriter, r *http.Request) {
	h.CookieSession.DestroySession(w, r)
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}
