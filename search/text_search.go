package search

import (
	"context"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TextSearchStrategy uses MongoDB's full-text search capability
type TextSearchStrategy struct{}

func NewTextSearchStrategy() *TextSearchStrategy {
	return &TextSearchStrategy{}
}

func (s *TextSearchStrategy) GetName() string {
	return "text_search"
}

func (s *TextSearchStrategy) GetDescription() string {
	return "MongoDB Text Index with $text operator - best for natural language search"
}

func (s *TextSearchStrategy) SetupDatabase(ctx context.Context, db *database.MongoDB) error {
	collection := db.Database.Collection("mails")

	// Drop existing text index if any
	indexes := collection.Indexes()
	cursor, err := indexes.List(ctx)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return err
	}

	for _, idx := range results {
		if name, ok := idx["name"].(string); ok && name != "_id_" {
			// Drop non-primary indexes to recreate
			if key, ok := idx["key"].(bson.M); ok {
				if _, hasText := key["_fts"]; hasText {
					indexes.DropOne(ctx, name)
				}
			}
		}
	}

	// Create text index on subject and content
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"subject": "text",
			"content": "text",
		},
		Options: options.Index().SetName("mail_text_index"),
	}

	_, err = collection.Indexes().CreateOne(ctx, indexModel)
	return err
}

func (s *TextSearchStrategy) SearchMails(ctx context.Context, db *database.MongoDB, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	collection := db.Database.Collection("mails")

	filter := bson.M{
		"userId": req.UserID,
		"$text":  bson.M{"$search": req.SearchTerm},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "score", Value: bson.M{"$meta": "textScore"}}}).
		SetProjection(bson.M{"score": bson.M{"$meta": "textScore"}})

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
