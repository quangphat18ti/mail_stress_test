package generator

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DataGenerator struct {
	db *database.MongoDB
}

func NewDataGenerator(db *database.MongoDB) *DataGenerator {
	return &DataGenerator{db: db}
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

func (g *DataGenerator) GenerateMail(userID, threadID string) *models.Mail {
	return &models.Mail{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		ThreadID:  threadID,
		Subject:   Subjects[rand.Intn(len(Subjects))],
		Content:   fmt.Sprintf(contentTemplates[rand.Intn(len(contentTemplates))], Subjects[rand.Intn(len(Subjects))]),
		CreatedAt: time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour),
	}
}

func (g *DataGenerator) CreateMailWithThread(ctx context.Context, senderID string, recipients []string) error {
	threadID := primitive.NewObjectID().Hex()

	// Create mail for sender
	senderMail := g.GenerateMail(senderID, threadID)

	mailCollection := g.db.Database.Collection("mails")
	threadCollection := g.db.Database.Collection("threads")

	// Insert sender's mail
	if _, err := mailCollection.InsertOne(ctx, senderMail); err != nil {
		return err
	}

	// Create thread mail metadata
	threadMail := models.ThreadMail{
		From:    senderID,
		MsgID:   senderMail.ID.Hex(),
		Subject: senderMail.Subject,
		Content: senderMail.Content,
		To:      recipients,
		Type:    1, // sent
	}

	// Update sender's thread
	userIDObj, _ := primitive.ObjectIDFromHex(senderID)
	g.updateThread(ctx, threadCollection, userIDObj, threadID, threadMail)

	// Create mails for recipients
	for _, recipientID := range recipients {
		recipientMail := &models.Mail{
			ID:        primitive.NewObjectID(),
			UserID:    recipientID,
			ThreadID:  threadID,
			Subject:   senderMail.Subject,
			Content:   senderMail.Content,
			CreatedAt: senderMail.CreatedAt,
		}

		if _, err := mailCollection.InsertOne(ctx, recipientMail); err != nil {
			return err
		}

		// Update recipient's thread
		recipientThreadMail := threadMail
		recipientThreadMail.Type = 0 // received

		userIDObj, _ := primitive.ObjectIDFromHex(recipientID)
		g.updateThread(ctx, threadCollection, userIDObj, threadID, recipientThreadMail)
	}

	return nil
}

func (g *DataGenerator) updateThread(ctx context.Context, collection *mongo.Collection, userID primitive.ObjectID, threadID string, threadMail models.ThreadMail) error {
	filter := map[string]interface{}{
		"user_id":   userID,
		"thread_id": threadID,
	}

	update := map[string]interface{}{
		"$push": map[string]interface{}{
			"mails": threadMail,
		},
		"$inc": map[string]interface{}{
			"total_mails": 1,
		},
		"$setOnInsert": map[string]interface{}{
			"user_id":   userID,
			"thread_id": threadID,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (g *DataGenerator) SeedData(ctx context.Context, numUsers, mailsPerUser int) error {
	userIDs := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		userIDs[i] = primitive.NewObjectID().Hex()
	}

	for i := 0; i < mailsPerUser; i++ {
		senderIdx := rand.Intn(numUsers)
		numRecipients := rand.Intn(5) + 1

		recipients := make([]string, 0, numRecipients)
		for j := 0; j < numRecipients; j++ {
			recipientIdx := rand.Intn(numUsers)
			if recipientIdx != senderIdx {
				recipients = append(recipients, userIDs[recipientIdx])
			}
		}

		if len(recipients) > 0 {
			if err := g.CreateMailWithThread(ctx, userIDs[senderIdx], recipients); err != nil {
				return err
			}
		}

		if i%100 == 0 {
			fmt.Printf("Created %d/%d mails\n", i, mailsPerUser)
		}
	}

	return nil
}
