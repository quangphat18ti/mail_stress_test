package search

import (
	"context"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RegexSearchStrategy uses regex pattern matching
type RegexSearchStrategy struct{}

func NewRegexSearchStrategy() *RegexSearchStrategy {
	return &RegexSearchStrategy{}
}

func (s *RegexSearchStrategy) GetName() string {
	return "regex"
}

func (s *RegexSearchStrategy) GetDescription() string {
	return "MongoDB Regex with $regex operator - flexible but can be slow on large datasets"
}

func (s *RegexSearchStrategy) SetupDatabase(ctx context.Context, db *database.MongoDB) error {
	collection := db.Database.Collection("mails")

	// Create compound index on userId and subject/content for better regex performance
	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "subject", Value: 1}},
			Options: options.Index().SetName("mail_userid_subject_idx"),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "content", Value: 1}},
			Options: options.Index().SetName("mail_userid_content_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (s *RegexSearchStrategy) SearchMails(ctx context.Context, db *database.MongoDB, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	collection := db.Database.Collection("mails")

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
