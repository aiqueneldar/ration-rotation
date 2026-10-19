package models

import (
	"time"
)

// Item represents an inventory item in the database
type Item struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Quantity          float32   `json:"quantity,omitempty"`
	Description       string    `json:"description,omitempty"`
	Ingredients       string    `json:"ingredients,omitempty"`
	StockDate         time.Time `json:"stock_date"`
	BestBeforeDate    time.Time `json:"best_before_date"`
	DaysBeforeWarning int       `json:"days_before_warning,omitempty"`
	Image             []byte    `json:"image,omitempty"` // Exclude from JSON serialization
	Barcode           string    `json:"barcode,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// NewEmptyItem creates an empty item for update operations
func NewEmptyItem() *Item {
	return &Item{}
}
