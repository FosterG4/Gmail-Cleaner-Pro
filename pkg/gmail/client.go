package gmail

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"mailcleanerpro/pkg/logger"
)

type Service struct {
	api *gmail.Service
}

func NewService(ctx context.Context, httpClient option.ClientOption) (*Service, error) {
	api, err := gmail.NewService(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("init gmail service: %w", err)
	}
	return &Service{api: api}, nil
}

// ListCategoryThreads returns thread IDs for a given category label.
func (s *Service) ListCategoryThreads(ctx context.Context, userID, categoryLabel string, max int64) ([]*gmail.Thread, error) {
	log := logger.L()
	start := time.Now()

	log.Info("Listing category threads with pagination",
		zap.String("user_id", userID),
		zap.String("category_label", categoryLabel),
		zap.Int64("max_results", max),
	)

	var allThreads []*gmail.Thread
	var pageToken string
	pageCount := 0
	totalFetched := int64(0)

	// Gmail API has a hard limit of 500 results per request
	const maxPerPage = 500

	for {
		pageCount++
		pageStart := time.Now()

		// Calculate how many results to request for this page
		remainingNeeded := max - totalFetched
		pageSize := maxPerPage
		if remainingNeeded < maxPerPage {
			pageSize = int(remainingNeeded)
		}

		log.Debug("Fetching page",
			zap.Int("page_number", pageCount),
			zap.Int("page_size", pageSize),
			zap.Int64("total_fetched", totalFetched),
			zap.String("page_token", pageToken),
		)

		call := s.api.Users.Threads.List(userID).LabelIds(categoryLabel).MaxResults(int64(pageSize))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		res, err := call.Context(ctx).Do()
		pageDuration := time.Since(pageStart)

		if err != nil {
			log.Error("Failed to list category threads page",
				zap.String("user_id", userID),
				zap.String("category_label", categoryLabel),
				zap.Int("page_number", pageCount),
				zap.Error(err),
				zap.Duration("page_duration", pageDuration),
			)
			return nil, fmt.Errorf("failed to list category threads for %s (page %d): %w", categoryLabel, pageCount, err)
		}

		pageThreadCount := len(res.Threads)
		allThreads = append(allThreads, res.Threads...)
		totalFetched += int64(pageThreadCount)

		log.Info("Successfully fetched page",
			zap.String("user_id", userID),
			zap.String("category_label", categoryLabel),
			zap.Int("page_number", pageCount),
			zap.Int("page_thread_count", pageThreadCount),
			zap.Int64("total_fetched", totalFetched),
			zap.Int64("result_size_estimate", res.ResultSizeEstimate),
			zap.String("next_page_token", res.NextPageToken),
			zap.Duration("page_duration", pageDuration),
		)

		// Check if we should continue pagination
		pageToken = res.NextPageToken
		if pageToken == "" || totalFetched >= max || pageThreadCount == 0 {
			break
		}

		// Add a small delay between requests to respect rate limits
		time.Sleep(200 * time.Millisecond)
	}

	totalDuration := time.Since(start)
	log.Info("Completed category threads listing with pagination",
		zap.String("user_id", userID),
		zap.String("category_label", categoryLabel),
		zap.Int("total_pages", pageCount),
		zap.Int64("total_threads_fetched", totalFetched),
		zap.Int64("requested_max", max),
		zap.Duration("total_duration", totalDuration),
	)

	return allThreads, nil
}

// EstimateCategoryThreads returns the Gmail API's estimated number of threads for a label.
func (s *Service) EstimateCategoryThreads(ctx context.Context, userID, categoryLabel string) (int64, error) {
	call := s.api.Users.Threads.List(userID).LabelIds(categoryLabel).MaxResults(1)
	res, err := call.Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	return res.ResultSizeEstimate, nil
}

// BatchTrashThreads moves threads to trash.
func (s *Service) BatchTrashThreads(ctx context.Context, userID string, threadIDs []string) error {
	log := logger.L()
	start := time.Now()

	log.Info("Starting batch trash operation",
		zap.String("user_id", userID),
		zap.Int("thread_count", len(threadIDs)),
		zap.Strings("thread_ids", threadIDs),
	)

	if len(threadIDs) == 0 {
		log.Info("No threads to trash, skipping operation")
		return nil
	}

	req := &gmail.BatchModifyMessagesRequest{
		RemoveLabelIds: []string{"INBOX"},
		AddLabelIds:    []string{"TRASH"},
	}

	successCount := 0
	for i, tid := range threadIDs {
		threadStart := time.Now()

		log.Debug("Processing thread for trash",
			zap.String("user_id", userID),
			zap.String("thread_id", tid),
			zap.Int("thread_index", i+1),
			zap.Int("total_threads", len(threadIDs)),
		)

		modifyReq := &gmail.ModifyThreadRequest{
			AddLabelIds:    req.AddLabelIds,
			RemoveLabelIds: req.RemoveLabelIds,
		}

		response, err := s.api.Users.Threads.Modify(userID, tid, modifyReq).Context(ctx).Do()
		threadDuration := time.Since(threadStart)

		if err != nil {
			log.Error("Failed to trash thread",
				zap.String("user_id", userID),
				zap.String("thread_id", tid),
				zap.Error(err),
				zap.Duration("duration", threadDuration),
				zap.Int("successful_before_error", successCount),
			)
			return fmt.Errorf("failed to trash thread %s: %w", tid, err)
		}

		successCount++
		log.Info("Successfully trashed thread",
			zap.String("user_id", userID),
			zap.String("thread_id", tid),
			zap.String("response_id", response.Id),
			zap.Duration("duration", threadDuration),
			zap.Int("progress", successCount),
			zap.Int("total", len(threadIDs)),
		)
	}

	totalDuration := time.Since(start)
	log.Info("Completed batch trash operation",
		zap.String("user_id", userID),
		zap.Int("total_threads", len(threadIDs)),
		zap.Int("successful_count", successCount),
		zap.Duration("total_duration", totalDuration),
		zap.Float64("avg_duration_per_thread_ms", float64(totalDuration.Nanoseconds())/float64(len(threadIDs))/1e6),
	)

	return nil
}

// BatchDeleteThreadsPermanently permanently deletes threads.
func (s *Service) BatchDeleteThreadsPermanently(ctx context.Context, userID string, threadIDs []string) error {
	log := logger.L()
	start := time.Now()

	log.Warn("Starting batch permanent delete operation - IRREVERSIBLE ACTION",
		zap.String("user_id", userID),
		zap.Int("thread_count", len(threadIDs)),
		zap.Strings("thread_ids", threadIDs),
	)

	if len(threadIDs) == 0 {
		log.Info("No threads to permanently delete, skipping operation")
		return nil
	}

	successCount := 0
	for i, tid := range threadIDs {
		threadStart := time.Now()

		log.Debug("Processing thread for permanent deletion",
			zap.String("user_id", userID),
			zap.String("thread_id", tid),
			zap.Int("thread_index", i+1),
			zap.Int("total_threads", len(threadIDs)),
		)

		// Log the request details before making the API call
		log.Info("Sending permanent delete request to Gmail API",
			zap.String("user_id", userID),
			zap.String("thread_id", tid),
			zap.String("api_endpoint", "Users.Threads.Delete"),
			zap.String("operation_type", "PERMANENT_DELETE"),
		)

		err := s.api.Users.Threads.Delete(userID, tid).Context(ctx).Do()
		threadDuration := time.Since(threadStart)

		if err != nil {
			log.Error("Failed to permanently delete thread",
				zap.String("user_id", userID),
				zap.String("thread_id", tid),
				zap.Error(err),
				zap.Duration("duration", threadDuration),
				zap.Int("successful_before_error", successCount),
				zap.String("error_type", "PERMANENT_DELETE_FAILED"),
			)
			return fmt.Errorf("failed to permanently delete thread %s: %w", tid, err)
		}

		successCount++
		log.Warn("Successfully permanently deleted thread - IRREVERSIBLE",
			zap.String("user_id", userID),
			zap.String("thread_id", tid),
			zap.Duration("duration", threadDuration),
			zap.Int("progress", successCount),
			zap.Int("total", len(threadIDs)),
			zap.String("operation_result", "PERMANENTLY_DELETED"),
			zap.Bool("recoverable", false),
		)
	}

	totalDuration := time.Since(start)
	log.Warn("Completed batch permanent delete operation - ALL DELETIONS IRREVERSIBLE",
		zap.String("user_id", userID),
		zap.Int("total_threads", len(threadIDs)),
		zap.Int("successful_count", successCount),
		zap.Duration("total_duration", totalDuration),
		zap.Float64("avg_duration_per_thread_ms", float64(totalDuration.Nanoseconds())/float64(len(threadIDs))/1e6),
		zap.String("operation_summary", "PERMANENT_DELETE_BATCH_COMPLETED"),
	)

	return nil
}

// ListTrashThreads returns thread IDs from the Trash folder.
func (s *Service) ListTrashThreads(ctx context.Context, userID string, max int64) ([]*gmail.Thread, error) {
	log := logger.L()
	start := time.Now()

	log.Info("Listing trash threads with pagination",
		zap.String("user_id", userID),
		zap.Int64("max_results", max),
		zap.String("label_filter", "TRASH"),
	)

	var allThreads []*gmail.Thread
	var pageToken string
	pageCount := 0
	totalFetched := int64(0)

	// Gmail API has a hard limit of 500 results per request
	const maxPerPage = 500

	for {
		pageCount++
		pageStart := time.Now()

		// Calculate how many results to request for this page
		remainingNeeded := max - totalFetched
		pageSize := maxPerPage
		if remainingNeeded < maxPerPage {
			pageSize = int(remainingNeeded)
		}

		log.Debug("Fetching trash page",
			zap.Int("page_number", pageCount),
			zap.Int("page_size", pageSize),
			zap.Int64("total_fetched", totalFetched),
			zap.String("page_token", pageToken),
		)

		call := s.api.Users.Threads.List(userID).LabelIds("TRASH").MaxResults(int64(pageSize))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		res, err := call.Context(ctx).Do()
		pageDuration := time.Since(pageStart)

		if err != nil {
			log.Error("Failed to list trash threads page",
				zap.String("user_id", userID),
				zap.Int("page_number", pageCount),
				zap.Error(err),
				zap.Duration("page_duration", pageDuration),
			)
			return nil, fmt.Errorf("failed to list trash threads (page %d): %w", pageCount, err)
		}

		pageThreadCount := len(res.Threads)
		allThreads = append(allThreads, res.Threads...)
		totalFetched += int64(pageThreadCount)

		log.Info("Successfully fetched trash page",
			zap.String("user_id", userID),
			zap.Int("page_number", pageCount),
			zap.Int("page_thread_count", pageThreadCount),
			zap.Int64("total_fetched", totalFetched),
			zap.Int64("result_size_estimate", res.ResultSizeEstimate),
			zap.String("next_page_token", res.NextPageToken),
			zap.Duration("page_duration", pageDuration),
		)

		// Check if we should continue pagination
		pageToken = res.NextPageToken
		if pageToken == "" || totalFetched >= max || pageThreadCount == 0 {
			break
		}

		// Add a small delay between requests to respect rate limits
		time.Sleep(200 * time.Millisecond)
	}

	totalDuration := time.Since(start)
	log.Info("Completed trash threads listing with pagination",
		zap.String("user_id", userID),
		zap.Int("total_pages", pageCount),
		zap.Int64("total_threads_fetched", totalFetched),
		zap.Int64("requested_max", max),
		zap.Duration("total_duration", totalDuration),
	)

	return allThreads, nil
}

// EstimateTrashThreads returns the Gmail API's estimated number of threads in Trash.
func (s *Service) EstimateTrashThreads(ctx context.Context, userID string) (int64, error) {
	call := s.api.Users.Threads.List(userID).LabelIds("TRASH").MaxResults(1)
	res, err := call.Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	return res.ResultSizeEstimate, nil
}
