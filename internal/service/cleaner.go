package service

import (
	"context"
	"strings"
	"time"

	"mailcleanerpro/pkg/gmail"
	"mailcleanerpro/pkg/logger"

	"go.uber.org/zap"
	gmailv1 "google.golang.org/api/gmail/v1"
)

// IsAuthError checks if an error is related to authentication or authorization
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	return strings.Contains(errorStr, "ACCESS_TOKEN_SCOPE_INSUFFICIENT") ||
		strings.Contains(errorStr, "insufficientPermissions") ||
		strings.Contains(errorStr, "Error 401") ||
		strings.Contains(errorStr, "Error 403")
}

type CleanerService struct {
	gmail *gmail.Service
}

type CleanSummary struct {
	PerCategoryDeleted map[string]int `json:"per_category_deleted"`
	TotalDeleted       int            `json:"total_deleted"`
	Completed          bool           `json:"completed"`
	Reason             string         `json:"reason"`
}

func NewCleanerService(g *gmail.Service) *CleanerService {
	return &CleanerService{gmail: g}
}

// CleanCategories identifies and removes emails in specified categories.
// categories should be Gmail label IDs: [CATEGORY_SOCIAL, CATEGORY_FORUMS, CATEGORY_PROMOTIONS, CATEGORY_UPDATES, TRASH]
// Regular categories are moved to trash, TRASH category emails are permanently deleted
func (s *CleanerService) CleanCategories(ctx context.Context, userID string, categories []string, maxPerCat int64) (*CleanSummary, error) {
	// Log operation start
	start := time.Now()
	logger.L().Info("Starting email cleanup operation",
		zap.String("user_id", userID),
		zap.Strings("categories", categories),
		zap.Int64("max_per_category", maxPerCat),
	)

	result := make(map[string]int)
	total := 0
	completed := true
	reason := "all categories processed"

	for _, label := range categories {
		// Log category processing start
		categoryStart := time.Now()
		logger.L().Debug("Processing category",
			zap.String("category", label),
			zap.String("user_id", userID),
		)

		var threads []*gmailv1.Thread
		var err error

		// Handle TRASH category differently
		if label == "TRASH" {
			threads, err = s.gmail.ListTrashThreads(ctx, userID, maxPerCat)
		} else {
			threads, err = s.gmail.ListCategoryThreads(ctx, userID, label, maxPerCat)
		}

		// Log query result
		logger.L().Debug("Category query completed",
			zap.String("category", label),
			zap.Int("threads_found", len(threads)),
			zap.Duration("query_duration", time.Since(categoryStart)),
			zap.Error(err),
		)
		if err != nil {
			logger.L().Error("Failed to query category threads",
				zap.String("category", label),
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
		ids := make([]string, 0, len(threads))
		for _, t := range threads {
			ids = append(ids, t.Id)
		}

		// For TRASH category, delete permanently; for other categories, move to trash
		if label == "TRASH" {
			err = s.gmail.BatchDeleteThreadsPermanently(ctx, userID, ids)
		} else {
			err = s.gmail.BatchTrashThreads(ctx, userID, ids)
		}
		if err != nil {
			logger.L().Error("Failed to delete/trash threads",
				zap.String("category", label),
				zap.String("user_id", userID),
				zap.Int("thread_count", len(ids)),
				zap.Bool("permanent_delete", label == "TRASH"),
				zap.Error(err),
			)
			return nil, err
		}
		deleted := len(ids)

		// Log successful deletion
		logger.L().Info("Successfully processed threads",
			zap.String("category", label),
			zap.Int("deleted_count", deleted),
			zap.Bool("permanent_delete", label == "TRASH"),
		)
		result[label] = deleted
		total += deleted

		// Determine if we reached the per-category max threshold or there are no more emails
		if int64(deleted) >= maxPerCat {
			completed = false
			reason = "max per category reached; more emails may remain"
		} else {
			var estimate int64
			if label == "TRASH" {
				estimate, err = s.gmail.EstimateTrashThreads(ctx, userID)
			} else {
				estimate, err = s.gmail.EstimateCategoryThreads(ctx, userID, label)
			}
			if err == nil && estimate > 0 {
				// There are still emails, so overall not fully completed
				completed = false
				reason = "remaining emails detected in one or more categories"
			}
		}

		// Log API quota management
		logger.L().Debug("API quota pause",
			zap.String("category", label),
			zap.Int("deleted_count", deleted),
			zap.Duration("pause_duration", 200*time.Millisecond),
		)

		// small pause to respect API quotas
		time.Sleep(200 * time.Millisecond)
	}

	// Log operation completion
	duration := time.Since(start)
	logger.L().Info("Email cleanup operation completed",
		zap.String("user_id", userID),
		zap.Int("total_deleted", total),
		zap.Bool("completed", completed),
		zap.String("reason", reason),
		zap.Duration("total_duration", duration),
		zap.Any("per_category_results", result),
		zap.Int("categories_processed", len(categories)),
	)

	return &CleanSummary{
		PerCategoryDeleted: result,
		TotalDeleted:       total,
		Completed:          completed,
		Reason:             reason,
	}, nil
}
