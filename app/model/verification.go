package model

import (
	"encoding/json"
	"strings"
	"time"
)

// QuestionAnswer merepresentasikan satu butir pertanyaan dan jawaban verifikasi di MongoDB.
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

// VerificationQuestionItem merepresentasikan satu butir pertanyaan ciri rahasia dari penemu.
type VerificationQuestionItem struct {
	Question string `json:"question" validate:"required,min=3,max=300"`
}

// UnmarshalJSON mendukung baik string langsung maupun objek {"question": "..."}.
func (q *VerificationQuestionItem) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		q.Question = strings.TrimSpace(str)
		return nil
	}
	type rawItem VerificationQuestionItem
	var raw rawItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	q.Question = strings.TrimSpace(raw.Question)
	return nil
}

// CreateVerificationSessionRequest adalah payload penemu saat membuat daftar pertanyaan verifikasi rahasia.
type CreateVerificationSessionRequest struct {
	Questions []VerificationQuestionItem `json:"questions" validate:"required,min=1,max=10"`
}

// SubmitAnswersRequest adalah payload pengklaim saat mengirimkan jawaban atas pertanyaan verifikasi.
type SubmitAnswersRequest struct {
	Answers []string `json:"answers" validate:"required,min=1,max=10"`
}

// UnmarshalJSON mendukung {"answers": ["..."]} maupun {"questions": [{"answer": "..."}]}.
func (s *SubmitAnswersRequest) UnmarshalJSON(data []byte) error {
	type rawAnswers struct {
		Answers   []string `json:"answers"`
		Questions []struct {
			Answer string `json:"answer"`
		} `json:"questions"`
	}
	var raw rawAnswers
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Answers) > 0 {
		for _, a := range raw.Answers {
			s.Answers = append(s.Answers, strings.TrimSpace(a))
		}
	} else if len(raw.Questions) > 0 {
		for _, q := range raw.Questions {
			s.Answers = append(s.Answers, strings.TrimSpace(q.Answer))
		}
	}
	return nil
}

// SubmitQARequest adalah alias untuk kompatibilitas ke belakang.
type SubmitQARequest = CreateVerificationSessionRequest

type ReviewVerificationRequest struct {
	IsVerified        *bool  `json:"is_verified,omitempty"`
	Action            string `json:"action,omitempty" swaggerignore:"true" validate:"omitempty,oneof=pass fail approved rejected"`
	ConfidentialNotes string `json:"confidential_notes,omitempty" validate:"omitempty,max=500"`
	Notes             string `json:"notes,omitempty" swaggerignore:"true" validate:"omitempty,max=500"`
}
