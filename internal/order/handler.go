package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
	stream  Streamer
}

type Streamer interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request, orderID string, initial any)
}

func NewHandler(service *Service, stream Streamer) *Handler {
	return &Handler{service: service, stream: stream}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orders", h.createOrder)
	mux.HandleFunc("GET /api/v1/orders/{orderId}", h.getOrder)
	mux.HandleFunc("GET /api/v1/orders/{orderId}/history", h.history)
	mux.HandleFunc("POST /api/v1/orders/{orderId}/transition", h.transition)
	mux.HandleFunc("POST /api/v1/orders/{orderId}/cancel", h.cancel)
	mux.HandleFunc("GET /api/v1/orders/{orderId}/stream", h.streamOrder)
	mux.HandleFunc("GET /healthz", h.health)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.CustomerID) == "" {
		writeError(w, http.StatusBadRequest, "customerId is required")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "at least one item is required")
		return
	}

	order, err := h.service.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create order failed")
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.Get(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	history, err := h.service.History(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	idemKey := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idemKey) == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header required")
		return
	}

	var req TransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(string(req.TargetState)) == "" {
		writeError(w, http.StatusBadRequest, "targetState is required")
		return
	}

	order, err := h.service.Transition(r.Context(), r.PathValue("orderId"), req, idemKey)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	idemKey := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idemKey) == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header required")
		return
	}

	order, err := h.service.Cancel(r.Context(), r.PathValue("orderId"), idemKey)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) streamOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.Get(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.stream.ServeHTTP(w, r, order.ID, order)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	var invalid *ErrInvalidTransition
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusConflict, invalid.Error())
	case errors.Is(err, ErrConcurrentModification):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
