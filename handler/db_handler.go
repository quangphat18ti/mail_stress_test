package handler

import (
	"context"
	"time"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DBHandler implements MailHandler with direct database operations
type DBHandler struct {
	db *database.MongoDB
}

// NewDBHandler creates a new DBHandler
func NewDBHandler(db *database.MongoDB) *DBHandler {
	return &DBHandler{db: db}
}

// CreateMail creates a new mail with proper threading logic
func (h *DBHandler) CreateMail(ctx context.Context, req *models.MailRequest) error {
	mailCollection := h.db.Database.Collection("mails")
	threadCollection := h.db.Database.Collection("threads")

	// Determine thread ID
	var threadID string
	if req.ReplyTo != "" {
		// If replying, find the original mail and use its thread
		var originalMail models.Mail
		objID, err := primitive.ObjectIDFromHex(req.ReplyTo)
		if err != nil {
			return err
		}
		err = mailCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&originalMail)
		if err != nil {
			return err
		}
		threadID = originalMail.ThreadID
	} else {
		// New thread
		threadID = primitive.NewObjectID().Hex()
	}

	// Create sender's mail
	senderMail := &models.Mail{
		ID:        primitive.NewObjectID(),
		From:      req.From,
		To:        req.To,
		Cc:        req.Cc,
		Bcc:       req.Bcc,
		Subject:   req.Subject,
		Content:   req.Content,
		Type:      1, // sent
		ReplyTo:   req.ReplyTo,
		ThreadID:  threadID,
		UserID:    req.From,
		CreatedAt: time.Now(),
	}

	// Insert sender's mail
	if _, err := mailCollection.InsertOne(ctx, senderMail); err != nil {
		return err
	}

	// Create thread mail metadata
	threadMail := models.ThreadMail{
		From:    req.From,
		MsgID:   senderMail.ID.Hex(),
		Subject: req.Subject,
		Content: req.Content,
		Cc:      req.Cc,
		To:      req.To,
		Bcc:     req.Bcc,
		Type:    1, // sent
	}

	// Update sender's thread
	senderIDObj, _ := primitive.ObjectIDFromHex(req.From)
	if err := h.updateThread(ctx, threadCollection, senderIDObj, threadID, threadMail); err != nil {
		return err
	}

	// Create mails for all recipients (To, Cc, Bcc)
	allRecipients := make([]string, 0)
	allRecipients = append(allRecipients, req.To...)
	allRecipients = append(allRecipients, req.Cc...)
	allRecipients = append(allRecipients, req.Bcc...)

	for _, recipientID := range allRecipients {
		if recipientID == req.From {
			continue // Skip sender
		}

		recipientMail := &models.Mail{
			ID:        primitive.NewObjectID(),
			From:      req.From,
			To:        req.To,
			Cc:        req.Cc,
			Bcc:       req.Bcc,
			Subject:   req.Subject,
			Content:   req.Content,
			Type:      0, // received
			ReplyTo:   req.ReplyTo,
			ThreadID:  threadID,
			UserID:    recipientID,
			CreatedAt: senderMail.CreatedAt,
		}

		if _, err := mailCollection.InsertOne(ctx, recipientMail); err != nil {
			return err
		}

		// Update recipient's thread
		recipientThreadMail := threadMail
		recipientThreadMail.Type = 0 // received

		recipientIDObj, _ := primitive.ObjectIDFromHex(recipientID)
		if err := h.updateThread(ctx, threadCollection, recipientIDObj, threadID, recipientThreadMail); err != nil {
			return err
		}
	}

	return nil
}

// ListMails retrieves mails for a user
func (h *DBHandler) ListMails(ctx context.Context, req *models.ListMailsRequest) ([]*models.Mail, error) {
	collection := h.db.Database.Collection("mails")

	filter := bson.M{"userId": req.UserID}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	if req.Limit > 0 {
		opts.SetLimit(int64(req.Limit))
	}
	if req.Offset > 0 {
		opts.SetSkip(int64(req.Offset))
	}

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var mails []*models.Mail
	if err := cursor.All(ctx, &mails); err != nil {
		return nil, err
	}

	return mails, nil
}

// SearchMails searches for mails matching the criteria
func (h *DBHandler) SearchMails(ctx context.Context, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	collection := h.db.Database.Collection("mails")

	filter := bson.M{
		"userId": req.UserID,
		"$or": []bson.M{
			{"subject": bson.M{"$regex": req.SearchTerm, "$options": "i"}},
			{"content": bson.M{"$regex": req.SearchTerm, "$options": "i"}},
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if req.Limit > 0 {
		opts.SetLimit(int64(req.Limit))
	}

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var mails []*models.Mail
	if err := cursor.All(ctx, &mails); err != nil {
		return nil, err
	}

	return mails, nil
}

// updateThread updates or creates a thread document
func (h *DBHandler) updateThread(ctx context.Context, collection *mongo.Collection, userID primitive.ObjectID, threadID string, threadMail models.ThreadMail) error {
	filter := bson.M{
		"user_id":   userID,
		"thread_id": threadID,
	}

	update := bson.M{
		"$push": bson.M{
			"mails": threadMail,
		},
		"$inc": bson.M{
			"total_mails": 1,
		},
		"$setOnInsert": bson.M{
			"user_id":   userID,
			"thread_id": threadID,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	return err
}
