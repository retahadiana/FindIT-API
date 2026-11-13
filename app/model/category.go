// === FILE: app/model/category.go ===
package model

import "time"

type Category struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateCategoryRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=80"`
	Description string `json:"description" validate:"omitempty,max=300"`
}
