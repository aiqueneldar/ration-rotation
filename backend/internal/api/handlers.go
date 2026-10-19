package api

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"ration-rotation/internal/models"
	"ration-rotation/internal/store"

	"github.com/gin-gonic/gin"
)

// Handlers holds API handler functions
type Handlers struct {
	itemStore *store.ItemStore
}

func NewHandlers(itemStore *store.ItemStore) *Handlers {
	return &Handlers{itemStore: itemStore}
}

// GetItems handles GET /api/items
func (h *Handlers) GetItems(c *gin.Context) {
	var items []*models.Item
	var err error

	offsetStr := c.Query("offset")
	limitStr := c.Query("limit")

	offset := 0
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			offset = 0
		}
	}

	limit := 100
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			limit = 100
		}
		if limit < 1 {
			limit = 100
		}
	}

	items, err = h.itemStore.GetAll(limit, offset)
	if err != nil {
		log.Printf("GetItems error: %v", err)
		http.Error(c.Writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, items)
}

// CreateItem handles POST /api/items
func (h *Handlers) CreateItem(c *gin.Context) {
	var item models.Item

	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err, "message": "Invalid request body"})
		return
	}

	item.StockDate = time.Now()
	if item.DaysBeforeWarning == 0 {
		item.DaysBeforeWarning = 7 // Default
	}

	// Sanity checks
	if item.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item is missing mandatory field: Name"})
	}

	if item.Quantity <= 0.0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item quantity has to be more than 0.0"})
	}

	if item.BestBeforeDate.Equal(time.Time{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item must have best before date"})
	}

	if err := h.itemStore.Create(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create item"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusCreated, gin.H{
		"message": "Item created successfully",
		"id":      item.ID,
		"name":    item.Name,
	})
}

// GetItemByBarcode handles GET /api/items/barcode/:barcode
func (h *Handlers) GetItemByBarcode(c *gin.Context) {
	barcode := c.Param("barcode")

	if barcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Barcode parameter required"})
		return
	}

	item, err := h.itemStore.GetByBarcode(barcode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

// GetByID handles GET /api/items/:id
func (h *Handlers) GetByID(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	item, err := h.itemStore.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, item)
}

// UpdateItem handles PATCH /api/items/:id
func (h *Handlers) UpdateItem(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item id required"})
		return
	}

	var item models.Item

	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	item.ID = id

	if err := h.itemStore.Update(id, &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err, "message": "Failed to update item"})
		return
	}

	// Set status and return JSON
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, item)
}

// DeleteItem handles DELETE /api/items/:id
func (h *Handlers) DeleteItem(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	if err := h.itemStore.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}

	c.Status(http.StatusNoContent)
}

// UploadImage handles POST /api/items/:id/image with multipart form data
func (h *Handlers) UploadImage(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	// Handle multipart/form-data image upload
	fileHeader, err := c.FormFile("image")
	if fileHeader == nil {
		// File header is nil, fallback to base64 query parameter
		imageDataStr := c.Query("image")
		if imageDataStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Image parameter or file required"})
			return
		}
		var img []byte
		img, err = base64.StdEncoding.DecodeString(imageDataStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image data"})
			return
		}

		item := models.Item{}
		item.ID = id
		item.Image = img

		if err := h.itemStore.Update(id, &item); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item image"})
			return
		}

		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, gin.H{
			"message":       "Image uploaded successfully",
			"image_size":    len(img),
			"upload_method": "base64",
			"item":          item,
		})
		return
	}

	// File upload succeeded - read the file into memory
	imageFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get image file object"})
	}
	imageData, err := io.ReadAll(imageFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read image file"})
		return
	}

	// Update item with image
	var item models.Item
	item.ID = id
	item.Image = imageData

	if err := h.itemStore.Update(id, &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item image"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{
		"message":       "Image uploaded successfully",
		"image_size":    len(imageData),
		"filename":      fileHeader.Filename,
		"upload_method": "multipart",
		"item":          item,
	})
}

// SnoozeItem handles PATCH /api/items/:id/snooze
func (h *Handlers) SnoozeItem(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	// Get days from query parameter
	daysStr := c.Request.URL.Query().Get("days")
	if daysStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Days parameter required"})
		return
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid days value"})
		return
	}

	item, err := h.itemStore.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// Extend the best before date
	newBestBefore := item.BestBeforeDate.AddDate(0, 0, days)
	item.BestBeforeDate = newBestBefore

	if err := h.itemStore.Update(id, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to snooze item"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Item '%s' snoozed by %d days. New expiry: %s",
			item.Name, days, newBestBefore.Format("2006-01-02")),
		"item": item,
	})
}

// RotateItem handles PATCH /api/items/:id/rotate
func (h *Handlers) RotateItem(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item ID required"})
		return
	}

	var rotateData struct {
		NewBestBeforeDate time.Time `json:"new_best_before_date"`
		StockDate         time.Time `json:"stock_date"`
		Quantity          float32   `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&rotateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	item, err := h.itemStore.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// Mark as rotated and set new dates
	item.StockDate = rotateData.StockDate
	item.BestBeforeDate = rotateData.NewBestBeforeDate
	item.Quantity = rotateData.Quantity

	if err := h.itemStore.Update(id, item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to rotate item", "error": err})
		return
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Item '%s' rotated successfully. New expiry: %s",
			item.Name, item.BestBeforeDate.Format("2006-01-02")),
		"item": item,
	})
}
