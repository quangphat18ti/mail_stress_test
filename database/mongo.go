package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri, dbName string, timeout int) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &MongoDB{
		Client:   client,
		Database: client.Database(dbName),
	}, nil
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}

func (m *MongoDB) CreateIndexes(ctx context.Context) error {
	// Mail collection indexes
	mailCollection := m.Database.Collection("mails")
	_, err := mailCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: map[string]interface{}{"userId": 1}},
		{Keys: map[string]interface{}{"threadId": 1}},
		{Keys: map[string]interface{}{"createdAt": -1}},
		{Keys: map[string]interface{}{"subject": "text", "content": "text"}},
	})
	if err != nil {
		return err
	}

	// Thread collection indexes
	threadCollection := m.Database.Collection("threads")
	_, err = threadCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: map[string]interface{}{"user_id": 1, "thread_id": 1}},
		{Keys: map[string]interface{}{"user_id": 1}},
	})

	return err
}
