package handler

import (
	"net/http"

	"mailcleanerpro/internal/service"

	"github.com/gin-gonic/gin"
)

type CleanHandler struct {
	cleaner *service.CleanerService
}

func NewCleanHandler(s *service.CleanerService) *CleanHandler {
	return &CleanHandler{cleaner: s}
}

type CleanRequest struct {
	MaxPerCategory int64    `json:"max_per_category" binding:"gte=0,lte=1000000"`
	Categories     []string `json:"categories" binding:"required,min=1,dive,oneof=CATEGORY_SOCIAL CATEGORY_FORUMS CATEGORY_PROMOTIONS CATEGORY_UPDATES TRASH"`
}

type CleanResponse struct {
	Deleted          map[string]int `json:"deleted"`
	TotalDeleted     int            `json:"total_deleted"`
	Completed        bool           `json:"completed"`
	CompletionReason string         `json:"completion_reason"`
}

func (h *CleanHandler) Clean(c *gin.Context) {
	var req CleanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.MaxPerCategory == 0 {
		req.MaxPerCategory = 1000000
	}

	summary, err := h.cleaner.CleanCategories(c, "me", req.Categories, req.MaxPerCategory)
	if err != nil {
		if service.IsAuthError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication failed or insufficient permissions",
				"suggestion": "Please re-authenticate with the required Gmail scopes",
				"details": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := &CleanResponse{
		Deleted:          summary.PerCategoryDeleted,
		TotalDeleted:     summary.TotalDeleted,
		Completed:        summary.Completed,
		CompletionReason: summary.Reason,
	}

	c.JSON(http.StatusOK, resp)
}
