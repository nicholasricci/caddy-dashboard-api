package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type UserHandler struct {
	svc   *services.UserService
	audit *services.AuditService
}

func NewUserHandler(svc *services.UserService, audit *services.AuditService) *UserHandler {
	return &UserHandler{svc: svc, audit: audit}
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
}

// List godoc
// @Summary List users
// @Tags users
// @Produce json
// @Success 200 {array} models.User
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/users [get]
func (h *UserHandler) List(c *gin.Context) {
	limit, offset := parseLimitOffset(c)
	users, total, err := h.svc.ListPaginated(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}

// Get godoc
// @Summary Get user
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}
	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load user"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// Create godoc
// @Summary Create user
// @Tags users
// @Accept json
// @Produce json
// @Param payload body createUserRequest true "User payload"
// @Success 201 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	u, err := h.svc.Create(c.Request.Context(), services.CreateUserInput{
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		if errors.Is(err, services.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "username already taken"})
			return
		}
		if errors.Is(err, services.ErrInvalidRole) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role"})
			return
		}
		if errors.Is(err, services.ErrUsernameRequired) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "username is required"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create user"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "user", u.ID.String(), gin.H{"username": u.Username, "role": u.Role})
	c.JSON(http.StatusCreated, u)
}

// Update godoc
// @Summary Update user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param payload body updateUserRequest true "User fields"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	u, err := h.svc.Update(c.Request.Context(), id, services.UpdateUserInput{
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		if errors.Is(err, services.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "username already taken"})
			return
		}
		if errors.Is(err, services.ErrInvalidRole) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role"})
			return
		}
		if errors.Is(err, services.ErrLastAdmin) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot demote the last admin"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update user"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "update", "user", u.ID.String(), gin.H{"username": u.Username, "role": u.Role})
	c.JSON(http.StatusOK, u)
}

// Delete godoc
// @Summary Delete user
// @Tags users
// @Param id path string true "User ID"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}
	actor := c.GetString("username")
	if err := h.svc.Delete(c.Request.Context(), id, actor); err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		if errors.Is(err, services.ErrCannotDeleteSelf) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete your own account"})
			return
		}
		if errors.Is(err, services.ErrLastAdmin) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete the last admin"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete user"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), actor, "delete", "user", id.String(), nil)
	c.Status(http.StatusNoContent)
}
