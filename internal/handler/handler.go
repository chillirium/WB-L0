package handler

import (
	"encoding/json"
	"net/http"

	"go-kafka-postgres/internal/cache"
	"go-kafka-postgres/internal/db"
	"go-kafka-postgres/internal/logger"
)

// Handler обрабатывает HTTP запросы
type Handler struct {
	cache cache.Cache
	db    db.DatabaseInterface
}

// New создает новый обработчик
func New(cache cache.Cache, db db.DatabaseInterface) *Handler {
	return &Handler{cache: cache, db: db}
}

// GetOrder обрабатывает запрос на получение заказа
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		http.Error(w, "Missing order uid", http.StatusBadRequest)
		return
	}

	order, found := h.cache.Get(uid)
	if found {
		logger.Infof("Order %s получен из кэша", uid)
	} else {
		// Fallback to DB
		var err error
		order, err = h.db.GetOrderByUID(uid)
		if err != nil {
			logger.Errorf("Failed to get order from DB: %v", err)
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		h.cache.Set(order) // Add to cache
		logger.Infof("Order %s получен из базы данных", uid)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		logger.Errorf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}
