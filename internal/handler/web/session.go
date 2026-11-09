package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"libratrack/internal/auth"
	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// credentialsForm — дані сторінок входу та реєстрації. Пароль назад у форму
// НЕ підставляється.
type credentialsForm struct {
	Email    string
	FullName string
	Error    string
}

// LoginPage відповідає на GET /login.
func (h *Handler) LoginPage(c *gin.Context) {
	if c.GetString(middleware.CtxRole) != "" {
		c.Redirect(http.StatusSeeOther, "/catalog")
		return
	}
	h.view.Page(c, http.StatusOK, "login.html", h.page(c, "Вхід", credentialsForm{}))
}

// Login відповідає на POST /login.
//
// JWT не повертається в тілі відповіді і не потрапляє в JavaScript: сервер
// кладе його в cookie HttpOnly; SameSite=Strict, а браузер сам додає її до
// наступних запитів. Невідомий email і неправильний пароль дають однакову
// відповідь (захист від перебору акаунтів), як і в JSON API.
func (h *Handler) Login(c *gin.Context) {
	if !h.limitForm(c) {
		return
	}
	form := credentialsForm{Email: strings.TrimSpace(c.PostForm("email"))}
	email, password := form.Email, c.PostForm("password")
	in := model.LoginRequest{Email: &email, Password: &password}
	if in.Validate() != nil {
		form.Error = "Вкажіть email і пароль"
		h.view.Page(c, http.StatusUnprocessableEntity, "login.html", h.page(c, "Вхід", form))
		return
	}

	user, err := h.repos.Users.FindByEmail(c.Request.Context(), in.NormalizedEmail())
	switch {
	case errors.Is(err, repository.ErrNotFound):
		auth.CheckDummyPassword(password) // вирівнювання часу відповіді
	case err != nil:
		h.serverError(c, err)
		return
	}
	if user == nil || !auth.CheckPassword(user.PasswordHash, password) {
		form.Error = "Неправильний email або пароль"
		h.view.Page(c, http.StatusUnauthorized, "login.html", h.page(c, "Вхід", form))
		return
	}

	h.startSession(c, user, "/catalog?msg=logged_in")
}

// SignUpPage відповідає на GET /register.
func (h *Handler) SignUpPage(c *gin.Context) {
	h.view.Page(c, http.StatusOK, "register.html", h.page(c, "Реєстрація", credentialsForm{}))
}

// SignUp відповідає на POST /register. Роль нового користувача завжди reader:
// форма не має поля role, а RegisterRequest.ToUser ігнорує будь-яку іншу.
func (h *Handler) SignUp(c *gin.Context) {
	if !h.limitForm(c) {
		return
	}
	email, password, fullName := c.PostForm("email"), c.PostForm("password"), c.PostForm("full_name")
	form := credentialsForm{Email: strings.TrimSpace(email), FullName: strings.TrimSpace(fullName)}
	in := model.RegisterRequest{Email: &email, Password: &password, FullName: &fullName}
	if err := in.Validate(); err != nil {
		form.Error = validationMessage(err)
		h.view.Page(c, http.StatusUnprocessableEntity, "register.html", h.page(c, "Реєстрація", form))
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		h.serverError(c, err)
		return
	}
	user := in.ToUser(hash)
	if err := h.repos.Users.Create(c.Request.Context(), &user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			form.Error = "Цей email уже зареєстровано"
			h.view.Page(c, http.StatusUnprocessableEntity, "register.html", h.page(c, "Реєстрація", form))
			return
		}
		h.serverError(c, err)
		return
	}
	h.startSession(c, &user, "/catalog?msg=registered")
}

// Logout відповідає на POST /logout (POST, а не GET: вихід змінює стан, і
// сторонній <img src="/logout"> не повинен розлогінювати користувача).
func (h *Handler) Logout(c *gin.Context) {
	middleware.ClearAuthCookie(c, h.secureCookie)
	c.Redirect(http.StatusSeeOther, "/catalog?msg=logged_out")
}

// startSession видає JWT, кладе його в HttpOnly-cookie і переадресовує на next.
func (h *Handler) startSession(c *gin.Context, user *model.User, next string) {
	token, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		h.serverError(c, err)
		return
	}
	middleware.SetAuthCookie(c, token, h.secureCookie)
	c.Redirect(http.StatusSeeOther, next)
}
