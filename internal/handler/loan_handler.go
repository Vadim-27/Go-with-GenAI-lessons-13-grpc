package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/middleware"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

// LoanHandler обслуговує позики: POST /loans, POST /loans/:id/return, GET /me/loans.
type LoanHandler struct {
	repo repository.LoanRepository
}

// NewLoanHandler повертає хендлер на основі сховища позик.
func NewLoanHandler(repo repository.LoanRepository) *LoanHandler {
	return &LoanHandler{repo: repo}
}

// RegisterProtected підключає маршрути для будь-якого автентифікованого користувача.
func (h *LoanHandler) RegisterProtected(r *gin.RouterGroup) {
	r.GET("/me/loans", h.MyLoans)
}

// RegisterStaff підключає маршрути видачі та повернення (librarian / admin).
func (h *LoanHandler) RegisterStaff(r *gin.RouterGroup) {
	r.POST("/loans", h.IssueLoan)
	r.POST("/loans/:id/return", h.ReturnLoan)
}

// currentUserID повертає id користувача з контексту (його поклав AuthMiddleware).
// Якщо id немає — це помилка конфігурації роутів (маршрут поза protected-групою):
// відповідаємо 401, а не пускаємо запит з user_id = 0.
func currentUserID(c *gin.Context) (uint, bool) {
	id, ok := c.Get(middleware.CtxUserID)
	if uid, isUint := id.(uint); ok && isUint && uid != 0 {
		return uid, true
	}
	respondError(c, http.StatusUnauthorized, model.CodeUnauthenticated, "authentication required")
	return 0, false
}

// IssueLoan відповідає на POST /loans.
func (h *LoanHandler) IssueLoan(c *gin.Context) {
	staffID, ok := currentUserID(c)
	if !ok {
		return
	}
	var in model.LoanInput
	if !bindInput(c, &in) {
		return
	}

	loan, err := h.repo.Issue(c.Request.Context(), *in.UserID, *in.CopyID, staffID, in.DaysOrDefault())
	if err != nil {
		respondRepoError(c, err, "Copy")
		return
	}
	c.JSON(http.StatusCreated, loan)
}

// ReturnLoan відповідає на POST /loans/{id}/return.
func (h *LoanHandler) ReturnLoan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	loan, err := h.repo.Return(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err, "Loan")
		return
	}
	c.JSON(http.StatusOK, loan)
}

// MyLoans відповідає на GET /me/loans. Повертає лише позики поточного користувача:
// id береться з токена, а не з query-параметра (захист від IDOR).
func (h *LoanHandler) MyLoans(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	page, limit, ok := pagination(c)
	if !ok {
		return
	}
	loans, err := h.repo.ListByUser(c.Request.Context(), userID, page, limit)
	if err != nil {
		respondRepoError(c, err, "Loan")
		return
	}
	c.JSON(http.StatusOK, loans)
}
