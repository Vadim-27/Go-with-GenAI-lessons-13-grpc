package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// AuthHandler обслуговує /auth/register та /auth/login.
type AuthHandler struct {
	users repository.UserRepository
}

// NewAuthHandler повертає хендлер на основі сховища користувачів.
func NewAuthHandler(users repository.UserRepository) *AuthHandler {
	return &AuthHandler{users: users}
}

// Register підключає публічні маршрути автентифікації до групи.
func (h *AuthHandler) Register(r *gin.RouterGroup) {
	r.POST("/auth/register", h.RegisterUser)
	r.POST("/auth/login", h.Login)
}

// loginResponse — відповідь на успішний логін. PasswordHash у User має
// json:"-", тому хеш у відповідь не потрапляє.
type loginResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// RegisterUser відповідає на POST /auth/register.
func (h *AuthHandler) RegisterUser(c *gin.Context) {
	var in model.RegisterRequest
	if !bindInput(c, &in) {
		return
	}

	hash, err := auth.HashPassword(*in.Password)
	if err != nil {
		log.Printf("hash password: %v", err)
		respondError(c, http.StatusInternalServerError, model.CodeInternalServerError, "Internal server error")
		return
	}

	user := in.ToUser(hash) // роль завжди reader
	if err := h.users.Create(c.Request.Context(), &user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			respondError(c, http.StatusUnprocessableEntity, model.CodeValidationError, "email already registered")
			return
		}
		respondRepoError(c, err, "User")
		return
	}
	c.JSON(http.StatusCreated, user)
}

// Login відповідає на POST /auth/login.
//
// Невідомий email і неправильний пароль дають ОДНАКОВУ відповідь 401
// "invalid credentials", щоб не можна було перебирати існуючі акаунти.
func (h *AuthHandler) Login(c *gin.Context) {
	var in model.LoginRequest
	if !bindInput(c, &in) {
		return
	}

	user, err := h.users.FindByEmail(c.Request.Context(), in.NormalizedEmail())
	switch {
	case errors.Is(err, repository.ErrNotFound):
		// Фіктивне порівняння: час відповіді не видає, що акаунта немає.
		auth.CheckDummyPassword(*in.Password)
		invalidCredentials(c)
		return
	case err != nil:
		respondRepoError(c, err, "User")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, *in.Password) {
		invalidCredentials(c)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		log.Printf("generate token: %v", err)
		respondError(c, http.StatusInternalServerError, model.CodeInternalServerError, "Internal server error")
		return
	}
	// Відповідь із токеном не можна кешувати ні в браузері, ні в проксі.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, loginResponse{Token: token, User: user})
}

func invalidCredentials(c *gin.Context) {
	respondError(c, http.StatusUnauthorized, model.CodeUnauthenticated, "invalid credentials")
}
