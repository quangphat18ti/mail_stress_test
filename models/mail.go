package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Mail represents a mail document in database
type Mail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	From      string             `bson:"from" json:"from"`
	To        []string           `bson:"to" json:"to"`
	Cc        []string           `bson:"cc,omitempty" json:"cc,omitempty"`
	Bcc       []string           `bson:"bcc,omitempty" json:"bcc,omitempty"`
	Subject   string             `bson:"subject" json:"subject"`
	Content   string             `bson:"content" json:"content"`
	Type      int                `bson:"type" json:"type"`                           // 0: received, 1: sent
	ReplyTo   string             `bson:"replyTo,omitempty" json:"replyTo,omitempty"` // ID of mail being replied to
	ThreadID  string             `bson:"threadId" json:"threadId"`
	UserID    string             `bson:"userId" json:"userId"` // Owner of this mail copy
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// MailRequest represents a request to create a mail
type MailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Content string   `json:"content"`
	ReplyTo string   `json:"replyTo,omitempty"` // If replying, ID of original mail
}

// ListMailsRequest represents a request to list mails
type ListMailsRequest struct {
	UserID string `json:"userId"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// SearchMailsRequest represents a request to search mails
type SearchMailsRequest struct {
	UserID     string `json:"userId"`
	SearchTerm string `json:"searchTerm"`
	Limit      int    `json:"limit,omitempty"`
}

// Thread represents a mail thread document
type Thread struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ThreadID   string             `bson:"thread_id" json:"threadId"`
	Mails      []ThreadMail       `bson:"mails" json:"mails"`
	TotalMails int                `bson:"total_mails" json:"totalMails"`
	UserID     primitive.ObjectID `bson:"user_id" json:"userId"`
}

// ThreadMail represents a mail reference in a thread
type ThreadMail struct {
	From    string   `bson:"from" json:"from"`
	MsgID   string   `bson:"msg_id" json:"msgId"`
	Subject string   `bson:"subject" json:"subject"`
	Content string   `bson:"content" json:"content"`
	Cc      []string `bson:"cc,omitempty" json:"cc,omitempty"`
	To      []string `bson:"to" json:"to"`
	Bcc     []string `bson:"bcc,omitempty" json:"bcc,omitempty"`
	Type    int      `bson:"type" json:"type"`
}
