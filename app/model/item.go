package model

import (
	"fmt"
	"strings"
	"time"
)

// FlexibleDate mendukung berbagai format tanggal umum: ISO-8601, RFC3339, YYYY-MM-DD, DD/MM/YYYY, dsb.
type FlexibleDate time.Time

func (fd *FlexibleDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "" || s == "null" {
		*fd = FlexibleDate(time.Now().UTC())
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02/01/2006",
		"02-01-2006",
		"02/01/2006 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			*fd = FlexibleDate(t.UTC())
			return nil
		}
	}
	return fmt.Errorf("format tanggal '%s' tidak valid, gunakan format 'YYYY-MM-DD', 'DD/MM/YYYY', atau ISO RFC3339", s)
}

func (fd FlexibleDate) Time() time.Time {
	t := time.Time(fd)
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func (fd FlexibleDate) MarshalJSON() ([]byte, error) {
	return time.Time(fd).MarshalJSON()
}

type Item struct {
	ID          int        `json:"id"`
	UserID      int        `json:"user_id"`
	CategoryID  int        `json:"category_id"`
	Category    string     `json:"category_name,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	ItemType    string     `json:"item_type"`
	Location    string     `json:"location"`
	EventDate   time.Time  `json:"event_date"`
	Status      string     `json:"status"`
	Version     int        `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type CreateItemRequest struct {
	CategoryID  int          `json:"category_id" validate:"required,min=1"`
	Title       string       `json:"title" validate:"required,min=3,max=150"`
	Description string       `json:"description" validate:"required,min=10,max=2000"`
	ItemType    string       `json:"item_type" validate:"required,oneof=lost found"`
	Location    string       `json:"location" validate:"required,min=3,max=150"`
	EventDate   FlexibleDate `json:"event_date,omitempty"`
}

type UpdateItemRequest struct {
	CategoryID  int    `json:"category_id" validate:"required,min=1"`
	Title       string `json:"title" validate:"required,min=3,max=150"`
	Description string `json:"description" validate:"required,min=10,max=2000"`
	Location    string `json:"location" validate:"required,min=3,max=150"`
	Status      string `json:"status,omitempty" swaggerignore:"true" validate:"omitempty,oneof=open claimed returned closed"`
}

type ItemQuery struct {
	ListQuery
	ItemType   string
	Status     string
	CategoryID int
}

type ItemCursorQuery struct {
	CursorQuery
	ItemType   string
	Status     string
	CategoryID int
}
