package repository

import (
	"context"
	"fmt"
	"time"

	"findit-api/app/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ReputationRepository adalah kontrak penyimpanan audit log reputasi di MongoDB.
type ReputationRepository interface {
	EnsureIndexes(ctx context.Context) error
	CreateLog(ctx context.Context, log model.ReputationLog) (model.ReputationLog, error)
	FindByUserID(ctx context.Context, userID int) ([]model.ReputationLog, error)
	GetTotalReturnedCount(ctx context.Context, userID int) (int64, error)
	SummaryByUserID(ctx context.Context, userID int) (model.ReputationSummary, error)
}

type feedbackDocument struct {
	Rating     int    `bson:"rating"`
	Comment    string `bson:"comment,omitempty"`
	FromUserID int    `bson:"from_user_id"`
}

type reputationLogDocument struct {
	ID        bson.ObjectID     `bson:"_id,omitempty"`
	UserID    int               `bson:"user_id"`
	ClaimID   int               `bson:"claim_id"`
	ItemID    int               `bson:"item_id"`
	Points    int               `bson:"points"`
	Reason    string            `bson:"reason"`
	Feedback  *feedbackDocument `bson:"feedback,omitempty"`
	CreatedAt time.Time         `bson:"created_at"`
}

func (d reputationLogDocument) toModel() model.ReputationLog {
	var fb *model.Feedback
	if d.Feedback != nil {
		fb = &model.Feedback{
			Rating:     d.Feedback.Rating,
			Comment:    d.Feedback.Comment,
			FromUserID: d.Feedback.FromUserID,
		}
	}
	return model.ReputationLog{
		ID:        d.ID.Hex(),
		UserID:    d.UserID,
		ClaimID:   d.ClaimID,
		ItemID:    d.ItemID,
		Points:    d.Points,
		Reason:    d.Reason,
		Feedback:  fb,
		CreatedAt: d.CreatedAt,
	}
}

type reputationMongoRepository struct {
	coll *mongo.Collection
}

// NewReputationRepository membuat implementasi MongoDB untuk collection reputation_logs.
func NewReputationRepository(db *mongo.Database) ReputationRepository {
	return &reputationMongoRepository{
		coll: db.Collection("reputation_logs"),
	}
}

func (r *reputationMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("user_created_idx"),
		},
		{
			// Mencegah pemberian reward ganda pada klaim yang sama untuk user yang sama.
			Keys: bson.D{
				{Key: "claim_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("claim_user_unique"),
		},
	})
	if err != nil {
		return fmt.Errorf("membuat index reputation_logs: %w", err)
	}
	return nil
}

func (r *reputationMongoRepository) CreateLog(ctx context.Context, entry model.ReputationLog) (model.ReputationLog, error) {
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = nowUTC()
	}
	var fbDoc *feedbackDocument
	if entry.Feedback != nil {
		fbDoc = &feedbackDocument{
			Rating:     entry.Feedback.Rating,
			Comment:    entry.Feedback.Comment,
			FromUserID: entry.Feedback.FromUserID,
		}
	}
	doc := reputationLogDocument{
		ID:        bson.NewObjectID(),
		UserID:    entry.UserID,
		ClaimID:   entry.ClaimID,
		ItemID:    entry.ItemID,
		Points:    entry.Points,
		Reason:    entry.Reason,
		Feedback:  fbDoc,
		CreatedAt: createdAt,
	}

	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		if isMongoDuplicateKey(err) {
			return model.ReputationLog{}, ErrDuplicate
		}
		return model.ReputationLog{}, fmt.Errorf("menyimpan reputation log: %w", err)
	}
	return doc.toModel(), nil
}

func (r *reputationMongoRepository) FindByUserID(ctx context.Context, userID int) ([]model.ReputationLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, fmt.Errorf("mengambil riwayat reputasi: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []reputationLogDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("membaca riwayat reputasi: %w", err)
	}

	logs := make([]model.ReputationLog, 0, len(docs))
	for _, doc := range docs {
		logs = append(logs, doc.toModel())
	}
	return logs, nil
}

func (r *reputationMongoRepository) GetTotalReturnedCount(ctx context.Context, userID int) (int64, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return 0, fmt.Errorf("menghitung total pengembalian barang: %w", err)
	}
	return count, nil
}


// SummaryByUserID meringkas log reputasi & distribusi bintang memakai aggregation pipeline $facet.
func (r *reputationMongoRepository) SummaryByUserID(ctx context.Context, userID int) (model.ReputationSummary, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$facet", Value: bson.M{
			"stats": bson.A{
				bson.M{"$group": bson.M{
					"_id":          nil,
					"count":        bson.M{"$sum": 1},
					"total_points": bson.M{"$sum": "$points"},
					"avg_rating":   bson.M{"$avg": "$feedback.rating"},
				}},
			},
			"distribution": bson.A{
				bson.M{"$match": bson.M{"feedback.rating": bson.M{"$gte": 1}}},
				bson.M{"$group": bson.M{"_id": "$feedback.rating", "total": bson.M{"$sum": 1}}},
			},
		}}},
	}
	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return model.ReputationSummary{}, err
	}
	defer cursor.Close(ctx)
	var out []struct {
		Stats        []struct{ Count, TotalPoints int; AvgRating float64 } `bson:"stats"`
		Distribution []struct{ ID int `bson:"_id"`; Total int }            `bson:"distribution"`
	}
	if err := cursor.All(ctx, &out); err != nil {
		return model.ReputationSummary{}, err
	}
	sum := model.ReputationSummary{
		UserID:       userID,
		Distribution: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0},
	}
	if len(out) == 0 {
		return sum, nil
	}
	if len(out[0].Stats) > 0 {
		sum.Count = out[0].Stats[0].Count
		sum.TotalPoints = out[0].Stats[0].TotalPoints
		sum.AvgRating = float64(int(out[0].Stats[0].AvgRating*100+0.5)) / 100
	}
	for _, d := range out[0].Distribution {
		sum.Distribution[fmt.Sprintf("%d", d.ID)] = d.Total
	}
	return sum, nil
}
