package model

import "time"

// Feedback menyimpan ulasan dan rating dari pemilik barang kepada penemu barang.
type Feedback struct {
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
	Comment    string `json:"comment,omitempty" validate:"omitempty,max=500"`
	FromUserID int    `json:"from_user_id"`
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
	ClaimID  int       `json:"claim_id" validate:"required,min=1"`
	ItemID   int       `json:"item_id" validate:"required,min=1"`
	Points   int       `json:"points" validate:"required,min=1,max=100"`
	Reason   string    `json:"reason" validate:"required,min=3,max=255"`
	Feedback *Feedback `json:"feedback,omitempty" validate:"omitempty"`
}


type ReputationSummary struct {
	UserID       int            `json:"user_id"`
	Count        int            `json:"count"`
	TotalPoints  int            `json:"total_points"`
	AvgRating    float64        `json:"avg_rating"`
	Distribution map[string]int `json:"distribution"`
}
