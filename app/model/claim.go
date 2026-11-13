package model

import "time"

type Claim struct {
	ID               int       `json:"id"`
	ItemID           int       `json:"item_id"`
	ClaimantID       int       `json:"claimant_id"`
	ProofDescription string    `json:"proof_description"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateClaimRequest struct {
	ProofDescription string `json:"proof_description" validate:"required,min=10,max=2000"`
}

type UpdateClaimStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=approved rejected cancelled"`
}
