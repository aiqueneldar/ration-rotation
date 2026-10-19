package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver import
)

var DB *sql.DB

// InitDB connects to PostgreSQL and sets up connection pooling
func InitDB(url string) error {
	var err error
	DB, err = sql.Open("postgres", url)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Database connected successfully")
	return nil
}

// CreateTablesIfMissing creates the items table if it doesn't exist
func CreateTablesIfMissing() error {
	createSQL := `
	CREATE TABLE IF NOT EXISTS items (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		quantity REAL NOT NULL,
		description TEXT,
		ingredients TEXT,
		stock_date DATE NOT NULL DEFAULT CURRENT_DATE,
		best_before_date DATE NOT NULL,
		days_before_warning INTEGER DEFAULT 7,
		image BYTEA,
		barcode VARCHAR(100),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_items_barcode ON items(barcode);
	CREATE INDEX IF NOT EXISTS idx_items_best_before_date ON items(best_before_date);
	CREATE INDEX IF NOT EXISTS idx_items_stock_date ON items(stock_date);
	`

	if _, err := DB.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database tables initialized successfully")
	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
