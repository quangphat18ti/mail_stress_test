package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Mail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"userId"`
	ThreadID  string             `bson:"threadId"`
	Subject   string             `bson:"subject"`
	Content   string             `bson:"content"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type Thread struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	ThreadID   string             `bson:"thread_id"`
	Mails      []ThreadMail       `bson:"mails"`
	TotalMails int                `bson:"total_mails"`
	UserID     primitive.ObjectID `bson:"user_id"`
}

type ThreadMail struct {
	From    string   `bson:"from"`
	MsgID   string   `bson:"msg_id"`
	Subject string   `bson:"subject"`
	Content string   `bson:"content"`
	Cc      []string `bson:"cc"`
	To      []string `bson:"to"`
	Bcc     []string `bson:"bcc"`
	Type    int      `bson:"type"`
}
