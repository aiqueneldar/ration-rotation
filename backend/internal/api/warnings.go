package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ration-rotation/internal/config"
	"ration-rotation/internal/db"
	"ration-rotation/internal/models"
	"ration-rotation/internal/store"

	"github.com/gin-gonic/gin"
)

// WarningsService handles expiring items and signal notifications
type WarningsService struct {
	itemStore     *store.ItemStore
	signalServer  *SignalServer // Use pointer to avoid interface nil comparison issues
	checkInterval int           // in minutes
	config        *config.Config
}

func NewWarningsService(itemStore *store.ItemStore, signalServer *SignalServer, checkInterval int, config *config.Config) *WarningsService {
	return &WarningsService{
		itemStore:     itemStore,
		signalServer:  signalServer,
		checkInterval: checkInterval,
		config:        config,
	}
}

// GetExpiringItems returns items that will expire within their intervals
func (ws *WarningsService) GetExpiringItems(c *gin.Context) {
	items, err := ws.getExpiringNow()
	if err != nil {
		log.Printf("Error in GetExpiringItems: %v", err)
		return
	}

	// If no items found, return empty array
	if len(items) == 0 {
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, []*models.Item{})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, items)
}

// TriggerCheck triggers manual warning check (for /api/warnings/check endpoint)
func (ws *WarningsService) TriggerCheck() error {
	return ws.CheckAndNotify()
}

// CheckAndNotify checks for expiring items and sends notifications
func (ws *WarningsService) CheckAndNotify() error {
	items, err := ws.getExpiringNow()
	if err != nil {
		return fmt.Errorf("failed to get expiring items: %w", err)
	}

	msgList := []string{}

	for _, item := range items {
		msg := fmt.Sprintf("⚠️ %s (%s) expires in %f days!\n\nItem link: "+ws.config.Server.Domain.Full+"/items/"+item.ID,
			item.Name, item.ID, time.Until(item.BestBeforeDate).Hours()/24)
		msgList = append(msgList, msg)
	}

	if ws.signalServer != nil {
		msg := strings.Join(msgList, "\n\n")
		if err := ws.sendSignalNotification(msg); err != nil {
			return fmt.Errorf("Failed to send notification for message %s: %w\n", msg, err)
		}
	}

	return nil
}

func (ws *WarningsService) getExpiringNow() ([]*models.Item, error) {
	var items []*models.Item
	// Get items expiring soon according to their limits
	query := `
		SELECT id, name, quantity, stock_date, best_before_date, days_before_warning
		FROM items
		WHERE best_before_date BETWEEN NOW() AND NOW() + make_interval(days => days_before_warning)
		ORDER BY best_before_date ASC
	`
	rows, err := db.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := &models.Item{}

		if err := rows.Scan(
			&item.ID, &item.Name, &item.Quantity,
			&item.StockDate, &item.BestBeforeDate,
			&item.DaysBeforeWarning); err != nil {
			log.Printf("Scan error: %v", err)
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (ws *WarningsService) sendSignalNotification(message string) error {
	if ws.signalServer == nil {
		return fmt.Errorf("no signal client configured")
	}

	toSlice := strings.Split(ws.config.Signal.To, ",")

	ws.signalServer.Send(toSlice, message)
	return nil
}
