package controlplane

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/partflow"
	"example.com/partflow/internal/observability"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	router   chi.Router
	store    *multipart.Store
	policies *multipart.PolicyRegistry
	quota    *multipart.QuotaLedger
	metrics  *observability.Registry
	now      func() time.Time
}

func New(store *multipart.Store, policies *multipart.PolicyRegistry, quota *multipart.QuotaLedger, now func() time.Time) (*Server, error) {
	if store == nil || policies == nil || quota == nil {
		return nil, fmt.Errorf("control plane dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	server := &Server{store: store, policies: policies, quota: quota, metrics: observability.NewRegistry(256), now: now}
	router := chi.NewRouter()
	router.Use(server.requestContext)
	router.Get("/healthz", server.health)
	router.Route("/v1/tenants/{tenant}", func(r chi.Router) {
		r.Get("/quota", server.quotaStatus)
		r.Get("/metrics", server.tenantMetrics)
		r.Get("/policies", server.listPolicies)
		r.Put("/policies", server.replacePolicies)
		r.Post("/uploads", server.beginUpload)
		r.Get("/uploads/{uploadID}", server.getUpload)
	})
	server.router = router
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	now := s.now().UTC()
	health := observability.SummarizeHealth([]observability.Component{
		{Name: "quota", State: observability.StateReady, CheckedAt: now},
		{Name: "policy", State: observability.StateReady, CheckedAt: now},
		{Name: "store", State: observability.StateReady, CheckedAt: now},
	}, now)
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) quotaStatus(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	writeJSON(w, http.StatusOK, s.quota.Usage(tenant))
}

func (s *Server) tenantMetrics(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	metrics, _ := s.metrics.Tenant(tenant)
	writeJSON(w, http.StatusOK, map[string]any{
		"metrics":         metrics,
		"error_rate":      s.metrics.ErrorRate(tenant),
		"average_latency": s.metrics.AverageLatency(tenant).String(),
	})
}

func (s *Server) listPolicies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.policies.List(chi.URLParam(r, "tenant"))})
}

func (s *Server) replacePolicies(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Items []multipart.UploadPolicy `json:"items"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.policies.Replace(chi.URLParam(r, "tenant"), request.Items); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(request.Items)})
}

func (s *Server) beginUpload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID            string `json:"id"`
		ObjectKey     string `json:"object_key"`
		ExpectedParts int    `json:"expected_parts"`
		ExpectedBytes int64  `json:"expected_bytes"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tenant := chi.URLParam(r, "tenant")
	if _, ok := s.policies.Resolve(tenant, request.ObjectKey); !ok {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("no upload policy matches object"))
		return
	}
	finish, err := s.quota.Reserve(tenant, request.ExpectedBytes)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err)
		return
	}
	qualified := multipart.TenantUploadID(tenant, request.ID)
	if err := s.store.Begin(qualified, request.ObjectKey, request.ExpectedParts); err != nil {
		finish(false)
		writeError(w, http.StatusConflict, err)
		return
	}
	finish(true)
	s.metrics.Record(observability.Sample{Tenant: tenant, UploadID: qualified, Outcome: observability.OutcomeAccepted, Bytes: request.ExpectedBytes, At: s.now()})
	writeJSON(w, http.StatusAccepted, map[string]any{"id": request.ID, "qualified_id": qualified})
}

func (s *Server) getUpload(w http.ResponseWriter, r *http.Request) {
	qualified := multipart.TenantUploadID(chi.URLParam(r, "tenant"), chi.URLParam(r, "uploadID"))
	session, err := s.store.Get(qualified)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = strconv.FormatInt(s.now().UnixNano(), 36)
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
