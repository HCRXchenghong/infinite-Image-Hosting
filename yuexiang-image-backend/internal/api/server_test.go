package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/yuexiang/image-backend/internal/config"
	"github.com/yuexiang/image-backend/internal/domain"
	"github.com/yuexiang/image-backend/internal/queue"
	"github.com/yuexiang/image-backend/internal/storage"
)

func TestPublicPlansDoNotExposeInfiniteMax(t *testing.T) {
	server := NewServer(config.Load())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, plan := range payload.Data {
		if plan.Slug == "infinite-max" {
			t.Fatalf("public API leaked infinite-max")
		}
	}
}

func TestReadyzReportsMemoryStackReady(t *testing.T) {
	server := NewServer(config.Load())
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ready"`) {
		t.Fatalf("readyz expected ready 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReadyzReportsRedisFailure(t *testing.T) {
	cfg := config.Load()
	cfg.QueueDriver = "redis"
	cfg.RedisAddr = "127.0.0.1:1"
	server := NewServer(cfg)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"not_ready"`) {
		t.Fatalf("readyz expected not_ready 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecurityHeadersAreApplied(t *testing.T) {
	server := NewServer(config.Load())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff header")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("expected content security policy header")
	}
}

func TestMetricsExposeBusinessCounters(t *testing.T) {
	server := NewServer(config.Load())
	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	server.router.ServeHTTP(healthRec, healthReq)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "yuexiang_users_total") {
		t.Fatalf("metrics expected business counters, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `yuexiang_http_requests_total{method="GET",route="/healthz",status="200"}`) {
		t.Fatalf("metrics expected HTTP request counter, got %s", rec.Body.String())
	}
}

func TestProductionConfigRejectsDevelopmentSecrets(t *testing.T) {
	cfg := config.Load()
	cfg.AppEnv = "production"
	cfg.DatabaseURL = "postgres://example"
	cfg.StorageDriver = "s3"
	cfg.S3Bucket = "bucket"
	cfg.S3AccessKey = "access"
	cfg.S3SecretKey = "secret"
	defer func() {
		if recover() == nil {
			t.Fatalf("expected production config with development secrets to panic")
		}
	}()
	_ = NewServer(cfg)
}

func TestAuthPasswordAndEmailVerificationFlow(t *testing.T) {
	server := NewServer(config.Load())
	rec := jsonRequest(server, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":    "test@example.com",
		"password": "secret123",
		"nickname": "Tester",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var registerPayload struct {
		Data struct {
			Token string `json:"token"`
			User  struct {
				ID string `json:"id"`
			} `json:"user"`
			Code string `json:"dev_email_verification_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &registerPayload); err != nil {
		t.Fatal(err)
	}
	if registerPayload.Data.Token == "" || registerPayload.Data.Code == "" {
		t.Fatalf("expected token and dev code: %s", rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodPost, "/api/v1/auth/resend-verification", map[string]any{
		"user_id": registerPayload.Data.User.ID,
	}, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "dev_email_verification_code") {
		t.Fatalf("resend verification expected dev code, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "test@example.com",
		"password": "wrong-password",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password expected 401, got %d", rec.Code)
	}

	rec = jsonRequest(server, http.MethodPost, "/api/v1/auth/verify-email", map[string]any{
		"user_id": registerPayload.Data.User.ID,
		"code":    registerPayload.Data.Code,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("verify expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodGet, "/api/v1/auth/me", nil, registerPayload.Data.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("me expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserSettingsProfileAndDestroyRequest(t *testing.T) {
	server := NewServer(config.Load())
	token, _, code := registerTestUser(t, server, "settings@example.com")
	verifyTestUser(t, server, token, code)
	rec := jsonRequest(server, http.MethodPatch, "/api/v1/settings/profile", map[string]any{"nickname": "运营同学"}, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "运营同学") {
		t.Fatalf("profile update expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/settings/account-destroy-request", map[string]any{"reason": "migration"}, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "destroy_") {
		t.Fatalf("destroy request expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(server.state.auditLogs) == 0 || len(server.state.riskEvents) == 0 {
		t.Fatalf("expected audit and risk event after destroy request")
	}
}

func TestUploadRequiresAuthAndAPIKeyScopes(t *testing.T) {
	server := NewServer(config.Load())
	rec := multipartUpload(server, "/api/v1/images", "", "tiny.txt", []byte("not-an-image"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized upload expected 401, got %d", rec.Code)
	}

	token, _, code := registerTestUser(t, server, "scope@example.com")
	rec = jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "unverified",
		"scopes": []string{"images:read"},
	}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unverified user key creation expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	verifyTestUser(t, server, token, code)
	rec = jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "invalid-scope",
		"scopes": []string{"admin:*"},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid API key scope expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "read-only",
		"scopes": []string{"images:read"},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("create key expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var keyPayload struct {
		Data struct {
			APIKey struct {
				Secret string `json:"secret"`
			} `json:"api_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &keyPayload); err != nil {
		t.Fatal(err)
	}
	rec = multipartUpload(server, "/api/v1/images", keyPayload.Data.APIKey.Secret, "tiny.txt", []byte("not-an-image"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only key upload expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyWithWriteScopeCanUpload(t *testing.T) {
	server := NewServer(config.Load())
	token, _, code := registerTestUser(t, server, "writer@example.com")
	verifyTestUser(t, server, token, code)
	rec := jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "writer",
		"scopes": []string{"images:read", "images:write"},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("create key expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var keyPayload struct {
		Data struct {
			APIKey struct {
				Secret string `json:"secret"`
			} `json:"api_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &keyPayload); err != nil {
		t.Fatal(err)
	}
	rec = multipartUpload(server, "/api/v1/images", keyPayload.Data.APIKey.Secret, "tiny.txt", []byte("not-an-image"))
	if rec.Code != http.StatusOK {
		t.Fatalf("write key upload expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var uploadPayload struct {
		Data struct {
			Image struct {
				PublicID string `json:"public_id"`
			} `json:"image"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatal(err)
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/images", nil, keyPayload.Data.APIKey.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("API key list expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	userID := currentOnlyUserID(t, server)
	if got := server.state.usage[userID].APICalls; got < 2 {
		t.Fatalf("expected API key calls to be counted, got %d", got)
	}
	if uploadPayload.Data.Image.PublicID == "" {
		t.Fatalf("expected uploaded public id")
	}
}

func TestBannedUserAPIKeyIsDeniedAndRevoked(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	token, userID, code := registerTestUser(t, server, "banned-key@example.com")
	verifyTestUser(t, server, token, code)
	rec := jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "writer",
		"scopes": []string{"images:read", "images:write"},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("create key expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var keyPayload struct {
		Data struct {
			APIKey struct {
				Secret string `json:"secret"`
			} `json:"api_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &keyPayload); err != nil {
		t.Fatal(err)
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/ban", map[string]any{"reason": "abuse"}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("ban expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/images", nil, keyPayload.Data.APIKey.Secret)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked/banned API key expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	defer server.state.mu.RUnlock()
	if len(server.state.apiKeys[userID]) == 0 || !server.state.apiKeys[userID][0].Revoked {
		t.Fatalf("expected user API keys to be revoked on ban")
	}
}

func TestProcessImageTaskUpdatesImageMetadata(t *testing.T) {
	server := NewServer(config.Load())
	body := tinyPNG(t)
	ctx := context.Background()
	if err := server.store.PutObject(ctx, storage.PutObjectInput{
		Key:         "original/img_worker.png",
		Body:        bytes.NewReader(body),
		ContentType: "image/png",
		Private:     true,
	}); err != nil {
		t.Fatal(err)
	}
	server.state.mu.Lock()
	server.state.users["usr_worker"] = User{ID: "usr_worker", Email: "worker@example.com", PlanSlug: "go", Status: "active"}
	server.state.images["img_worker"] = Image{
		ID:          "image_worker",
		PublicID:    "img_worker",
		UserID:      "usr_worker",
		Filename:    "worker.png",
		ObjectKey:   "original/img_worker.png",
		ContentType: "image/png",
		Bytes:       int64(len(body)),
		Status:      "active",
	}
	server.state.mu.Unlock()

	if err := server.ProcessTask(ctx, queue.Task{Type: "image.process", Payload: map[string]any{"public_id": "img_worker"}}); err != nil {
		t.Fatalf("process image task failed: %v", err)
	}
	server.state.mu.RLock()
	defer server.state.mu.RUnlock()
	image := server.state.images["img_worker"]
	if image.Width != 2 || image.Height != 2 {
		t.Fatalf("expected decoded image dimensions, got %dx%d", image.Width, image.Height)
	}
	if image.PerceptualHash == "" {
		t.Fatalf("expected perceptual hash to be recorded")
	}
	if len(server.state.auditLogs) == 0 {
		t.Fatalf("expected worker audit log")
	}
}

func TestProcessImageAutoFreezesBlockedPerceptualHash(t *testing.T) {
	server := NewServer(config.Load())
	body := tinyPNG(t)
	ctx := context.Background()
	processed, err := server.processor.Process(ctx, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	server.cfg.ModerationBlockedHashes = processed.PerceptualHash
	if err := server.store.PutObject(ctx, storage.PutObjectInput{
		Key:         "original/img_blocked.png",
		Body:        bytes.NewReader(body),
		ContentType: "image/png",
		Private:     true,
	}); err != nil {
		t.Fatal(err)
	}
	server.state.mu.Lock()
	server.state.users["usr_blocked"] = User{ID: "usr_blocked", Email: "blocked@example.com", PlanSlug: "go", Status: "active"}
	server.state.images["img_blocked"] = Image{
		ID:          "image_blocked",
		PublicID:    "img_blocked",
		UserID:      "usr_blocked",
		Filename:    "blocked.png",
		ObjectKey:   "original/img_blocked.png",
		ContentType: "image/png",
		Bytes:       int64(len(body)),
		Status:      "active",
	}
	server.state.mu.Unlock()

	if err := server.ProcessTask(ctx, queue.Task{Type: "image.process", Payload: map[string]any{"public_id": "img_blocked"}}); err != nil {
		t.Fatalf("process blocked image task failed: %v", err)
	}
	server.state.mu.RLock()
	defer server.state.mu.RUnlock()
	image := server.state.images["img_blocked"]
	if image.Status != "frozen" || !strings.Contains(image.ModerationReason, "违规特征库") {
		t.Fatalf("expected auto-frozen image, got status=%s reason=%s", image.Status, image.ModerationReason)
	}
	if len(server.state.riskEvents) == 0 {
		t.Fatalf("expected moderation risk event")
	}
	foundAudit := false
	for _, log := range server.state.auditLogs {
		if log.Action == "image.moderation.freeze" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("expected moderation freeze audit log")
	}
	rec := jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/images?status=frozen&q=img_blocked", nil, config.Load().AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("filtered admin images expected frozen match, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBackupImportValidatesManifest(t *testing.T) {
	server := NewServer(config.Load())
	token, _, code := registerTestUser(t, server, "backup@example.com")
	verifyTestUser(t, server, token, code)

	rec := rawZipRequest(server, "/api/v1/backups/import", token, map[string]string{"database/users.ndjson": "[]"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing manifest expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = rawZipRequest(server, "/api/v1/backups/import", token, map[string]string{"manifest.json": `{}`, "checksums.sha256": ""})
	if rec.Code != http.StatusOK {
		t.Fatalf("manifest zip expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBackupImportRestoresImageObjects(t *testing.T) {
	server := NewServer(config.Load())
	token, _, _ := registerTestUser(t, server, "restore@example.com")
	imageLine := `{"public_id":"old_img_1","filename":"restore.png","content_type":"image/png","bytes":7,"status":"active","created_at":"2026-05-08T00:00:00Z"}`
	rec := rawZipRequest(server, "/api/v1/backups/import", token, map[string]string{
		"manifest.json":                 `{}`,
		"database/images.ndjson":        imageLine + "\n",
		"objects/old_img_1/restore.png": "PNGDATA",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("restore import expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"restored":1`) {
		t.Fatalf("expected one restored image: %s", rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/images", nil, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "restore.png") {
		t.Fatalf("restored image should be listed, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBackupExportAndValidate(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backups/export", nil)
	req.Header.Set("X-Admin-Token", cfg.AdminToken)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin export expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/backups/import/validate", bytes.NewReader(rec.Body.Bytes()))
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("X-Admin-Token", cfg.AdminToken)
	rec = httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"validated"`) {
		t.Fatalf("admin validate expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGrantInfiniteMaxRequiresReason(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	_, userID, code := registerTestUser(t, server, "grant@example.com")
	verifyTestUser(t, server, "", code)

	rec := jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/grant-plan", map[string]any{
		"plan_slug": "infinite-max",
		"days":      30,
	}, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("grant without reason expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/grant-plan", map[string]any{
		"plan_slug": "infinite-max",
		"days":      30,
		"reason":    "internal seed user",
	}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("grant with reason expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPlanValidationAndUserStatus(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	rec := jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/plans", map[string]any{
		"slug":               "",
		"name":               "Broken",
		"monthly_price_cent": 100,
		"yearly_price_cent":  1000,
		"visibility":         "visible",
		"purchasable":        true,
		"quota":              map[string]any{"storage_bytes": 1024},
	}, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid plan expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/plans", map[string]any{
		"slug":               "team",
		"name":               "Team",
		"monthly_price_cent": 9900,
		"yearly_price_cent":  99900,
		"visibility":         "visible",
		"purchasable":        true,
		"quota":              map[string]any{"storage_bytes": 10 * 1024 * 1024 * 1024, "single_file_bytes": 100 * 1024 * 1024},
	}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid plan expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	_, userID, _ := registerTestUser(t, server, "status@example.com")
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/ban", map[string]any{"reason": "abuse"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"banned"`) {
		t.Fatalf("ban expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/unban", map[string]any{"reason": "appeal accepted"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"active"`) {
		t.Fatalf("unban expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminInviteValidation(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	rec := jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/invites", map[string]any{
		"code":       "max-without-approval",
		"name":       "Max no approval",
		"plan_slug":  "infinite-max",
		"grant_days": 30,
		"status":     "active",
	}, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("infinite-max invite without approval expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/invites", map[string]any{
		"code":                   "max-with-approval",
		"name":                   "Max with approval",
		"plan_slug":              "infinite-max",
		"grant_days":             30,
		"status":                 "active",
		"require_admin_approval": true,
	}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("infinite-max invite with approval expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/invites", map[string]any{
		"code":       "max-with-approval",
		"name":       "Duplicate",
		"plan_slug":  "pro",
		"grant_days": 30,
		"status":     "active",
	}, cfg.AdminToken)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate invite expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHotlinkPolicyUpdate(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	rec := jsonRequestWithAdmin(server, http.MethodPatch, "/api/v1/admin/security/hotlink", map[string]any{
		"allowed_domains":     []string{},
		"blocked_domains":     []string{},
		"allow_empty_referer": false,
	}, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty strict hotlink policy expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPatch, "/api/v1/admin/security/hotlink", map[string]any{
		"allowed_domains":     []string{"HTTPS://Example.COM/", "example.com"},
		"blocked_domains":     []string{"bad.example.com"},
		"allow_empty_referer": false,
	}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid hotlink policy expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"example.com"`) || strings.Contains(rec.Body.String(), "HTTPS://") {
		t.Fatalf("expected normalized domains: %s", rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/security/hotlink", nil, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"allow_empty_referer":false`) {
		t.Fatalf("hotlink get expected saved policy, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminQueueStatusInlineMode(t *testing.T) {
	cfg := config.Load()
	cfg.QueueDriver = "inline"
	server := NewServer(cfg)

	rec := jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/queue/status", nil, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue status expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"driver":"inline"`) || !strings.Contains(rec.Body.String(), `"reachable":false`) {
		t.Fatalf("queue status should expose inline fallback state: %s", rec.Body.String())
	}
}

func TestAdminDeadLettersRequireRedisQueue(t *testing.T) {
	cfg := config.Load()
	cfg.QueueDriver = "inline"
	server := NewServer(cfg)

	rec := jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/queue/dead-letters", nil, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("dead-letter list in inline mode expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/queue/dead-letters/1-0/requeue", nil, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("dead-letter requeue in inline mode expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOrderOperationsSubscriptionExpiryAndAuditFilters(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	_, userID, code := registerTestUser(t, server, "billing-ops@example.com")
	verifyTestUser(t, server, "", code)

	now := time.Now().UTC()
	server.state.mu.Lock()
	server.state.orders["ord_manual_paid"] = Order{
		ID:             "ord_manual_paid",
		UserID:         userID,
		PlanSlug:       "pro",
		BillingCycle:   "monthly",
		AmountCent:     7900,
		Status:         "pending",
		IFPayPaymentID: "pay_manual_paid",
		CreatedAt:      now,
	}
	server.state.orders["ord_refund"] = Order{
		ID:             "ord_refund",
		UserID:         userID,
		PlanSlug:       "plus",
		BillingCycle:   "monthly",
		AmountCent:     2900,
		Status:         "pending",
		IFPayPaymentID: "pay_refund",
		CreatedAt:      now,
	}
	server.state.orders["ord_cancel"] = Order{
		ID:           "ord_cancel",
		UserID:       userID,
		PlanSlug:     "go",
		BillingCycle: "monthly",
		AmountCent:   1200,
		Status:       "pending",
		CreatedAt:    now,
	}
	server.state.mu.Unlock()

	rec := jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_manual_paid/mark-paid", map[string]any{}, cfg.AdminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mark paid without reason expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_manual_paid/mark-paid", map[string]any{"reason": "manual reconcile bank statement"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"paid"`) {
		t.Fatalf("mark paid expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.users[userID].PlanSlug != "pro" || !server.state.subscriptions[userID].CanUpload(time.Now().UTC()) {
		t.Fatalf("mark paid should activate pro subscription")
	}
	firstSubID := server.state.subscriptions[userID].ID
	server.state.mu.RUnlock()
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_manual_paid/mark-paid", map[string]any{"reason": "manual reconcile bank statement"}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent mark paid expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.subscriptions[userID].ID != firstSubID {
		t.Fatalf("idempotent mark paid should not recreate subscription")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/subscription/expire", map[string]any{"reason": "chargeback risk"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"past_due"`) {
		t.Fatalf("expire subscription expected past_due, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.subscriptions[userID].CanUpload(time.Now().UTC()) {
		t.Fatalf("expired subscription should block uploads")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_refund/mark-paid", map[string]any{"reason": "manual reconcile before refund"}, cfg.AdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("second mark paid expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_refund/refund", map[string]any{"reason": "customer refund request"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"refunded"`) {
		t.Fatalf("refund expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_cancel/cancel", map[string]any{"reason": "duplicate checkout"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("cancel expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/audit-logs?q=manual%20reconcile&action=order.mark_paid&limit=1", nil, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":3`) || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("filtered audit logs expected two mark-paid records, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIFPayWebhookOrderLifecycle(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	_, userID, code := registerTestUser(t, server, "webhook-billing@example.com")
	verifyTestUser(t, server, "", code)

	now := time.Now().UTC()
	server.state.mu.Lock()
	server.state.orders["ord_success"] = Order{
		ID:             "ord_success",
		UserID:         userID,
		PlanSlug:       "plus",
		BillingCycle:   "monthly",
		AmountCent:     2900,
		Status:         "pending",
		IFPayPaymentID: "pay_success",
		CreatedAt:      now,
	}
	server.state.orders["ord_failed"] = Order{
		ID:             "ord_failed",
		UserID:         userID,
		PlanSlug:       "go",
		BillingCycle:   "monthly",
		AmountCent:     1200,
		Status:         "pending",
		IFPayPaymentID: "pay_failed",
		CreatedAt:      now,
	}
	server.state.orders["ord_cancel"] = Order{
		ID:             "ord_cancel",
		UserID:         userID,
		PlanSlug:       "pro",
		BillingCycle:   "monthly",
		AmountCent:     7900,
		Status:         "pending",
		IFPayPaymentID: "pay_cancel",
		CreatedAt:      now,
	}
	server.state.orders["ord_refund"] = Order{
		ID:             "ord_refund",
		UserID:         userID,
		PlanSlug:       "ultra",
		BillingCycle:   "yearly",
		AmountCent:     120000,
		Status:         "pending",
		IFPayPaymentID: "pay_refund_webhook",
		CreatedAt:      now,
	}
	server.state.mu.Unlock()

	rec := jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_success",
		"event_type":  "payment.succeeded",
		"resource_id": "pay_success",
		"payload": map[string]any{
			"order_id": "ord_success",
		},
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("payment success webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.orders["ord_success"].Status != "paid" || server.state.users[userID].PlanSlug != "plus" {
		t.Fatalf("payment success should mark paid and activate plan")
	}
	firstSubID := server.state.subscriptions[userID].ID
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_success_duplicate",
		"event_type":  "payment.succeeded",
		"resource_id": "pay_success",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("second success webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.subscriptions[userID].ID != firstSubID {
		t.Fatalf("idempotent success webhook should not recreate active subscription")
	}
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_failed",
		"event_type":  "payment.failed",
		"resource_id": "pay_failed",
		"payload": map[string]any{
			"failure_reason": "insufficient balance",
		},
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("payment failed webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_cancel",
		"event_type":  "payment.canceled",
		"resource_id": "pay_cancel",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("payment cancel webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_refund_activate",
		"event_type":  "payment.succeeded",
		"resource_id": "pay_refund_webhook",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("refund setup success webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_refund",
		"event_type":  "payment.refunded",
		"resource_id": "pay_refund_webhook",
		"payload": map[string]any{
			"refund_reason": "chargeback",
		},
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("payment refund webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if order := server.state.orders["ord_failed"]; order.Status != "failed" || order.FailedAt == nil {
		t.Fatalf("failed webhook should mark order failed with timestamp: %+v", order)
	}
	if order := server.state.orders["ord_cancel"]; order.Status != "cancelled" || order.CancelledAt == nil {
		t.Fatalf("cancel webhook should mark order cancelled with timestamp: %+v", order)
	}
	if order := server.state.orders["ord_refund"]; order.Status != "refunded" || order.RefundedAt == nil {
		t.Fatalf("refund webhook should mark order refunded with timestamp: %+v", order)
	}
	if sub := server.state.subscriptions[userID]; sub.Status != domain.SubscriptionPastDue || sub.CanUpload(time.Now().UTC()) {
		t.Fatalf("refund webhook should retain subscription read-only, got %+v", sub)
	}
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_unmatched",
		"event_type":  "payment.succeeded",
		"resource_id": "pay_missing",
	}, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"matched":false`) {
		t.Fatalf("unmatched webhook expected matched=false, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFullUserAndAdminIntegrationFlow(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	token, userID, code := registerTestUser(t, server, "full-flow@example.com")
	verifyTestUser(t, server, token, code)

	rec := jsonRequest(server, http.MethodPost, "/api/v1/checkout/ifpay", map[string]any{
		"plan_slug":     "plus",
		"billing_cycle": "monthly",
		"access_token":  "ifpay_dev_access_test",
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("checkout expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var checkoutPayload struct {
		Data struct {
			Order   Order `json:"order"`
			Payment struct {
				PaymentID string `json:"payment_id"`
			} `json:"payment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &checkoutPayload); err != nil {
		t.Fatal(err)
	}
	if checkoutPayload.Data.Order.ID == "" || checkoutPayload.Data.Payment.PaymentID == "" {
		t.Fatalf("checkout should return order and payment: %s", rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodPost, "/api/v1/ifpay/webhooks/payments", map[string]any{
		"event_id":    "evt_full_flow_paid",
		"event_type":  "payment.succeeded",
		"resource_id": checkoutPayload.Data.Payment.PaymentID,
		"payload": map[string]any{
			"order_id": checkoutPayload.Data.Order.ID,
		},
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("payment webhook expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.users[userID].PlanSlug != "plus" || server.state.orders[checkoutPayload.Data.Order.ID].Status != "paid" {
		t.Fatalf("webhook should activate plus plan and mark order paid")
	}
	server.state.mu.RUnlock()

	rec = multipartUploadWithBearer(server, "/api/v1/images", token, "full-flow.png", tinyPNG(t), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var uploadPayload struct {
		Data struct {
			Image Image `json:"image"`
			Links struct {
				Raw string `json:"raw"`
			} `json:"links"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatal(err)
	}
	publicID := uploadPayload.Data.Image.PublicID
	if publicID == "" || uploadPayload.Data.Links.Raw == "" {
		t.Fatalf("upload should return image and raw link: %s", rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodGet, "/api/v1/images", nil, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), publicID) {
		t.Fatalf("image list should include upload, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/images/"+publicID+"/sign", nil, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sig=") {
		t.Fatalf("signed URL expected sig parameter, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPatch, "/api/v1/images/"+publicID+"/privacy", map[string]any{"private": false}, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"private":false`) {
		t.Fatalf("privacy update expected public image, got %d: %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/i/"+publicID, nil)
	imgRec := httptest.NewRecorder()
	server.router.ServeHTTP(imgRec, req)
	if imgRec.Code != http.StatusOK || imgRec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("public image serve expected png 200, got %d %q", imgRec.Code, imgRec.Header().Get("Content-Type"))
	}

	rec = jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/images?q="+publicID, nil, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("admin image search expected uploaded image, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/images/"+publicID+"/freeze", map[string]any{"reason": "integration test"}, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"frozen"`) {
		t.Fatalf("admin freeze expected frozen image, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodDelete, "/api/v1/admin/images/"+publicID, nil, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":true`) {
		t.Fatalf("admin delete expected success, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequestWithAdmin(server, http.MethodGet, "/api/v1/admin/audit-logs?q="+publicID, nil, cfg.AdminToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "image.freeze") || !strings.Contains(rec.Body.String(), "image.delete") {
		t.Fatalf("audit logs should include image freeze/delete, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/backups/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	backupRec := httptest.NewRecorder()
	server.router.ServeHTTP(backupRec, req)
	if backupRec.Code != http.StatusOK || backupRec.Header().Get("Content-Type") != "application/zip" || backupRec.Body.Len() == 0 {
		t.Fatalf("user backup export expected zip 200, got %d %q len=%d", backupRec.Code, backupRec.Header().Get("Content-Type"), backupRec.Body.Len())
	}
}

func TestCheckoutRequiresVerifiedUserAndValidBillingCycle(t *testing.T) {
	server := NewServer(config.Load())
	token, _, code := registerTestUser(t, server, "checkout@example.com")
	rec := jsonRequest(server, http.MethodPost, "/api/v1/checkout/ifpay", map[string]any{
		"plan_slug":     "plus",
		"billing_cycle": "monthly",
	}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unverified checkout expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	verifyTestUser(t, server, token, code)
	rec = jsonRequest(server, http.MethodPost, "/api/v1/checkout/ifpay", map[string]any{
		"plan_slug":     "plus",
		"billing_cycle": "weekly",
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid billing cycle expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/checkout/ifpay", map[string]any{
		"plan_slug":     "plus",
		"billing_cycle": "monthly",
		"access_token":  "ifpay_dev_access_test",
	}, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("checkout expected pending order, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/orders", nil, token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"plan_slug":"plus"`) {
		t.Fatalf("list orders expected checkout order, got %d: %s", rec.Code, rec.Body.String())
	}
}

func jsonRequest(server *Server, method, path string, body any, token string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

func jsonRequestWithAdmin(server *Server, method, path string, body any, token string) *httptest.ResponseRecorder {
	recorderBody, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(recorderBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", token)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

type failingPersister struct{}

func (failingPersister) Ping(context.Context) error {
	return nil
}

func (failingPersister) Load(context.Context, *memoryState) error {
	return nil
}

func (failingPersister) Save(context.Context, *memoryState) error {
	return errors.New("forced persistence failure")
}

func (failingPersister) Close() error {
	return nil
}

func TestAdminPersistenceFailureRollsBackMutations(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	_, userID, code := registerTestUser(t, server, "rollback@example.com")
	verifyTestUser(t, server, "", code)

	now := time.Now().UTC()
	server.state.mu.Lock()
	server.state.orders["ord_rollback"] = Order{
		ID:           "ord_rollback",
		UserID:       userID,
		PlanSlug:     "pro",
		BillingCycle: "monthly",
		AmountCent:   7900,
		Status:       "pending",
		CreatedAt:    now,
	}
	server.state.images["img_rollback"] = Image{
		ID:        "img_rollback",
		PublicID:  "img_rollback",
		UserID:    userID,
		Filename:  "rollback.png",
		ObjectKey: "objects/rollback.png",
		Bytes:     128,
		Status:    "active",
		CreatedAt: now,
	}
	server.state.usage[userID] = domain.Usage{StorageBytes: 128}
	originalSub := server.state.subscriptions[userID]
	originalUser := server.state.users[userID]
	originalHotlink := server.state.hotlink
	server.state.mu.Unlock()

	if err := server.store.PutObject(context.Background(), storage.PutObjectInput{
		Key:         "objects/rollback.png",
		Body:        strings.NewReader("image-bytes"),
		ContentType: "image/png",
	}); err != nil {
		t.Fatal(err)
	}

	server.persister = failingPersister{}

	rec := jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/plans", map[string]any{
		"slug":               "rollback-plan",
		"name":               "Rollback Plan",
		"monthly_price_cent": 1000,
		"yearly_price_cent":  10000,
		"visibility":         "visible",
		"purchasable":        true,
		"quota":              map[string]any{"storage_bytes": 1024, "single_file_bytes": 512},
	}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("plan create with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if _, exists := domain.FindPlan(server.state.plans, "rollback-plan"); exists {
		t.Fatalf("failed plan create should roll back")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/invites", map[string]any{
		"code":       "rollback-invite",
		"name":       "Rollback Invite",
		"plan_slug":  "pro",
		"grant_days": 30,
		"status":     "active",
	}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("invite create with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if _, exists := server.state.invites["rollback-invite"]; exists {
		t.Fatalf("failed invite create should roll back")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/grant-plan", map[string]any{
		"plan_slug": "infinite-max",
		"days":      30,
		"reason":    "rollback check",
	}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("grant plan with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if got := server.state.users[userID].PlanSlug; got != originalUser.PlanSlug {
		t.Fatalf("failed grant should keep original plan %q, got %q", originalUser.PlanSlug, got)
	}
	if server.state.subscriptions[userID] != originalSub {
		t.Fatalf("failed grant should keep original subscription")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/orders/ord_rollback/mark-paid", map[string]any{"reason": "rollback check"}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("mark paid with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.orders["ord_rollback"].Status != "pending" || server.state.users[userID].PlanSlug != originalUser.PlanSlug {
		t.Fatalf("failed mark paid should roll back order and user")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPatch, "/api/v1/admin/security/hotlink", map[string]any{
		"allowed_domains":     []string{"rollback.example.com"},
		"blocked_domains":     []string{"bad.example.com"},
		"allow_empty_referer": false,
	}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("hotlink update with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if strings.Join(server.state.hotlink.AllowedDomains, ",") != strings.Join(originalHotlink.AllowedDomains, ",") ||
		strings.Join(server.state.hotlink.BlockedDomains, ",") != strings.Join(originalHotlink.BlockedDomains, ",") ||
		server.state.hotlink.AllowEmptyReferer != originalHotlink.AllowEmptyReferer {
		t.Fatalf("failed hotlink update should roll back")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/images/img_rollback/freeze", map[string]any{"reason": "rollback check"}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("image freeze with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.images["img_rollback"].Status != "active" {
		t.Fatalf("failed freeze should roll back image status")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodDelete, "/api/v1/admin/images/img_rollback", nil, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("image delete with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.images["img_rollback"].Status != "active" || server.state.usage[userID].StorageBytes != 128 {
		t.Fatalf("failed delete should roll back image and usage")
	}
	server.state.mu.RUnlock()
	if _, _, err := server.store.GetObject(context.Background(), "objects/rollback.png"); err != nil {
		t.Fatalf("failed delete should not remove object before persistence succeeds: %v", err)
	}

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/ban", map[string]any{"reason": "rollback check"}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ban with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.users[userID].Status != originalUser.Status {
		t.Fatalf("failed ban should roll back user status")
	}
	server.state.mu.RUnlock()

	rec = jsonRequestWithAdmin(server, http.MethodPost, "/api/v1/admin/users/"+userID+"/subscription/expire", map[string]any{"reason": "rollback check"}, cfg.AdminToken)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expire subscription with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.subscriptions[userID] != originalSub || server.state.users[userID].PlanSlug != originalUser.PlanSlug {
		t.Fatalf("failed subscription expire should roll back")
	}
	server.state.mu.RUnlock()
}

func TestUserPersistenceFailureRollsBackMutations(t *testing.T) {
	server := NewServer(config.Load())
	token, userID, code := registerTestUser(t, server, "user-rollback@example.com")
	verifyTestUser(t, server, token, code)

	server.state.mu.RLock()
	originalUser := server.state.users[userID]
	originalUsage := server.state.usage[userID]
	server.state.mu.RUnlock()

	server.persister = failingPersister{}
	rec := jsonRequest(server, http.MethodPatch, "/api/v1/settings/profile", map[string]any{"nickname": "should rollback"}, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("profile update with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.users[userID].Nickname != originalUser.Nickname {
		t.Fatalf("failed profile update should roll back nickname")
	}
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodPost, "/api/v1/api-keys", map[string]any{
		"name":   "rollback-key",
		"scopes": []string{"images:read", "images:write"},
	}, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("API key create with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if len(server.state.apiKeys[userID]) != 0 {
		t.Fatalf("failed API key create should roll back")
	}
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodPost, "/api/v1/checkout/ifpay", map[string]any{
		"plan_slug":     "plus",
		"billing_cycle": "monthly",
		"access_token":  "ifpay_dev_access_test",
	}, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("checkout with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if len(server.state.orders) != 0 {
		t.Fatalf("failed checkout should roll back order")
	}
	server.state.mu.RUnlock()

	rec = multipartUploadWithBearer(server, "/api/v1/images", token, "rollback-upload.png", tinyPNG(t), true)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("upload with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if len(server.state.images) != 0 || server.state.usage[userID] != originalUsage {
		t.Fatalf("failed upload should roll back images and usage")
	}
	server.state.mu.RUnlock()

	server.persister = nil
	rec = multipartUploadWithBearer(server, "/api/v1/images", token, "rollback-delete.png", tinyPNG(t), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup upload expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var uploadPayload struct {
		Data struct {
			Image Image `json:"image"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatal(err)
	}
	publicID := uploadPayload.Data.Image.PublicID
	objectKey := uploadPayload.Data.Image.ObjectKey
	server.state.mu.RLock()
	usageAfterUpload := server.state.usage[userID]
	server.state.mu.RUnlock()

	server.persister = failingPersister{}
	rec = jsonRequest(server, http.MethodPatch, "/api/v1/images/"+publicID+"/privacy", map[string]any{"private": false}, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("privacy update with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if !server.state.images[publicID].Private {
		t.Fatalf("failed privacy update should roll back private flag")
	}
	server.state.mu.RUnlock()

	rec = jsonRequest(server, http.MethodDelete, "/api/v1/images/"+publicID, nil, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("image delete with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	if server.state.images[publicID].Status != "active" || server.state.usage[userID] != usageAfterUpload {
		t.Fatalf("failed delete should roll back image status and usage")
	}
	server.state.mu.RUnlock()
	if _, _, err := server.store.GetObject(context.Background(), objectKey); err != nil {
		t.Fatalf("failed delete should keep object available: %v", err)
	}
}

func registerTestUser(t *testing.T, server *Server, email string) (string, string, string) {
	t.Helper()
	rec := jsonRequest(server, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":    email,
		"password": "secret123",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data struct {
			Token string `json:"token"`
			User  struct {
				ID string `json:"id"`
			} `json:"user"`
			Code string `json:"dev_email_verification_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.Token, payload.Data.User.ID, payload.Data.Code
}

func verifyTestUser(t *testing.T, server *Server, token, code string) {
	t.Helper()
	userID := currentOnlyUserID(t, server)
	rec := jsonRequest(server, http.MethodPost, "/api/v1/auth/verify-email", map[string]any{
		"user_id": userID,
		"code":    code,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func currentOnlyUserID(t *testing.T, server *Server) string {
	t.Helper()
	var userID string
	server.state.mu.RLock()
	defer server.state.mu.RUnlock()
	for id := range server.state.users {
		userID = id
	}
	if userID == "" {
		t.Fatalf("missing test user")
	}
	return userID
}

func multipartUpload(server *Server, path, apiKey, filename string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = part.Write(data)
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

func multipartUploadWithBearer(server *Server, path, token, filename string, data []byte, private bool) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = part.Write(data)
	if private {
		_ = writer.WriteField("private", "true")
	}
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

func rawZipRequest(server *Server, path, token string, files map[string]string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAdminPlansExposeInfiniteMax(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/plans", nil)
	req.Header.Set("X-Admin-Token", cfg.AdminToken)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "infinite-max") {
		t.Fatalf("admin API should expose hidden infinite-max")
	}
}

func TestAdminBootstrapLoginAndSessionAuth(t *testing.T) {
	cfg := config.Load()
	server := NewServer(cfg)

	rec := jsonRequest(server, http.MethodGet, "/api/v1/admin/auth/status", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"setup_required":true`) {
		t.Fatalf("admin status should require setup, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/start", map[string]any{
		"email":        "root@yuexiang.local",
		"display_name": "Root Admin",
		"password":     "super-secret-admin",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap start expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var startPayload struct {
		Data struct {
			SetupToken     string `json:"setup_token"`
			ManualEntryKey string `json:"manual_entry_key"`
			QRCodeDataURL  string `json:"qr_code_data_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startPayload); err != nil {
		t.Fatal(err)
	}
	if startPayload.Data.SetupToken == "" || startPayload.Data.ManualEntryKey == "" || !strings.HasPrefix(startPayload.Data.QRCodeDataURL, "data:image/png;base64,") {
		t.Fatalf("bootstrap start should return setup token and QR code: %s", rec.Body.String())
	}
	code, err := totp.GenerateCode(startPayload.Data.ManualEntryKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/complete", map[string]any{
		"setup_token": startPayload.Data.SetupToken,
		"totp_code":   code,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap complete expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var authPayload struct {
		Data struct {
			Token string    `json:"token"`
			Admin AdminUser `json:"admin"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &authPayload); err != nil {
		t.Fatal(err)
	}
	if authPayload.Data.Token == "" || authPayload.Data.Admin.Email != "root@yuexiang.local" {
		t.Fatalf("bootstrap complete should login admin: %s", rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/auth/status", nil, authPayload.Data.Token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"setup_required":false`) || !strings.Contains(rec.Body.String(), `"email":"root@yuexiang.local"`) {
		t.Fatalf("admin status should return current admin, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/plans", nil, authPayload.Data.Token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "infinite-max") {
		t.Fatalf("admin session should access plans, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/logout", nil, authPayload.Data.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin logout expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/plans", nil, authPayload.Data.Token)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logged out admin session should be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	code, err = totp.GenerateCode(startPayload.Data.ManualEntryKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{
		"email":     "root@yuexiang.local",
		"password":  "super-secret-admin",
		"totp_code": code,
	}, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"token":"yx_admin_`) {
		t.Fatalf("admin login expected token, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/login", map[string]any{
		"email":     "root@yuexiang.local",
		"password":  "super-secret-admin",
		"totp_code": "000000",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin login with invalid totp expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBootstrapPersistenceFailureRollsBack(t *testing.T) {
	server := NewServer(config.Load())

	rec := jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/start", map[string]any{
		"email":        "root@yuexiang.local",
		"display_name": "Root Admin",
		"password":     "super-secret-admin",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap start expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var startPayload struct {
		Data struct {
			SetupToken     string `json:"setup_token"`
			ManualEntryKey string `json:"manual_entry_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startPayload); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(startPayload.Data.ManualEntryKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	server.persister = failingPersister{}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/complete", map[string]any{
		"setup_token": startPayload.Data.SetupToken,
		"totp_code":   code,
	}, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("bootstrap complete with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	server.state.mu.RLock()
	adminCount := len(server.state.admins)
	sessionCount := len(server.state.adminSessions)
	server.state.mu.RUnlock()
	if adminCount != 0 || sessionCount != 0 {
		t.Fatalf("failed bootstrap should roll back admin/session, got admins=%d sessions=%d", adminCount, sessionCount)
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/auth/status", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"setup_required":true`) {
		t.Fatalf("status should still require setup after rollback, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSessionSurvivesRolledBackMutation(t *testing.T) {
	server := NewServer(config.Load())

	rec := jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/start", map[string]any{
		"email":        "root@yuexiang.local",
		"display_name": "Root Admin",
		"password":     "super-secret-admin",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap start expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var startPayload struct {
		Data struct {
			SetupToken     string `json:"setup_token"`
			ManualEntryKey string `json:"manual_entry_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startPayload); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(startPayload.Data.ManualEntryKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/auth/bootstrap/complete", map[string]any{
		"setup_token": startPayload.Data.SetupToken,
		"totp_code":   code,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap complete expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var authPayload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &authPayload); err != nil {
		t.Fatal(err)
	}

	server.persister = failingPersister{}
	rec = jsonRequest(server, http.MethodPost, "/api/v1/admin/plans", map[string]any{
		"slug":               "rollback-session-plan",
		"name":               "Rollback Session Plan",
		"monthly_price_cent": 1000,
		"yearly_price_cent":  10000,
		"visibility":         "visible",
		"purchasable":        true,
		"quota":              map[string]any{"storage_bytes": 1024, "single_file_bytes": 512},
	}, authPayload.Data.Token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("plan create with failed persistence expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	server.persister = nil
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/auth/status", nil, authPayload.Data.Token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"email":"root@yuexiang.local"`) {
		t.Fatalf("rolled back mutation should not invalidate admin session, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = jsonRequest(server, http.MethodGet, "/api/v1/admin/plans", nil, authPayload.Data.Token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "infinite-max") {
		t.Fatalf("admin session should still access plans after rolled back mutation, got %d: %s", rec.Code, rec.Body.String())
	}
}
