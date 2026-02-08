package tasks

import (
	"context"
	"log"

	"gorm.io/gorm"
)

// LogInfoHandler handles logging information
func LogInfoHandler(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	message, ok := args["message"].(string)
	if !ok {
		message = "No message provided"
	}
	log.Printf("[Task: log_info] Message: %s", message)

	maxAttempt, _ := args["max_attempt"].(int) // retrieve max_limit just in case

	return map[string]interface{}{
		"status":            "success",
		"message":           message,
		"max_attempts_info": maxAttempt,
	}, nil
}
