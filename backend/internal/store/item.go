package store

import (
	"database/sql"
	"fmt"
	"log"

	"time"

	mp "github.com/geraldo-labs/merge-struct"
	"github.com/google/uuid"

	"ration-rotation/internal/models"
)

// ItemStore handles item business logic
type ItemStore struct {
	db *sql.DB
}

func NewItemStore(db *sql.DB) *ItemStore {
	return &ItemStore{db: db}
}

// GetByID retrieves an item by its ID
func (s *ItemStore) GetByID(id string) (*models.Item, error) {
	query := `
		SELECT
			id, name, quantity, description, ingredients,
			stock_date, best_before_date, days_before_warning,
			image, barcode, created_at, updated_at
		FROM items
		WHERE id = $1
	`

	var item models.Item

	err := s.db.QueryRow(query, id).Scan(
		&item.ID, &item.Name, &item.Quantity, &item.Description, &item.Ingredients,
		&item.StockDate, &item.BestBeforeDate, &item.DaysBeforeWarning,
		&item.Image, &item.Barcode,
		&item.CreatedAt, &item.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	}

	return &item, nil
}

// GetByBarcode retrieves items by barcode,
// potentially several added items can share barcode
// if there are different best before dates on different quantities
func (s *ItemStore) GetByBarcode(barcode string) ([]*models.Item, error) {
	query := `
		SELECT
			id, name, quantity, description, ingredients,
			stock_date, best_before_date, days_before_warning,
			image, barcode, created_at, updated_at
		FROM items
		WHERE barcode = $1
	`

	var item *models.Item
	var items []*models.Item

	rows, err := s.db.Query(query, barcode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item = models.NewEmptyItem()

		if err := rows.Scan(
			&item.ID, &item.Name, &item.Quantity, &item.Description, &item.Ingredients,
			&item.StockDate, &item.BestBeforeDate, &item.DaysBeforeWarning,
			&item.Image, &item.Barcode,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

// GetAll retrieves all items (for the list view)
func (s *ItemStore) GetAll(limit int, offset int) ([]*models.Item, error) {
	query := `
		SELECT
			id, name, quantity, description, ingredients,
			stock_date, best_before_date, days_before_warning,
			image, barcode, created_at, updated_at
		FROM items
		ORDER BY name
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		item := &models.Item{}

		if err := rows.Scan(
			&item.ID, &item.Name, &item.Quantity, &item.Description, &item.Ingredients,
			&item.StockDate, &item.BestBeforeDate, &item.DaysBeforeWarning,
			&item.Image, &item.Barcode,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

// Create adds a new item to the database
func (s *ItemStore) Create(item *models.Item) error {
	query := `
		INSERT INTO items (
			id, name, quantity, description, ingredients,
			stock_date, best_before_date, days_before_warning,
			image, barcode
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if item.ID == "" {
		item.ID = uuid.New().String()
	}

	_, err := s.db.Exec(query,
		item.ID, item.Name, item.Quantity,
		item.Description, item.Ingredients,
		item.StockDate, item.BestBeforeDate, item.DaysBeforeWarning,
		item.Image, item.Barcode,
	)

	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	return nil
}

// Update modifies an existing item
func (s *ItemStore) Update(id string, item *models.Item) error {
	query := `
		UPDATE items SET
			name = $2, quantity = $3, description = $4, ingredients = $5,
			stock_date = $6, best_before_date = $7, days_before_warning = $8, image = $9, barcode = $10, updated_at = NOW()
		WHERE id = $1
	`
	org_item, err := s.GetByID(id)
	if err != nil {
		log.Println(err)
		return err
	}
	changed, err := mp.Struct(org_item, *item)
	if err != nil {
		log.Println(err)
		return err
	}
	if !changed {
		log.Println("No change to Item")
	}

	result, err := s.db.Exec(query, id, org_item.Name, org_item.Quantity,
		org_item.Description, org_item.Ingredients, org_item.StockDate,
		org_item.BestBeforeDate, org_item.DaysBeforeWarning, org_item.Image, org_item.Barcode)

	if err != nil {
		log.Println(err)
		return fmt.Errorf("failed to update item: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Println(err)
		return fmt.Errorf("item not found")
	}

	return nil
}

// Rotate updates an item's stock date and best before date (for rotation)
func (s *ItemStore) Rotate(id string, stockDate time.Time, bestBeforeDate time.Time) error {
	query := `
		UPDATE items SET
			stock_date = $2, best_before_date = $3, updated_at = NOW()
		WHERE id = $1
	`

	result, err := s.db.Exec(query, id, stockDate, bestBeforeDate)
	if err != nil {
		return fmt.Errorf("failed to rotate item: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// Delete removes an item from the database
func (s *ItemStore) Delete(id string) error {
	query := `DELETE FROM items WHERE id = $1`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}
