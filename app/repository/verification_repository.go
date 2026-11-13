package repository

import (
	"context"
	"errors"
	"time"

	"findit-api/app/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrQuestionLimit = errors.New("jumlah pertanyaan verifikasi maksimal 10")

type VerificationRepository interface {
	EnsureIndexes(ctx context.Context) error
	CreateSession(ctx context.Context, s model.VerificationSession) (model.VerificationSession, error)
	FindByClaimID(ctx context.Context, claimID int) (model.VerificationSession, error)
	AddQuestion(ctx context.Context, claimID int, q model.QuestionAnswer) error
	UpdateAnswers(ctx context.Context, claimID int, questions []model.QuestionAnswer) (model.VerificationSession, error)
	Review(ctx context.Context, claimID int, verified bool, notes string) (model.VerificationSession, error)
}

type verificationDoc struct {
	ID                bson.ObjectID          `bson:"_id,omitempty"`
	ClaimID           int                    `bson:"claim_id"`
	ItemID            int                    `bson:"item_id"`
	FinderID          int                    `bson:"finder_id"`
	ClaimantID        int                    `bson:"claimant_id"`
	Questions         []model.QuestionAnswer `bson:"questions"`
	ConfidentialNotes string                 `bson:"confidential_notes,omitempty"`
	IsVerified        bool                   `bson:"is_verified"`
	CreatedAt         time.Time              `bson:"created_at"`
	UpdatedAt         time.Time              `bson:"updated_at"`
}

func (d verificationDoc) toModel() model.VerificationSession {
	return model.VerificationSession{
		ID: d.ID.Hex(), ClaimID: d.ClaimID, ItemID: d.ItemID, FinderID: d.FinderID,
		ClaimantID: d.ClaimantID, Questions: d.Questions, ConfidentialNotes: d.ConfidentialNotes,

		IsVerified: d.IsVerified, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type verificationMongoRepository struct{ coll *mongo.Collection }

func NewVerificationRepository(db *mongo.Database) VerificationRepository {
	return &verificationMongoRepository{coll: db.Collection("claim_verifications")}
}

func (r *verificationMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "claim_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("claim_id_unique"),
	})
	return err
}

func (r *verificationMongoRepository) CreateSession(ctx context.Context, s model.VerificationSession) (model.VerificationSession, error) {
	now := nowUTC()
	doc := verificationDoc{
		ID: bson.NewObjectID(), ClaimID: s.ClaimID, ItemID: s.ItemID, FinderID: s.FinderID,
		ClaimantID: s.ClaimantID, Questions: s.Questions, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		if isMongoDuplicateKey(err) {
			return model.VerificationSession{}, ErrDuplicate
		}
		return model.VerificationSession{}, err
	}
	return doc.toModel(), nil
}
func (r *verificationMongoRepository) FindByClaimID(ctx context.Context, claimID int) (model.VerificationSession, error) {
	var doc verificationDoc
	if err := r.coll.FindOne(ctx, bson.M{"claim_id": claimID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.VerificationSession{}, ErrNotFound
		}
		return model.VerificationSession{}, err
	}
	return doc.toModel(), nil
}

func (r *verificationMongoRepository) AddQuestion(ctx context.Context, claimID int, q model.QuestionAnswer) error {
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"claim_id": claimID, "questions.9": bson.M{"$exists": false}},
		bson.M{"$push": bson.M{"questions": q}, "$set": bson.M{"updated_at": nowUTC()}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrQuestionLimit
	}
	return nil
}

func (r *verificationMongoRepository) UpdateAnswers(ctx context.Context, claimID int, questions []model.QuestionAnswer) (model.VerificationSession, error) {
	var doc verificationDoc
	err := r.coll.FindOneAndUpdate(ctx,
		bson.M{"claim_id": claimID},
		bson.M{"$set": bson.M{"questions": questions, "updated_at": nowUTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return model.VerificationSession{}, ErrNotFound
	}
	return doc.toModel(), nil
}

func (r *verificationMongoRepository) Review(ctx context.Context, claimID int, verified bool, notes string) (model.VerificationSession, error) {
	var doc verificationDoc
	err := r.coll.FindOneAndUpdate(ctx,
		bson.M{"claim_id": claimID},
		bson.M{"$set": bson.M{"is_verified": verified, "confidential_notes": notes, "updated_at": nowUTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return model.VerificationSession{}, ErrNotFound
	}
	return doc.toModel(), nil
}
