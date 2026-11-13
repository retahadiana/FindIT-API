package model

import "time"

type QuestionAnswer struct {
	Question  string `json:"question" bson:"question" validate:"required,min=3,max=300"`
	Answer    string `json:"answer,omitempty" bson:"answer,omitempty" validate:"omitempty,max=500"`
	IsMatched *bool  `json:"is_matched,omitempty" bson:"is_matched,omitempty"`
}

type VerificationSession struct {
	ID                string           `json:"id"`
	ClaimID           int              `json:"claim_id"`
	ItemID            int              `json:"item_id"`
	FinderID          int              `json:"finder_id"`
	ClaimantID        int              `json:"claimant_id"`
	Questions         []QuestionAnswer `json:"questions"`
	ConfidentialNotes string           `json:"confidential_notes,omitempty"`
	IsVerified        bool             `json:"is_verified"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type SubmitQARequest struct {
	Questions []QuestionAnswer `json:"questions" validate:"required,min=1,max=10,dive"`
}

type ReviewVerificationRequest struct {
	IsVerified        bool   `json:"is_verified"`
	ConfidentialNotes string `json:"confidential_notes" validate:"omitempty,max=500"`
}
