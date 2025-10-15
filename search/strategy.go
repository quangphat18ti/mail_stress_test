package search

import (
	"context"

	"mail-stress-test/database"
	"mail-stress-test/models"
)

// SearchStrategy defines the interface for different search implementations
type SearchStrategy interface {
	// GetName returns the strategy name for reporting
	GetName() string

	// SetupDatabase prepares database indexes and configuration for this strategy
	SetupDatabase(ctx context.Context, db *database.MongoDB) error

	// SearchMails performs search using this strategy
	SearchMails(ctx context.Context, db *database.MongoDB, req *models.SearchMailsRequest) ([]*models.Mail, error)

	// GetDescription returns a description of how this strategy works
	GetDescription() string
}
