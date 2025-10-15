package handler

import (
	"context"
	"mail-stress-test/models"
)

// MailHandler defines the interface for mail operations
type MailHandler interface {
	// CreateMail creates a new mail based on the request
	CreateMail(ctx context.Context, req *models.MailRequest) error

	// ListMails retrieves mails for a user
	ListMails(ctx context.Context, req *models.ListMailsRequest) ([]*models.Mail, error)

	// SearchMails searches for mails matching the criteria
	SearchMails(ctx context.Context, req *models.SearchMailsRequest) ([]*models.Mail, error)
}
