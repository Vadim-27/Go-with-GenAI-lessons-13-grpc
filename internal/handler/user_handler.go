package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// UserHandler обслуговує профіль (/me) та керування користувачами (/users).
type UserHandler struct {
	repo repository.UserRepository
}

// NewUserHandler повертає хендлер на основі сховища користувачів.
func NewUserHandler(repo repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// RegisterProtected підключає профіль (будь-який автентифікований користувач).
func (h *UserHandler) RegisterProtected(r *gin.RouterGroup) {
	r.GET("/me", h.GetMe)
	r.PUT("/me", h.UpdateMe)
}

// RegisterAdmin підключає керування користувачами (лише admin).
func (h *UserHandler) RegisterAdmin(r *gin.RouterGroup) {
	r.GET("/users", h.ListUsers)
	r.PUT("/users/:id/role", h.UpdateRole)
}

// GetMe відповідає на GET /me.
func (h *UserHandler) GetMe(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}
	user, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err, "User")
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateMe відповідає на PUT /me. Змінюється лише full_name (див. UpdateProfileRequest).
func (h *UserHandler) UpdateMe(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}
	var in model.UpdateProfileRequest
	if !bindInput(c, &in) {
		return
	}
	user, err := h.repo.UpdateFullName(c.Request.Context(), id, in.Value())
	if err != nil {
		respondRepoError(c, err, "User")
		return
	}
	c.JSON(http.StatusOK, user)
}

// ListUsers відповідає на GET /users?page=&limit=. PasswordHash не серіалізується (json:"-").
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, limit, ok := pagination(c)
	if !ok {
		return
	}
	users, err := h.repo.List(c.Request.Context(), page, limit)
	if err != nil {
		respondRepoError(c, err, "User")
		return
	}
	c.JSON(http.StatusOK, users)
}

// UpdateRole відповідає на PUT /users/{id}/role.
func (h *UserHandler) UpdateRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	me, ok := currentUserID(c)
	if !ok {
		return
	}
	// Адмін не може змінити роль собі: інакше можна випадково лишити систему без адміністраторів.
	if id == me {
		respondError(c, http.StatusUnprocessableEntity, model.CodeValidationError, "you cannot change your own role")
		return
	}
	var in model.UpdateRoleRequest
	if !bindInput(c, &in) {
		return
	}
	user, err := h.repo.UpdateRole(c.Request.Context(), id, *in.Role)
	if err != nil {
		respondRepoError(c, err, "User")
		return
	}
	c.JSON(http.StatusOK, user)
}
