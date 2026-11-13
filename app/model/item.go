// === FILE: app/model/item.go ===
package model

import "time"

type Item struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	CategoryID  int       `json:"category_id"`
	Category    string    `json:"category_name,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ItemType    string    `json:"item_type"`
	Location    string    `json:"location"`
	EventDate   time.Time `json:"event_date"`
	Status      string    `json:"status"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateItemRequest struct {
	CategoryID  int       `json:"category_id" validate:"required,min=1"`
	Title       string    `json:"title" validate:"required,min=3,max=150"`
	Description string    `json:"description" validate:"required,min=10,max=2000"`
	ItemType    string    `json:"item_type" validate:"required,oneof=lost found"`
	Location    string    `json:"location" validate:"required,min=3,max=150"`
	EventDate   time.Time `json:"event_date" validate:"required"`
}

type UpdateItemRequest struct {
	CategoryID  int    `json:"category_id" validate:"required,min=1"`
	Title       string `json:"title" validate:"required,min=3,max=150"`
	Description string `json:"description" validate:"required,min=10,max=2000"`
	Location    string `json:"location" validate:"required,min=3,max=150"`
	Status      string `json:"status" validate:"required,oneof=open claimed returned closed"`
}

type ItemQuery struct {
	ListQuery
	ItemType   string
	Status     string
	CategoryID int
}
