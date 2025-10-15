package search

import (
	"context"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexOptimizedStrategy uses compound indexes for optimal query performance
type IndexOptimizedStrategy struct{}

func NewIndexOptimizedStrategy() *IndexOptimizedStrategy {
	return &IndexOptimizedStrategy{}
}

func (s *IndexOptimizedStrategy) GetName() string {
	return "index_optimized"
}

func (s *IndexOptimizedStrategy) GetDescription() string {
	return "Compound Index on userId + subject/content with case-insensitive collation - best performance for exact/prefix matches"
}

func (s *IndexOptimizedStrategy) SetupDatabase(ctx context.Context, db *database.MongoDB) error {
	collection := db.Database.Collection("mails")

	// Create compound indexes with collation for case-insensitive search
	collation := &options.Collation{
		Locale:   "en",
		Strength: 2, // Case-insensitive
	}

	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "subject", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().
				SetName("mail_optimized_subject_idx").
				SetCollation(collation),
		},
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "content", Value: "text"},
			},
			Options: options.Index().
				SetName("mail_optimized_content_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (s *IndexOptimizedStrategy) SearchMails(ctx context.Context, db *database.MongoDB, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	collection := db.Database.Collection("mails")

	// Use regex with anchored pattern for better index utilization
	filter := bson.M{
		"userId": req.UserID,
		"$or": []bson.M{
			{"subject": bson.M{"$regex": "^.*" + req.SearchTerm, "$options": "i"}},
			{"content": bson.M{"$regex": req.SearchTerm, "$options": "i"}},
		},
	}

	collation := &options.Collation{
		Locale:   "en",
		Strength: 2,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetCollation(collation)

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
