package generator

import (
	"fmt"
	"math/rand"

	"mail-stress-test/models"
)

// DataGenerator generates random mail requests for stress testing
type DataGenerator struct {
	userIDs []string
}

// NewDataGenerator creates a new DataGenerator with a list of user IDs
func NewDataGenerator(userIDs []string) *DataGenerator {
	return &DataGenerator{
		userIDs: userIDs,
	}
}

var Subjects = []string{
	"Meeting Update", "Project Status", "Quick Question",
	"Follow Up", "Important Notice", "Weekly Report",
	"Team Sync", "Budget Review", "Action Required",
}

var contentTemplates = []string{
	"Hi team, I wanted to follow up on our discussion about %s. Please review and provide feedback.",
	"This is regarding the %s project. We need to discuss the next steps.",
	"Can you please take a look at %s? Your input would be valuable.",
	"Update on %s: We've made significant progress this week.",
	"Reminder about %s. Please complete by end of day.",
}

// GenerateCreateMailRequest generates a random CreateMail request
func (g *DataGenerator) GenerateCreateMailRequest(replyToID string) *models.MailRequest {
	from := g.userIDs[rand.Intn(len(g.userIDs))]

	// Generate 1-3 recipients
	numRecipients := rand.Intn(3) + 1
	to := make([]string, 0, numRecipients)
	for i := 0; i < numRecipients; i++ {
		recipient := g.userIDs[rand.Intn(len(g.userIDs))]
		if recipient != from {
			to = append(to, recipient)
		}
	}

	// Sometimes add Cc
	var cc []string
	if rand.Float32() < 0.3 { // 30% chance
		ccRecipient := g.userIDs[rand.Intn(len(g.userIDs))]
		if ccRecipient != from {
			cc = []string{ccRecipient}
		}
	}

	// Rarely add Bcc
	var bcc []string
	if rand.Float32() < 0.1 { // 10% chance
		bccRecipient := g.userIDs[rand.Intn(len(g.userIDs))]
		if bccRecipient != from {
			bcc = []string{bccRecipient}
		}
	}

	subject := Subjects[rand.Intn(len(Subjects))]
	content := fmt.Sprintf(contentTemplates[rand.Intn(len(contentTemplates))], subject)

	return &models.MailRequest{
		From:    from,
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: subject,
		Content: content,
		ReplyTo: replyToID,
	}
}

// GenerateListMailsRequest generates a random ListMails request
func (g *DataGenerator) GenerateListMailsRequest() *models.ListMailsRequest {
	userID := g.userIDs[rand.Intn(len(g.userIDs))]

	return &models.ListMailsRequest{
		UserID: userID,
		Limit:  20 + rand.Intn(80), // 20-100
		Offset: rand.Intn(100),
	}
}

// GenerateSearchMailsRequest generates a random SearchMails request
func (g *DataGenerator) GenerateSearchMailsRequest() *models.SearchMailsRequest {
	userID := g.userIDs[rand.Intn(len(g.userIDs))]
	searchTerm := Subjects[rand.Intn(len(Subjects))]

	return &models.SearchMailsRequest{
		UserID:     userID,
		SearchTerm: searchTerm,
		Limit:      50,
	}
}

// GetRandomUserID returns a random user ID from the generator's list
func (g *DataGenerator) GetRandomUserID() string {
	return g.userIDs[rand.Intn(len(g.userIDs))]
}

// GetUserIDs returns all user IDs
func (g *DataGenerator) GetUserIDs() []string {
	return g.userIDs
}
