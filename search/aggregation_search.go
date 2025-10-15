package search

import (
	"context"

	"mail-stress-test/database"
	"mail-stress-test/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AggregationSearchStrategy uses MongoDB aggregation pipeline
type AggregationSearchStrategy struct{}

func NewAggregationSearchStrategy() *AggregationSearchStrategy {
	return &AggregationSearchStrategy{}
}

func (s *AggregationSearchStrategy) GetName() string {
	return "aggregation"
}

func (s *AggregationSearchStrategy) GetDescription() string {
	return "MongoDB Aggregation Pipeline - powerful for complex queries and transformations"
}

func (s *AggregationSearchStrategy) SetupDatabase(ctx context.Context, db *database.MongoDB) error {
	collection := db.Database.Collection("mails")

	// Create indexes to support aggregation
	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("mail_userid_created_idx"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (s *AggregationSearchStrategy) SearchMails(ctx context.Context, db *database.MongoDB, req *models.SearchMailsRequest) ([]*models.Mail, error) {
	collection := db.Database.Collection("mails")

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"userId": req.UserID,
				"$or": []bson.M{
					{"subject": bson.M{"$regex": req.SearchTerm, "$options": "i"}},
					{"content": bson.M{"$regex": req.SearchTerm, "$options": "i"}},
				},
			},
		},
		{
			"$addFields": bson.M{
				"relevanceScore": bson.M{
					"$add": []interface{}{
						bson.M{
							"$cond": []interface{}{
								bson.M{"$regexMatch": bson.M{
									"input":   "$subject",
									"regex":   req.SearchTerm,
									"options": "i",
								}},
								10, // Higher score for subject match
								0,
							},
						},
						bson.M{
							"$cond": []interface{}{
								bson.M{"$regexMatch": bson.M{
									"input":   "$content",
									"regex":   req.SearchTerm,
									"options": "i",
								}},
								5, // Lower score for content match
								0,
							},
						},
					},
				},
			},
		},
		{
			"$sort": bson.M{
				"relevanceScore": -1,
				"createdAt":      -1,
			},
		},
	}

	if req.Limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": req.Limit})
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
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
