package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"Graft/internal/admin"
	"Graft/internal/metrics"
	"Graft/internal/models"
)

// AdminHandler serves authenticated admin APIs under /api/v1 (after StripPrefix).
type AdminHandler struct {
	svc *admin.Service
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(svc *admin.Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// Register attaches routes on mux (paths relative to /api/v1 mount).
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /rules", h.listRules)
	mux.HandleFunc("POST /rules", h.createRule)
	mux.HandleFunc("GET /rules/{id}/deliveries", h.listDeliveries)
	mux.HandleFunc("GET /rules/{id}", h.getRule)
	mux.HandleFunc("PUT /rules/{id}", h.updateRule)
	mux.HandleFunc("DELETE /rules/{id}", h.deleteRule)
	mux.HandleFunc("GET /metrics", h.getMetrics)
}

func (h *AdminHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics.WriteMetricsJSON(w)
}

type ruleWrite struct {
	Name                     string            `json:"name"`
	Description              string            `json:"description"`
	ListenPath               string            `json:"listen_path"`
	RequiredSignature        bool              `json:"required_signature"`
	SignatureHeader          string            `json:"signature_header"`
	SignatureFormat          string            `json:"signature_format"`
	SignatureTimestampHeader string            `json:"signature_timestamp_header"`
	SignatureMaxSkewSeconds  int               `json:"signature_max_skew_seconds"`
	SignatureSecret          string            `json:"signature_secret"`
	ClearSignatureSecret     bool              `json:"clear_signature_secret"`
	TransformTemplate        string            `json:"transform_template"`
	DestinationURL           string            `json:"destination_url"`
	DestinationMethod        string            `json:"destination_method"`
	DestinationHeaders       map[string]string `json:"destination_headers"`
}

func (r ruleWrite) toInput() admin.RuleInput {
	return admin.RuleInput{
		Name:                     r.Name,
		Description:              r.Description,
		ListenPath:               r.ListenPath,
		RequiredSignature:        r.RequiredSignature,
		SignatureHeader:          r.SignatureHeader,
		SignatureFormat:          r.SignatureFormat,
		SignatureTimestampHeader: r.SignatureTimestampHeader,
		SignatureMaxSkewSeconds:  r.SignatureMaxSkewSeconds,
		SignatureSecret:          r.SignatureSecret,
		ClearSignatureSecret:     r.ClearSignatureSecret,
		TransformTemplate:        r.TransformTemplate,
		DestinationURL:           r.DestinationURL,
		DestinationMethod:        r.DestinationMethod,
		DestinationHeaders:       r.DestinationHeaders,
	}
}

func (h *AdminHandler) listRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rules, err := h.svc.ListRules(r.Context())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []models.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *AdminHandler) getRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	rule, err := h.svc.GetRule(r.Context(), id)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *AdminHandler) createRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in ruleWrite
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rule, err := h.svc.CreateRule(r.Context(), in.toInput())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") { // Basic check, maybe improve service error returns
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}
		// Basic validation errors check
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *AdminHandler) updateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	var in ruleWrite
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rule, err := h.svc.UpdateRule(r.Context(), id, in.toInput())
	if err != nil {
		if errors.Is(err, models.ErrRuleNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must") || strings.Contains(err.Error(), "cannot") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *AdminHandler) deleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if err := h.svc.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, models.ErrRuleNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) listDeliveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	
	// Validate rule exists first?
	// Service ListDeliveries doesn't explicitly check existence but Repo finds by RuleID.
	// We can check if rule exists first.
	rule, err := h.svc.GetRule(r.Context(), id)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	d, err := h.svc.ListDeliveries(r.Context(), id, limit)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if d == nil {
		d = []models.Delivery{}
	}
	writeJSON(w, http.StatusOK, d)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

