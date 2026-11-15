package model

import "time"

// FeedbackInput adalah payload ulasan dari klien saat memberikan feedback.
type FeedbackInput struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment,omitempty" validate:"omitempty,max=500"`
}

// Feedback menyimpan ulasan dan rating dari pemilik barang kepada penemu barang.
type Feedback struct {
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
	Comment    string `json:"comment,omitempty" validate:"omitempty,max=500"`
	FromUserID int    `json:"from_user_id,omitempty"`
}

// ReputationLog merepresentasikan satu dokumen audit pemberian poin reputasi di MongoDB.
type ReputationLog struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	ClaimID   int       `json:"claim_id"`
	ItemID    int       `json:"item_id"`
	Points    int       `json:"points"`
	Reason    string    `json:"reason"`
	Feedback  *Feedback `json:"feedback,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AddReputationRequest adalah payload untuk memberikan poin reputasi dan feedback
// kepada penemu setelah pengembalian barang selesai.
type AddReputationRequest struct {
	ClaimID  int            `json:"claim_id" validate:"required,min=1"`
	Reason   string         `json:"reason" validate:"required,min=3,max=255"`
	Feedback *FeedbackInput `json:"feedback,omitempty" validate:"omitempty"`
	Points   int            `json:"points,omitempty" swaggerignore:"true"`
	ItemID   int            `json:"item_id,omitempty" swaggerignore:"true"`
}


type ReputationSummary struct {
	UserID       int            `json:"user_id"`
	Count        int            `json:"count"`
	TotalPoints  int            `json:"total_points"`
	AvgRating    float64        `json:"avg_rating"`
	Distribution map[string]int `json:"distribution"`
}
