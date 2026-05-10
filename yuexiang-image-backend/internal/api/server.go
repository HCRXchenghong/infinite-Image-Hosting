package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yuexiang/image-backend/internal/backup"
	"github.com/yuexiang/image-backend/internal/config"
	"github.com/yuexiang/image-backend/internal/domain"
	"github.com/yuexiang/image-backend/internal/ifpay"
	"github.com/yuexiang/image-backend/internal/imageproc"
	"github.com/yuexiang/image-backend/internal/mail"
	"github.com/yuexiang/image-backend/internal/queue"
	"github.com/yuexiang/image-backend/internal/security"
	"github.com/yuexiang/image-backend/internal/storage"
)

type Server struct {
	cfg       config.Config
	router    *gin.Engine
	state     *memoryState
	store     storage.ObjectStore
	persister statePersister
	tasks     queue.Queue
	mailer    mail.Sender
	processor imageproc.LibvipsWorker
	ifpay     ifpay.Client
	rateMu    sync.Mutex
	rate      map[string]rateWindow
	metricsMu sync.Mutex
	requests  map[requestStatKey]int64
	persistMu sync.Mutex
}

type rateWindow struct {
	Count      int
	WindowEnds time.Time
}

type requestStatKey struct {
	Method string
	Route  string
	Status int
}

type memoryState struct {
	mu             sync.RWMutex
	plans          []domain.Plan
	users          map[string]User
	admins         map[string]AdminUser
	adminSessions  map[string]AdminSession
	adminBootstrap map[string]PendingAdminBootstrap
	images         map[string]Image
	albums         map[string]Album
	apiKeys        map[string][]APIKey
	orders         map[string]Order
	subscriptions  map[string]domain.Subscription
	usage          map[string]domain.Usage
	invites        map[string]domain.InviteCampaign
	redemptions    []InviteRedemption
	webhooks       map[string]bool
	riskEvents     []RiskEvent
	auditLogs      []AuditLog
	hotlink        HotlinkConfig
	integrations   IntegrationConfig
}

type HotlinkConfig struct {
	AllowedDomains    []string  `json:"allowed_domains"`
	BlockedDomains    []string  `json:"blocked_domains"`
	AllowEmptyReferer bool      `json:"allow_empty_referer"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type IntegrationConfig struct {
	IFPayBaseURL          string    `json:"ifpay_base_url"`
	IFPayPartnerAppID     string    `json:"ifpay_partner_app_id"`
	IFPayClientID         string    `json:"ifpay_client_id"`
	IFPayClientSecret     string    `json:"ifpay_client_secret,omitempty"`
	IFPayPrivateKeyPEM    string    `json:"ifpay_private_key_pem,omitempty"`
	IFPayPublicKeyPEM     string    `json:"ifpay_public_key_pem,omitempty"`
	IFPayWebhookPublicKey string    `json:"ifpay_webhook_public_key_pem,omitempty"`
	IFPayRedirectURI      string    `json:"ifpay_redirect_uri"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type User struct {
	ID                    string    `json:"id"`
	Email                 string    `json:"email"`
	Nickname              string    `json:"nickname"`
	AvatarURL             string    `json:"avatar_url,omitempty"`
	EmailVerified         bool      `json:"email_verified"`
	OAuthBound            bool      `json:"oauth_bound"`
	PlanSlug              string    `json:"plan_slug"`
	PasswordHash          string    `json:"-"`
	EmailVerificationCode string    `json:"-"`
	PasswordResetCode     string    `json:"-"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
}

type Image struct {
	ID               string              `json:"id"`
	PublicID         string              `json:"public_id"`
	UserID           string              `json:"user_id"`
	Filename         string              `json:"filename"`
	ObjectKey        string              `json:"object_key"`
	ContentType      string              `json:"content_type"`
	Bytes            int64               `json:"bytes"`
	Private          bool                `json:"private"`
	Width            int                 `json:"width"`
	Height           int                 `json:"height"`
	PerceptualHash   string              `json:"perceptual_hash"`
	Variants         []imageproc.Variant `json:"variants"`
	Status           string              `json:"status"`
	ModerationReason string              `json:"moderation_reason,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
}

type Album struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Private   bool      `json:"private"`
	CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Secret     string     `json:"secret,omitempty"`
	SecretHash string     `json:"-"`
	Scopes     []string   `json:"scopes"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type AuthIdentity struct {
	UserID string
	Source string
	Scopes []string
}

type AdminIdentity struct {
	AdminID string
	Email   string
	Name    string
	Role    string
}

type AdminUser struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	PasswordHash string     `json:"-"`
	TOTPSecret   string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type AdminSession struct {
	ID        string    `json:"id"`
	AdminID   string    `json:"admin_id"`
	TokenHash string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IP        string    `json:"ip,omitempty"`
}

type PendingAdminBootstrap struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	TOTPSecret   string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Order struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	PlanSlug       string     `json:"plan_slug"`
	BillingCycle   string     `json:"billing_cycle"`
	AmountCent     int64      `json:"amount_cent"`
	Status         string     `json:"status"`
	IFPayPaymentID string     `json:"ifpay_payment_id"`
	IFPaySubMethod string     `json:"ifpay_sub_method"`
	CreatedAt      time.Time  `json:"created_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	FailedAt       *time.Time `json:"failed_at,omitempty"`
	CancelledAt    *time.Time `json:"cancelled_at,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	OperatorNote   string     `json:"operator_note,omitempty"`
}

type InviteRedemption struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	IP        string    `json:"ip"`
	DeviceID  string    `json:"device_id"`
	PlanSlug  string    `json:"plan_slug"`
	CreatedAt time.Time `json:"created_at"`
}

type RiskEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	IP        string    `json:"ip,omitempty"`
	Referer   string    `json:"referer,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        string         `json:"id"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Target    string         `json:"target"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func NewServer(cfg config.Config) *Server {
	if err := cfg.ValidateProduction(); err != nil {
		panic(fmt.Sprintf("invalid production config: %v", err))
	}
	state := &memoryState{
		plans:          domain.DefaultPlans(),
		users:          map[string]User{},
		admins:         map[string]AdminUser{},
		adminSessions:  map[string]AdminSession{},
		adminBootstrap: map[string]PendingAdminBootstrap{},
		images:         map[string]Image{},
		albums:         map[string]Album{},
		apiKeys:        map[string][]APIKey{},
		orders:         map[string]Order{},
		subscriptions:  map[string]domain.Subscription{},
		usage:          map[string]domain.Usage{},
		invites:        seedInviteCampaigns(),
		webhooks:       map[string]bool{},
		hotlink: HotlinkConfig{
			AllowedDomains:    splitCSV(cfg.AllowedRefererDomains),
			BlockedDomains:    splitCSV(cfg.BlockedRefererDomains),
			AllowEmptyReferer: cfg.AllowEmptyReferer,
			UpdatedAt:         time.Now().UTC(),
		},
		integrations: IntegrationConfig{
			IFPayBaseURL:          cfg.IFPayBaseURL,
			IFPayPartnerAppID:     cfg.IFPayPartnerAppID,
			IFPayClientID:         cfg.IFPayClientID,
			IFPayClientSecret:     cfg.IFPayClientSecret,
			IFPayPrivateKeyPEM:    cfg.IFPayPrivateKeyPEM,
			IFPayPublicKeyPEM:     cfg.IFPayPublicKeyPEM,
			IFPayWebhookPublicKey: cfg.IFPayWebhookPublicKey,
			IFPayRedirectURI:      cfg.IFPayRedirectURI,
			UpdatedAt:             time.Now().UTC(),
		},
	}
	objectStore := storage.ObjectStore(storage.NewMemoryObjectStore())
	if cfg.StorageDriver == "s3" {
		s3Store, err := storage.NewS3CompatibleObjectStore(storage.S3CompatibleConfig{
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			ForcePathStyle: cfg.S3ForcePathStyle,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
		})
		if err != nil {
			panic(fmt.Sprintf("initialize s3 object store: %v", err))
		}
		objectStore = s3Store
	}
	taskQueue := queue.Queue(queue.InlineQueue{})
	if cfg.QueueDriver == "redis" {
		taskQueue = queue.NewRedisQueue(queue.RedisConfig{
			Addr:             cfg.RedisAddr,
			DB:               cfg.RedisDB,
			Stream:           cfg.QueueStream,
			DeadLetterStream: cfg.QueueDeadLetterStream,
		})
	}
	mailer := mail.Sender(mail.ConsoleSender{})
	if cfg.SMTPHost != "" {
		mailer = mail.SMTPSender{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		}
	}
	server := &Server{
		cfg:       cfg,
		state:     state,
		store:     objectStore,
		tasks:     taskQueue,
		mailer:    mailer,
		processor: imageproc.LibvipsWorker{},
		ifpay: ifpay.Client{
			BaseURL:       cfg.IFPayBaseURL,
			PartnerAppID:  cfg.IFPayPartnerAppID,
			ClientID:      cfg.IFPayClientID,
			ClientSecret:  cfg.IFPayClientSecret,
			PrivateKeyPEM: cfg.IFPayPrivateKeyPEM,
		},
		rate:     map[string]rateWindow{},
		requests: map[requestStatKey]int64{},
	}
	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		persister, err := newPostgresStateStore(ctx, cfg.DatabaseURL)
		if err != nil {
			panic(fmt.Sprintf("initialize postgres state store: %v", err))
		}
		server.persister = persister
		if err := server.persister.Load(ctx, server.state); err != nil {
			panic(fmt.Sprintf("load postgres state: %v", err))
		}
		if err := server.persister.Save(ctx, server.state); err != nil {
			panic(fmt.Sprintf("seed postgres state: %v", err))
		}
	}
	router := gin.New()
	router.Use(gin.Recovery(), requestID(), securityHeaders(cfg.AppEnv), server.requestMetrics(), requestLogger(), cors(cfg.CORSAllowedOrigins), server.rateLimit())
	server.router = router
	server.routes()
	return server
}

func (s *Server) Run() error {
	return s.router.Run(s.cfg.HTTPAddr)
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) Addr() string {
	return s.cfg.HTTPAddr
}

func (s *Server) persistState(ctx context.Context) error {
	if s.persister == nil {
		return nil
	}
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	saveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.persister.Save(saveCtx, s.state)
}

func (s *Server) persistOrFail(c *gin.Context) bool {
	if err := s.persistState(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return false
	}
	return true
}

func (s *Server) persistOrRollback(c *gin.Context, snap stateSnapshot) bool {
	if err := s.persistState(c.Request.Context()); err != nil {
		restoreStateSnapshot(s.state, snap)
		fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return false
	}
	return true
}

func (s *Server) persistOrRollbackWithCleanup(c *gin.Context, snap stateSnapshot, cleanup func()) bool {
	if err := s.persistState(c.Request.Context()); err != nil {
		restoreStateSnapshot(s.state, snap)
		if cleanup != nil {
			cleanup()
		}
		fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return false
	}
	return true
}

func (s *Server) restoreSnapshotAndPersist(c *gin.Context, snap stateSnapshot) bool {
	restoreStateSnapshot(s.state, snap)
	if err := s.persistState(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return false
	}
	return true
}

func (s *Server) cleanupObjects(ctx context.Context, keys []string) {
	seen := map[string]bool{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		_ = s.store.DeleteObject(ctx, key)
	}
}

func (s *Server) readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := gin.H{
		"config":   "ok",
		"database": "skipped",
		"redis":    "skipped",
		"storage":  "ok",
	}
	ready := true
	if err := s.store.Ping(ctx); err != nil {
		checks["storage"] = err.Error()
		ready = false
	}
	if s.cfg.DatabaseURL != "" {
		if s.persister == nil {
			checks["database"] = "not_configured"
			ready = false
		} else if err := s.persister.Ping(ctx); err != nil {
			checks["database"] = err.Error()
			ready = false
		} else {
			checks["database"] = "ok"
		}
	}
	if s.cfg.QueueDriver == "redis" {
		if _, err := queue.InspectRedis(ctx, queue.RedisConfig{
			Addr:             s.cfg.RedisAddr,
			DB:               s.cfg.RedisDB,
			Stream:           s.cfg.QueueStream,
			DeadLetterStream: s.cfg.QueueDeadLetterStream,
		}, s.cfg.QueueGroup); err != nil {
			checks["redis"] = err.Error()
			ready = false
		} else {
			checks["redis"] = "ok"
		}
	}
	status := http.StatusOK
	state := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	c.JSON(status, gin.H{
		"ok":    ready,
		"data":  gin.H{"status": state, "checks": checks, "time": time.Now().UTC()},
		"error": nil,
	})
}

func (s *Server) routes() {
	r := s.router
	r.GET("/healthz", func(c *gin.Context) {
		ok(c, gin.H{"status": "ok", "app": "yuexiang-image", "time": time.Now().UTC()})
	})
	r.GET("/readyz", s.readyz)
	r.GET("/metrics", s.metrics)
	r.GET("/docs/openapi.yaml", serveDocFile("openapi.yaml", "application/yaml; charset=utf-8"))
	r.GET("/docs/api.md", serveDocFile("api.md", "text/markdown; charset=utf-8"))
	r.GET("/i/:public_id/:variant", s.serveImageVariant)
	r.GET("/i/:public_id", s.serveImage)

	v1 := r.Group("/api/v1")
	v1.GET("/plans", s.listPublicPlans)
	v1.POST("/auth/register", s.register)
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/forgot-password", s.forgotPassword)
	v1.POST("/auth/reset-password", s.resetPassword)
	v1.POST("/auth/verify-email", s.verifyEmail)
	v1.POST("/auth/resend-verification", s.resendVerification)
	v1.GET("/oauth/ifpay/start", s.ifpayOAuthStart)
	v1.GET("/oauth/ifpay/callback", s.ifpayOAuthCallback)
	v1.POST("/ifpay/webhooks/payments", s.ifpayWebhook)
	v1.GET("/admin/auth/status", s.adminAuthStatus)
	v1.POST("/admin/auth/bootstrap/start", s.adminBootstrapStart)
	v1.POST("/admin/auth/bootstrap/complete", s.adminBootstrapComplete)
	v1.POST("/admin/auth/login", s.adminLogin)
	v1.POST("/admin/auth/logout", s.adminLogout)
	protected := v1.Group("", s.userAuth())
	protected.GET("/auth/me", s.me)
	protected.POST("/checkout/ifpay", s.createIFPayCheckout)
	protected.GET("/orders", s.listOrders)
	protected.POST("/images", s.uploadImage)
	protected.GET("/images", s.listImages)
	protected.GET("/images/:public_id/sign", s.signImage)
	protected.PATCH("/images/:public_id/privacy", s.updateImagePrivacy)
	protected.DELETE("/images/:public_id", s.deleteImage)
	protected.GET("/albums", s.listAlbums)
	protected.POST("/albums", s.createAlbum)
	protected.GET("/api-keys", s.listAPIKeys)
	protected.POST("/api-keys", s.createAPIKey)
	protected.DELETE("/api-keys/:id", s.revokeAPIKey)
	protected.GET("/backups/export", s.exportBackup)
	protected.POST("/backups/import", s.importBackup)
	protected.POST("/invites/:code/redeem", s.redeemInvite)
	protected.PATCH("/settings/profile", s.updateProfile)
	protected.POST("/settings/account-destroy-request", s.requestAccountDestroy)
	v1.GET("/internal/image-auth", s.internalImageAuth)

	admin := v1.Group("/admin", s.adminAuth())
	admin.GET("/auth/me", s.adminMe)
	admin.GET("/overview", s.adminOverview)
	admin.GET("/users", s.adminUsers)
	admin.POST("/users/:id/grant-plan", s.adminGrantPlan)
	admin.GET("/plans", s.adminPlans)
	admin.POST("/plans", s.adminUpsertPlan)
	admin.GET("/invites", s.adminInvites)
	admin.POST("/invites", s.adminCreateInvite)
	admin.GET("/orders", s.adminOrders)
	admin.POST("/orders/:id/mark-paid", s.adminMarkOrderPaid)
	admin.POST("/orders/:id/cancel", s.adminCancelOrder)
	admin.POST("/orders/:id/refund", s.adminRefundOrder)
	admin.GET("/security/events", s.adminRiskEvents)
	admin.GET("/security/hotlink", s.adminHotlinkConfig)
	admin.PATCH("/security/hotlink", s.adminUpdateHotlinkConfig)
	admin.GET("/storage/config", s.adminStorageConfig)
	admin.GET("/system/config", s.adminSystemConfig)
	admin.GET("/queue/status", s.adminQueueStatus)
	admin.GET("/queue/dead-letters", s.adminQueueDeadLetters)
	admin.POST("/queue/dead-letters/:id/requeue", s.adminRequeueDeadLetter)
	admin.GET("/cdn/config", s.adminCDNConfig)
	admin.GET("/api/config", s.adminAPIConfig)
	admin.GET("/integrations/ifpay", s.adminIFPayConfig)
	admin.PATCH("/integrations/ifpay", s.adminUpdateIFPayConfig)
	admin.GET("/backups/export", s.adminExportBackup)
	admin.POST("/backups/import/validate", s.adminValidateBackup)
	admin.GET("/audit-logs", s.adminAuditLogs)
	admin.GET("/images", s.adminImages)
	admin.POST("/images/:public_id/freeze", s.adminFreezeImage)
	admin.DELETE("/images/:public_id", s.adminDeleteImage)
	admin.POST("/users/:id/ban", s.adminBanUser)
	admin.POST("/users/:id/unban", s.adminUnbanUser)
	admin.POST("/users/:id/subscription/expire", s.adminExpireSubscription)
	admin.POST("/jobs/purge-expired", s.adminPurgeExpired)
}

func serveDocFile(filename, contentType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path, exists := findDocFile(filename)
		if !exists {
			fail(c, http.StatusNotFound, "doc_not_found", "接口文档未打包，请检查部署产物")
			return
		}
		c.Header("Content-Type", contentType)
		c.File(path)
	}
}

func findDocFile(filename string) (string, bool) {
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, `\`) {
		return "", false
	}
	for _, dir := range []string{"docs", "/app/docs", "yuexiang-image-backend/docs"} {
		path := filepath.Join(dir, filename)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func (s *Server) metrics(c *gin.Context) {
	s.state.mu.RLock()
	var storageBytes, bandwidthBytes, imageRequests, apiCalls int64
	for _, image := range s.state.images {
		if image.Status != "deleted" {
			storageBytes += image.Bytes + variantBytesTotal(image.Variants)
		}
	}
	for _, usage := range s.state.usage {
		bandwidthBytes += usage.BandwidthBytes
		imageRequests += usage.ImageRequests
		apiCalls += usage.APICalls
	}
	usersTotal := len(s.state.users)
	imagesTotal := len(s.state.images)
	riskEventsTotal := len(s.state.riskEvents)
	s.state.mu.RUnlock()
	httpMetrics := s.httpMetricsText()

	queueMetrics := ""
	if s.cfg.QueueDriver == "redis" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		stats, err := queue.InspectRedis(ctx, queue.RedisConfig{
			Addr:             s.cfg.RedisAddr,
			DB:               s.cfg.RedisDB,
			Stream:           s.cfg.QueueStream,
			DeadLetterStream: s.cfg.QueueDeadLetterStream,
		}, s.cfg.QueueGroup)
		reachable := 1
		if err != nil {
			reachable = 0
		}
		queueMetrics = fmt.Sprintf(`# HELP yuexiang_queue_redis_reachable Redis queue health, 1 means reachable.
# TYPE yuexiang_queue_redis_reachable gauge
yuexiang_queue_redis_reachable %d
# HELP yuexiang_queue_length Redis Stream task length.
# TYPE yuexiang_queue_length gauge
yuexiang_queue_length %d
# HELP yuexiang_queue_pending Redis Stream pending task count.
# TYPE yuexiang_queue_pending gauge
yuexiang_queue_pending %d
# HELP yuexiang_queue_lag Redis Stream consumer group lag.
# TYPE yuexiang_queue_lag gauge
yuexiang_queue_lag %d
# HELP yuexiang_queue_dead_letters Redis Stream dead-letter task count.
# TYPE yuexiang_queue_dead_letters gauge
yuexiang_queue_dead_letters %d
`, reachable, stats.Length, stats.Pending, stats.Lag, stats.DeadLetterLength)
	}

	body := fmt.Sprintf(`# HELP yuexiang_users_total Total users.
# TYPE yuexiang_users_total gauge
yuexiang_users_total %d
# HELP yuexiang_images_total Total image records.
# TYPE yuexiang_images_total gauge
yuexiang_images_total %d
# HELP yuexiang_storage_bytes Stored image bytes.
# TYPE yuexiang_storage_bytes gauge
yuexiang_storage_bytes %d
# HELP yuexiang_bandwidth_bytes Delivered bandwidth bytes in current counters.
# TYPE yuexiang_bandwidth_bytes counter
yuexiang_bandwidth_bytes %d
# HELP yuexiang_image_requests_total Delivered image requests in current counters.
# TYPE yuexiang_image_requests_total counter
yuexiang_image_requests_total %d
# HELP yuexiang_api_calls_total API calls in current counters.
# TYPE yuexiang_api_calls_total counter
yuexiang_api_calls_total %d
# HELP yuexiang_risk_events_total Risk events recorded.
# TYPE yuexiang_risk_events_total counter
yuexiang_risk_events_total %d
%s
%s`, usersTotal, imagesTotal, storageBytes, bandwidthBytes, imageRequests, apiCalls, riskEventsTotal, httpMetrics, queueMetrics)
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(body))
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		gin.DefaultWriter.Write([]byte(fmt.Sprintf("%s %s %d %s\n", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))))
	}
}

func (s *Server) requestMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		key := requestStatKey{
			Method: c.Request.Method,
			Route:  route,
			Status: c.Writer.Status(),
		}
		s.metricsMu.Lock()
		s.requests[key]++
		s.metricsMu.Unlock()
	}
}

func (s *Server) httpMetricsText() string {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	if len(s.requests) == 0 {
		return "# HELP yuexiang_http_requests_total HTTP requests by method, route and status.\n# TYPE yuexiang_http_requests_total counter"
	}
	keys := make([]requestStatKey, 0, len(s.requests))
	for key := range s.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Status < keys[j].Status
	})
	var b strings.Builder
	b.WriteString("# HELP yuexiang_http_requests_total HTTP requests by method, route and status.\n")
	b.WriteString("# TYPE yuexiang_http_requests_total counter\n")
	for _, key := range keys {
		b.WriteString(fmt.Sprintf(
			"yuexiang_http_requests_total{method=%q,route=%q,status=%q} %d\n",
			key.Method,
			key.Route,
			strconv.Itoa(key.Status),
			s.requests[key],
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = "req_" + uuid.NewString()
		}
		c.Header("X-Request-Id", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

func securityHeaders(appEnv string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if appEnv == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		c.Next()
	}
}

func cors(allowedOrigins string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range strings.Split(allowedOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed["*"] {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token, X-Device-Id")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 600
		if c.Request.Method == http.MethodPost && strings.HasPrefix(c.Request.URL.Path, "/api/v1/images") {
			limit = 120
		}
		if strings.Contains(c.Request.URL.Path, "/checkout/ifpay") {
			limit = 30
		}
		if strings.Contains(c.Request.URL.Path, "/auth/login") {
			limit = 20
		}
		key := c.ClientIP() + ":" + c.Request.Method + ":" + c.FullPath()
		if key == c.ClientIP()+":"+c.Request.Method+":" {
			key = c.ClientIP() + ":" + c.Request.Method + ":" + c.Request.URL.Path
		}
		now := time.Now()
		s.rateMu.Lock()
		window := s.rate[key]
		if now.After(window.WindowEnds) {
			window = rateWindow{WindowEnds: now.Add(time.Minute)}
		}
		window.Count++
		s.rate[key] = window
		s.rateMu.Unlock()
		if window.Count > limit {
			s.recordRisk("rate_limited", "application_rate_limit_exceeded", c.ClientIP(), c.GetHeader("Referer"))
			fail(c, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后再试")
			return
		}
		c.Next()
	}
}

func (s *Server) adminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := adminTokenFromRequest(c)
		legacyToken := strings.TrimSpace(c.GetHeader("X-Admin-Token"))
		if token == "" && legacyToken == s.cfg.AdminToken && s.cfg.AppEnv != "production" {
			c.Set("admin_identity", AdminIdentity{
				AdminID: "legacy-admin",
				Email:   "legacy-admin@local",
				Name:    "LegacyAdmin",
				Role:    "super_admin",
			})
			c.Next()
			return
		}
		if token == "" {
			fail(c, http.StatusUnauthorized, "admin_unauthorized", "管理员鉴权失败")
			return
		}
		adminID, err := security.VerifyAdminToken(s.cfg.JWTSecret, token, time.Now())
		if err != nil {
			fail(c, http.StatusUnauthorized, "admin_unauthorized", "管理员登录状态无效或已过期")
			return
		}
		tokenHash := security.HashSecret(token, s.cfg.JWTSecret)
		s.state.mu.Lock()
		session, sessionExists := s.state.adminSessions[tokenHash]
		admin, adminExists := s.state.admins[adminID]
		now := time.Now().UTC()
		if !sessionExists || !adminExists || session.AdminID != adminID || session.ExpiresAt.Before(now) || admin.Status != "active" {
			if sessionExists && session.ExpiresAt.Before(now) {
				delete(s.state.adminSessions, tokenHash)
			}
			s.state.mu.Unlock()
			fail(c, http.StatusUnauthorized, "admin_unauthorized", "管理员登录状态无效或已过期")
			return
		}
		s.state.mu.Unlock()
		c.Set("admin_identity", AdminIdentity{
			AdminID: admin.ID,
			Email:   admin.Email,
			Name:    admin.Name,
			Role:    admin.Role,
		})
		c.Next()
	}
}

func (s *Server) userAuth(requiredScopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(auth, "Bearer yx_session_") {
			userID, err := security.VerifyUserToken(s.cfg.JWTSecret, strings.TrimPrefix(auth, "Bearer "), time.Now())
			if err != nil {
				fail(c, http.StatusUnauthorized, "invalid_session", "登录状态无效或已过期")
				return
			}
			s.state.mu.RLock()
			user, okUser := s.state.users[userID]
			s.state.mu.RUnlock()
			if !okUser {
				fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
				return
			}
			if user.Status == "banned" {
				fail(c, http.StatusForbidden, "user_banned", "账号已被封禁")
				return
			}
			c.Set("auth_identity", AuthIdentity{UserID: userID, Source: "session"})
			c.Next()
			return
		}

		apiSecret := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if apiSecret == "" && strings.HasPrefix(auth, "Bearer yx_") {
			apiSecret = strings.TrimPrefix(auth, "Bearer ")
		}
		if apiSecret == "" {
			fail(c, http.StatusUnauthorized, "auth_required", "请先登录或提供 API Key")
			return
		}
		secretHash := security.HashSecret(apiSecret, s.cfg.JWTSecret)
		now := time.Now().UTC()
		s.state.mu.Lock()
		for userID, keys := range s.state.apiKeys {
			for idx, key := range keys {
				if key.Revoked || key.SecretHash != secretHash {
					continue
				}
				user, okUser := s.state.users[userID]
				if !okUser {
					s.state.mu.Unlock()
					fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
					return
				}
				if user.Status == "banned" {
					s.state.mu.Unlock()
					fail(c, http.StatusForbidden, "user_banned", "账号已被封禁")
					return
				}
				if !hasRequiredScopes(key.Scopes, requiredScopes) {
					s.state.mu.Unlock()
					fail(c, http.StatusForbidden, "api_key_scope_denied", "API Key 权限不足")
					return
				}
				plan, _, usage, entitlementErr := s.entitlementSnapshotLocked(userID)
				if entitlementErr != nil {
					s.state.mu.Unlock()
					fail(c, http.StatusPaymentRequired, "subscription_required", entitlementErr.Error())
					return
				}
				snap := snapshotStateLocked(s.state)
				usage.APICalls++
				if quotaCheck := domain.CheckUsage(plan, usage); !quotaCheck.Allowed {
					s.state.mu.Unlock()
					fail(c, http.StatusPaymentRequired, "api_quota_exceeded", quotaCheck.Message)
					return
				}
				key.LastUsedAt = &now
				keys[idx] = key
				s.state.apiKeys[userID] = keys
				s.state.usage[userID] = usage
				s.state.mu.Unlock()
				if err := s.persistState(c.Request.Context()); err != nil {
					restoreStateSnapshot(s.state, snap)
					fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
					return
				}
				c.Set("auth_identity", AuthIdentity{UserID: userID, Source: "api_key", Scopes: key.Scopes})
				c.Next()
				return
			}
		}
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "invalid_api_key", "API Key 无效或已撤销")
	}
}

func (s *Server) listPublicPlans(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	ok(c, domain.PublicPlans(s.state.plans))
}

func (s *Server) register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if bind(c, &req) {
		return
	}
	if len(req.Password) < 8 {
		fail(c, http.StatusBadRequest, "weak_password", "密码至少需要 8 位")
		return
	}
	now := time.Now().UTC()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	passwordHash, err := security.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "password_hash_failed", "密码处理失败")
		return
	}
	verificationCode := shortCode()
	user := User{
		ID:                    "usr_" + uuid.NewString(),
		Email:                 email,
		Nickname:              fallback(req.Nickname, "悦享用户"),
		EmailVerified:         false,
		PlanSlug:              "go",
		PasswordHash:          passwordHash,
		EmailVerificationCode: verificationCode,
		Status:                "active",
		CreatedAt:             now,
	}
	s.state.mu.Lock()
	for _, existing := range s.state.users {
		if existing.Email == email {
			s.state.mu.Unlock()
			fail(c, http.StatusConflict, "email_exists", "邮箱已注册")
			return
		}
	}
	snap := snapshotStateLocked(s.state)
	s.state.users[user.ID] = user
	s.state.subscriptions[user.ID] = domain.NewSubscription("sub_"+uuid.NewString(), user.ID, "go", now, 30*24*time.Hour)
	s.state.usage[user.ID] = domain.Usage{}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	if err := s.mailer.Send(c.Request.Context(), mail.VerificationMessage(user.Email, verificationCode)); err != nil && s.cfg.AppEnv == "production" {
		if !s.restoreSnapshotAndPersist(c, snap) {
			return
		}
		fail(c, http.StatusBadGateway, "mail_send_failed", "邮箱验证码发送失败")
		return
	}
	token, err := security.SignUserToken(s.cfg.JWTSecret, user.ID, 7*24*time.Hour, now)
	if err != nil {
		fail(c, http.StatusInternalServerError, "token_sign_failed", "登录令牌生成失败")
		return
	}
	payload := gin.H{"user": user, "token": token, "message": "注册成功，请检查邮箱验证码"}
	if s.cfg.AppEnv != "production" {
		payload["dev_email_verification_code"] = verificationCode
	}
	ok(c, payload)
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if bind(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	for _, user := range s.state.users {
		if user.Email == email && security.VerifyPassword(user.PasswordHash, req.Password) {
			token, err := security.SignUserToken(s.cfg.JWTSecret, user.ID, 7*24*time.Hour, time.Now().UTC())
			if err != nil {
				fail(c, http.StatusInternalServerError, "token_sign_failed", "登录令牌生成失败")
				return
			}
			ok(c, gin.H{"user": user, "token": token})
			return
		}
	}
	fail(c, http.StatusUnauthorized, "invalid_credentials", "邮箱或密码错误")
}

func (s *Server) adminAuthStatus(c *gin.Context) {
	s.state.mu.RLock()
	setupRequired := len(s.state.admins) == 0
	var admin *AdminUser
	token := adminTokenFromRequest(c)
	if adminID, err := security.VerifyAdminToken(s.cfg.JWTSecret, token, time.Now()); err == nil {
		tokenHash := security.HashSecret(token, s.cfg.JWTSecret)
		session, sessionExists := s.state.adminSessions[tokenHash]
		if value, exists := s.state.admins[adminID]; exists && sessionExists && session.AdminID == adminID && session.ExpiresAt.After(time.Now().UTC()) && value.Status == "active" {
			value.PasswordHash = ""
			value.TOTPSecret = ""
			admin = &value
		}
	}
	s.state.mu.RUnlock()
	ok(c, gin.H{"setup_required": setupRequired, "admin": admin})
}

func (s *Server) adminBootstrapStart(c *gin.Context) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if bind(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.DisplayName)
	if email == "" || !strings.Contains(email, "@") {
		fail(c, http.StatusBadRequest, "admin_email_invalid", "请输入正确的管理员邮箱")
		return
	}
	if len(req.Password) < 10 {
		fail(c, http.StatusBadRequest, "admin_password_too_short", "管理员密码至少需要 10 位")
		return
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	passwordHash, err := security.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "password_hash_failed", "密码处理失败")
		return
	}
	totpSecret, provisioningURL, qrCodeDataURL, err := security.GenerateTOTPSetup(email, "悦享图床管理后台")
	if err != nil {
		fail(c, http.StatusInternalServerError, "admin_totp_setup_failed", "管理员两步验证初始化失败")
		return
	}
	now := time.Now().UTC()
	setupID := "setup_" + uuid.NewString()
	setup := PendingAdminBootstrap{
		ID:           setupID,
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		TOTPSecret:   totpSecret,
		CreatedAt:    now,
		ExpiresAt:    now.Add(15 * time.Minute),
	}
	s.state.mu.Lock()
	if len(s.state.admins) > 0 {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "admin_setup_unavailable", "首个管理员账号已经初始化")
		return
	}
	for id, pending := range s.state.adminBootstrap {
		if pending.ExpiresAt.Before(now) {
			delete(s.state.adminBootstrap, id)
		}
	}
	s.state.adminBootstrap[setupID] = setup
	s.state.mu.Unlock()
	ok(c, gin.H{
		"setup_token":        setupID,
		"email":              email,
		"manual_entry_key":   totpSecret,
		"provisioning_url":   provisioningURL,
		"qr_code_data_url":   qrCodeDataURL,
		"issuer":             "悦享图床管理后台",
		"totp_app_hint":      "Microsoft Authenticator / Google Authenticator",
		"expires_in_seconds": 900,
	})
}

func (s *Server) adminBootstrapComplete(c *gin.Context) {
	var req struct {
		SetupToken string `json:"setup_token"`
		TOTPCode   string `json:"totp_code"`
	}
	if bind(c, &req) {
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	if len(s.state.admins) > 0 {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "admin_setup_unavailable", "首个管理员账号已经初始化")
		return
	}
	setup, exists := s.state.adminBootstrap[strings.TrimSpace(req.SetupToken)]
	if !exists || setup.ExpiresAt.Before(now) {
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "admin_setup_expired", "管理员初始化会话已过期，请重新开始")
		return
	}
	if !security.ValidateTOTP(setup.TOTPSecret, req.TOTPCode) {
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "invalid_totp", "两步验证码不正确")
		return
	}
	snap := snapshotStateLocked(s.state)
	admin := AdminUser{
		ID:           "adm_" + uuid.NewString(),
		Email:        setup.Email,
		Name:         setup.Name,
		Role:         "super_admin",
		Status:       "active",
		PasswordHash: setup.PasswordHash,
		TOTPSecret:   setup.TOTPSecret,
		CreatedAt:    now,
		LastLoginAt:  &now,
	}
	s.state.admins[admin.ID] = admin
	delete(s.state.adminBootstrap, setup.ID)
	token, session, err := s.issueAdminSessionLocked(admin.ID, c, now)
	if err != nil {
		s.state.mu.Unlock()
		restoreStateSnapshot(s.state, snap)
		fail(c, http.StatusInternalServerError, "admin_session_failed", "管理员登录状态创建失败")
		return
	}
	s.auditLocked("admin:"+admin.ID, "admin.bootstrap", admin.ID, map[string]any{"email": admin.Email})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	admin.PasswordHash = ""
	admin.TOTPSecret = ""
	ok(c, gin.H{"admin": admin, "token": token, "expires_at": session.ExpiresAt})
}

func (s *Server) adminLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if bind(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	now := time.Now().UTC()
	s.state.mu.Lock()
	if len(s.state.admins) == 0 {
		s.state.mu.Unlock()
		fail(c, http.StatusForbidden, "admin_setup_required", "请先初始化首个管理员账号")
		return
	}
	for id, admin := range s.state.admins {
		if admin.Email != email || admin.Status != "active" || !security.VerifyPassword(admin.PasswordHash, req.Password) || !security.ValidateTOTP(admin.TOTPSecret, req.TOTPCode) {
			continue
		}
		snap := snapshotStateLocked(s.state)
		admin.LastLoginAt = &now
		s.state.admins[id] = admin
		token, session, err := s.issueAdminSessionLocked(admin.ID, c, now)
		if err != nil {
			s.state.mu.Unlock()
			restoreStateSnapshot(s.state, snap)
			fail(c, http.StatusInternalServerError, "admin_session_failed", "管理员登录状态创建失败")
			return
		}
		s.auditLocked("admin:"+admin.ID, "admin.login", admin.ID, map[string]any{"email": admin.Email})
		s.state.mu.Unlock()
		if !s.persistOrRollback(c, snap) {
			return
		}
		admin.PasswordHash = ""
		admin.TOTPSecret = ""
		ok(c, gin.H{"admin": admin, "token": token, "expires_at": session.ExpiresAt})
		return
	}
	s.state.mu.Unlock()
	fail(c, http.StatusUnauthorized, "invalid_credentials", "邮箱、密码或 2FA 验证码不正确")
}

func (s *Server) adminLogout(c *gin.Context) {
	token := adminTokenFromRequest(c)
	if token != "" {
		tokenHash := security.HashSecret(token, s.cfg.JWTSecret)
		s.state.mu.Lock()
		snap := snapshotStateLocked(s.state)
		delete(s.state.adminSessions, tokenHash)
		s.state.mu.Unlock()
		if !s.persistOrRollback(c, snap) {
			return
		}
	}
	ok(c, gin.H{"logged_out": true})
}

func (s *Server) adminMe(c *gin.Context) {
	identity := currentAdminIdentity(c)
	ok(c, gin.H{"admin": identity})
}

func (s *Server) forgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if bind(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	resetCode := shortCode()
	var matched bool
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	for id, user := range s.state.users {
		if user.Email == email {
			user.PasswordResetCode = resetCode
			s.state.users[id] = user
			matched = true
			break
		}
	}
	s.state.mu.Unlock()
	if matched {
		if !s.persistOrRollback(c, snap) {
			return
		}
		if err := s.mailer.Send(c.Request.Context(), mail.PasswordResetMessage(email, resetCode)); err != nil && s.cfg.AppEnv == "production" {
			if !s.restoreSnapshotAndPersist(c, snap) {
				return
			}
			fail(c, http.StatusBadGateway, "mail_send_failed", "密码重置邮件发送失败")
			return
		}
	}
	payload := gin.H{"message": "如果邮箱存在，系统会发送重置链接"}
	if matched && s.cfg.AppEnv != "production" {
		payload["dev_password_reset_code"] = resetCode
	}
	ok(c, payload)
}

func (s *Server) resetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if bind(c, &req) {
		return
	}
	if len(req.NewPassword) < 8 {
		fail(c, http.StatusBadRequest, "weak_password", "密码至少需要 8 位")
		return
	}
	hash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "password_hash_failed", "密码处理失败")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	s.state.mu.Lock()
	for id, user := range s.state.users {
		if user.Email == email && user.PasswordResetCode != "" && user.PasswordResetCode == strings.TrimSpace(req.Code) {
			snap := snapshotStateLocked(s.state)
			user.PasswordHash = hash
			user.PasswordResetCode = ""
			s.state.users[id] = user
			s.state.mu.Unlock()
			if !s.persistOrRollback(c, snap) {
				return
			}
			ok(c, gin.H{"message": "密码已重置"})
			return
		}
	}
	s.state.mu.Unlock()
	fail(c, http.StatusBadRequest, "invalid_reset_code", "重置验证码无效")
}

func (s *Server) verifyEmail(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if bind(c, &req) {
		return
	}
	s.state.mu.Lock()
	user, okUser := s.state.users[req.UserID]
	if !okUser {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	}
	if user.EmailVerificationCode != "" && strings.TrimSpace(req.Code) != user.EmailVerificationCode {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "invalid_verification_code", "邮箱验证码无效")
		return
	}
	snap := snapshotStateLocked(s.state)
	user.EmailVerified = true
	user.EmailVerificationCode = ""
	s.state.users[user.ID] = user
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, user)
}

func (s *Server) resendVerification(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if bind(c, &req) {
		return
	}
	s.state.mu.Lock()
	user, okUser := s.state.users[req.UserID]
	if !okUser {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	}
	if user.EmailVerified {
		s.state.mu.Unlock()
		ok(c, gin.H{"message": "邮箱已验证"})
		return
	}
	snap := snapshotStateLocked(s.state)
	if user.EmailVerificationCode == "" {
		user.EmailVerificationCode = shortCode()
		s.state.users[user.ID] = user
	}
	code := user.EmailVerificationCode
	email := user.Email
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	if err := s.mailer.Send(c.Request.Context(), mail.VerificationMessage(email, code)); err != nil && s.cfg.AppEnv == "production" {
		if !s.restoreSnapshotAndPersist(c, snap) {
			return
		}
		fail(c, http.StatusBadGateway, "mail_send_failed", "邮箱验证码发送失败")
		return
	}
	payload := gin.H{"message": "验证码已重新发送"}
	if s.cfg.AppEnv != "production" {
		payload["dev_email_verification_code"] = code
	}
	ok(c, payload)
}

func (s *Server) updateProfile(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	var req struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	}
	if bind(c, &req) {
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" || len([]rune(nickname)) > 40 {
		fail(c, http.StatusBadRequest, "invalid_nickname", "昵称不能为空且不能超过 40 个字符")
		return
	}
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if avatarURL != "" && !(strings.HasPrefix(avatarURL, "data:image/") || strings.HasPrefix(avatarURL, "https://") || strings.HasPrefix(avatarURL, "/")) {
		fail(c, http.StatusBadRequest, "invalid_avatar_url", "Avatar 必须是图片 data URL、HTTPS 地址或站内路径")
		return
	}
	if len(avatarURL) > 512*1024 {
		fail(c, http.StatusBadRequest, "avatar_too_large", "Avatar 数据过大，请上传 512KB 以内图片")
		return
	}
	identity := currentIdentity(c)
	s.state.mu.Lock()
	user, exists := s.state.users[identity.UserID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	user.Nickname = nickname
	if avatarURL != "" {
		user.AvatarURL = avatarURL
	}
	s.state.users[user.ID] = user
	s.auditLocked("user", "profile.update", user.ID, map[string]any{"nickname": nickname, "avatar_updated": avatarURL != ""})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"user": user})
}

func (s *Server) requestAccountDestroy(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	identity := currentIdentity(c)
	ticketID := "destroy_" + uuid.NewString()
	s.state.mu.Lock()
	user, exists := s.state.users[identity.UserID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	s.auditLocked("user", "account.destroy.request", user.ID, map[string]any{
		"ticket_id": ticketID,
		"email":     user.Email,
		"reason":    strings.TrimSpace(req.Reason),
	})
	s.state.riskEvents = append(s.state.riskEvents, RiskEvent{
		ID:        "risk_" + uuid.NewString(),
		Type:      "account_destroy_requested",
		Message:   "user requested account destruction",
		IP:        c.ClientIP(),
		CreatedAt: time.Now().UTC(),
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"ticket_id": ticketID, "status": "submitted"})
}

func (s *Server) me(c *gin.Context) {
	identity := currentIdentity(c)
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	user, okUser := s.state.users[identity.UserID]
	if !okUser {
		fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
		return
	}
	ok(c, gin.H{
		"user":         user,
		"subscription": s.state.subscriptions[identity.UserID],
		"usage":        s.state.usage[identity.UserID],
		"auth_source":  identity.Source,
		"scopes":       identity.Scopes,
	})
}

func (s *Server) ifpayOAuthStart(c *gin.Context) {
	state := uuid.NewString()
	integration := s.integrationSnapshot()
	authURL := fmt.Sprintf("%s/api/ifpay/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid%%20wallet:read%%20payments:write&state=%s",
		ifpayAPIBase(integration.IFPayBaseURL),
		urlQuery(integration.IFPayClientID),
		urlQuery(integration.IFPayRedirectURI),
		urlQuery(state),
	)
	ok(c, gin.H{"authorization_url": authURL, "state": state, "provider": "ifpay"})
}

func (s *Server) ifpayOAuthCallback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		fail(c, http.StatusBadRequest, "oauth_code_required", "缺少 IF-Pay OAuth code")
		return
	}
	integration := s.integrationSnapshot()
	ifpayClient := integration.IFPayClient()
	tokenResponse, err := ifpayClient.ExchangeOAuthCode(c.Request.Context(), code, integration.IFPayRedirectURI)
	if err != nil {
		fail(c, http.StatusBadGateway, "ifpay_oauth_token_failed", err.Error())
		return
	}
	userInfo, err := ifpayClient.FetchUserInfo(c.Request.Context(), tokenResponse.AccessToken)
	if err != nil {
		fail(c, http.StatusBadGateway, "ifpay_userinfo_failed", err.Error())
		return
	}
	email := strings.ToLower(strings.TrimSpace(userInfo.Email))
	if email == "" {
		email = "ifpay-user-" + uuid.NewString()[:8] + "@oauth.local"
	}
	nickname := fallback(userInfo.Nickname, fallback(userInfo.Name, "IF-Pay 用户"))
	now := time.Now().UTC()
	var user User
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	for id, existing := range s.state.users {
		if existing.Email == email {
			existing.OAuthBound = true
			existing.EmailVerified = existing.EmailVerified || userInfo.EmailVerified || s.cfg.AppEnv != "production"
			if strings.TrimSpace(existing.Nickname) == "" {
				existing.Nickname = nickname
			}
			s.state.users[id] = existing
			user = existing
			break
		}
	}
	if user.ID == "" {
		user = User{
			ID:            "usr_" + uuid.NewString(),
			Email:         email,
			Nickname:      nickname,
			EmailVerified: userInfo.EmailVerified || s.cfg.AppEnv != "production",
			OAuthBound:    true,
			PlanSlug:      "go",
			Status:        "active",
			CreatedAt:     now,
		}
		s.state.users[user.ID] = user
		s.state.subscriptions[user.ID] = domain.NewSubscription("sub_"+uuid.NewString(), user.ID, "go", now, 30*24*time.Hour)
		s.state.usage[user.ID] = domain.Usage{}
	}
	s.auditLocked("system", "oauth.ifpay.login", user.ID, map[string]any{"email": user.Email, "subject": userInfo.Subject})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	token, err := security.SignUserToken(s.cfg.JWTSecret, user.ID, 7*24*time.Hour, time.Now().UTC())
	if err != nil {
		fail(c, http.StatusInternalServerError, "token_sign_failed", "登录令牌生成失败")
		return
	}
	ok(c, gin.H{"user": user, "token": token, "ifpay_access_token": tokenResponse.AccessToken, "ifpay_expires_in": tokenResponse.ExpiresIn})
}

func (s *Server) createIFPayCheckout(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	var req struct {
		PlanSlug     string `json:"plan_slug"`
		BillingCycle string `json:"billing_cycle"`
		SubMethod    string `json:"sub_method"`
		AccessToken  string `json:"access_token"`
	}
	if bind(c, &req) {
		return
	}
	identity := currentIdentity(c)
	if !s.userEmailVerified(identity.UserID) {
		fail(c, http.StatusForbidden, "email_not_verified", "请先完成邮箱验证再购买资源包")
		return
	}
	billingCycle := fallback(req.BillingCycle, "monthly")
	if billingCycle != "monthly" && billingCycle != "yearly" {
		fail(c, http.StatusBadRequest, "invalid_billing_cycle", "计费周期必须是 monthly 或 yearly")
		return
	}
	if s.cfg.AppEnv == "production" && strings.TrimSpace(req.AccessToken) == "" {
		fail(c, http.StatusBadRequest, "ifpay_access_token_required", "生产环境创建支付订单需要 IF-Pay access token")
		return
	}
	s.state.mu.RLock()
	plan, found := domain.FindPlan(s.state.plans, req.PlanSlug)
	s.state.mu.RUnlock()
	if !found || !plan.Purchasable {
		fail(c, http.StatusBadRequest, "plan_not_purchasable", "套餐不可购买")
		return
	}
	amount := plan.MonthlyPriceCent
	if billingCycle == "yearly" {
		amount = plan.YearlyPriceCent
	}
	orderID := "ord_" + uuid.NewString()
	now := time.Now().UTC()
	order := Order{
		ID:           orderID,
		UserID:       identity.UserID,
		PlanSlug:     req.PlanSlug,
		BillingCycle: billingCycle,
		AmountCent:   amount,
		Status:       "pending",
		CreatedAt:    now,
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.orders[order.ID] = order
	s.auditLocked("system", "order.checkout_created", order.ID, map[string]any{
		"user_id":       identity.UserID,
		"plan_slug":     req.PlanSlug,
		"billing_cycle": billingCycle,
		"amount_cent":   amount,
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	payment, err := s.integrationSnapshot().IFPayClient().CreatePayment(c.Request.Context(), req.AccessToken, ifpay.PaymentCreateRequest{
		PaymentMethod: "ifpay",
		SubMethod:     fallback(req.SubMethod, "ifpay_balance"),
		OrderID:       orderID,
		Amount:        amount,
		Currency:      "CNY",
		Description:   "悦享图床 " + plan.Name + " 套餐",
		Metadata: map[string]any{
			"user_id":       identity.UserID,
			"plan_slug":     req.PlanSlug,
			"billing_cycle": billingCycle,
		},
	})
	if err != nil {
		failedAt := time.Now().UTC()
		s.state.mu.Lock()
		failSnap := snapshotStateLocked(s.state)
		current := s.state.orders[order.ID]
		current.Status = "failed"
		current.FailedAt = &failedAt
		current.OperatorNote = err.Error()
		s.state.orders[current.ID] = current
		s.auditLocked("system", "order.checkout_payment_failed", current.ID, map[string]any{
			"user_id": identity.UserID,
			"error":   err.Error(),
		})
		s.state.mu.Unlock()
		if !s.persistOrRollback(c, failSnap) {
			return
		}
		fail(c, http.StatusBadGateway, "ifpay_create_failed", err.Error())
		return
	}
	s.state.mu.Lock()
	order = s.state.orders[order.ID]
	order.IFPayPaymentID = payment.PaymentID
	order.IFPaySubMethod = payment.SubMethod
	s.state.orders[order.ID] = order
	s.auditLocked("system", "order.ifpay_payment_created", order.ID, map[string]any{
		"payment_id": payment.PaymentID,
		"sub_method": payment.SubMethod,
	})
	s.state.mu.Unlock()
	if err := s.persistState(c.Request.Context()); err != nil {
		fail(c, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	ok(c, gin.H{"order": order, "payment": payment})
}

func (s *Server) listOrders(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	orders := make([]Order, 0)
	for _, order := range s.state.orders {
		if order.UserID == identity.UserID {
			orders = append(orders, order)
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].CreatedAt.After(orders[j].CreatedAt) })
	ok(c, orders)
}

func (s *Server) ifpayWebhook(c *gin.Context) {
	raw, _ := io.ReadAll(c.Request.Body)
	integration := s.integrationSnapshot()
	if integration.IFPayWebhookPublicKey != "" {
		digest := c.GetHeader(ifpay.HeaderDigest)
		if !ifpay.VerifyDigest(digest, raw) {
			fail(c, http.StatusUnauthorized, "invalid_digest", "IF-Pay webhook digest 校验失败")
			return
		}
		canonical := ifpay.CanonicalMessage(c.Request.Method, c.Request.URL.Path, c.GetHeader(ifpay.HeaderTimestamp), c.GetHeader(ifpay.HeaderNonce), digest)
		if err := ifpay.VerifyRSASHA256(integration.IFPayWebhookPublicKey, canonical, c.GetHeader(ifpay.HeaderSignature)); err != nil {
			fail(c, http.StatusUnauthorized, "invalid_signature", "IF-Pay webhook 签名校验失败")
			return
		}
	}
	var event struct {
		EventID      string         `json:"event_id"`
		EventType    string         `json:"event_type"`
		ResourceID   string         `json:"resource_id"`
		ResourceType string         `json:"resource_type"`
		Payload      map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		fail(c, http.StatusBadRequest, "invalid_webhook", "webhook body 无效")
		return
	}
	if event.EventID == "" {
		event.EventID = "evt_" + uuid.NewString()
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	if s.state.webhooks[event.EventID] {
		s.state.mu.Unlock()
		ok(c, gin.H{"deduplicated": true})
		return
	}
	s.state.webhooks[event.EventID] = true
	order, found := s.findWebhookOrderLocked(event.ResourceID, event.Payload)
	if !found {
		s.auditLocked("system", "ifpay.webhook_unmatched", event.EventID, map[string]any{
			"event_type":  event.EventType,
			"resource_id": event.ResourceID,
			"order_id":    stringFromMap(event.Payload, "order_id"),
		})
		s.state.mu.Unlock()
		if !s.persistOrRollback(c, snap) {
			return
		}
		ok(c, gin.H{"received": true, "matched": false})
		return
	}
	now := time.Now().UTC()
	eventType := strings.ToLower(strings.TrimSpace(event.EventType))
	switch eventType {
	case "payment.succeeded":
		if _, _, err := s.activateOrderLocked(order, now, "system", "order.payment_succeeded", map[string]any{
			"event_id":   event.EventID,
			"payment_id": order.IFPayPaymentID,
		}); err != nil {
			s.auditLocked("system", "order.payment_succeeded_ignored", order.ID, map[string]any{
				"event_id": event.EventID,
				"status":   order.Status,
				"reason":   err.Error(),
			})
		}
	case "payment.failed", "payment.expired":
		reason := stringFromMap(event.Payload, "failure_reason")
		if reason == "" && eventType == "payment.expired" {
			reason = "payment expired"
		}
		if _, err := s.failOrderLocked(order, now, "system", "order.payment_failed", reason, map[string]any{
			"event_id":   event.EventID,
			"payment_id": order.IFPayPaymentID,
		}); err != nil {
			s.auditLocked("system", "order.payment_failed_ignored", order.ID, map[string]any{
				"event_id": event.EventID,
				"status":   order.Status,
				"reason":   err.Error(),
			})
		}
	case "payment.cancelled", "payment.canceled":
		if _, err := s.cancelOrderLocked(order, now, "system", "order.payment_cancelled", stringFromMap(event.Payload, "cancel_reason"), map[string]any{
			"event_id":   event.EventID,
			"payment_id": order.IFPayPaymentID,
		}); err != nil {
			s.auditLocked("system", "order.payment_cancelled_ignored", order.ID, map[string]any{
				"event_id": event.EventID,
				"status":   order.Status,
				"reason":   err.Error(),
			})
		}
	case "payment.refunded", "payment.refund.succeeded":
		if _, _, _, err := s.refundOrderLocked(order, now, "system", "order.payment_refunded", stringFromMap(event.Payload, "refund_reason"), map[string]any{
			"event_id":   event.EventID,
			"payment_id": order.IFPayPaymentID,
		}); err != nil {
			s.auditLocked("system", "order.payment_refunded_ignored", order.ID, map[string]any{
				"event_id": event.EventID,
				"status":   order.Status,
				"reason":   err.Error(),
			})
		}
	default:
		s.auditLocked("system", "ifpay.webhook_ignored", order.ID, map[string]any{
			"event_id":   event.EventID,
			"event_type": eventType,
		})
	}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"received": true})
}

func (s *Server) uploadImage(c *gin.Context) {
	if !requireScopes(c, "images:write") {
		return
	}
	identity := currentIdentity(c)
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "file_required", "请上传 file 字段")
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "file_open_failed", err.Error())
		return
	}
	defer src.Close()
	body, err := io.ReadAll(src)
	if err != nil {
		fail(c, http.StatusBadRequest, "file_read_failed", err.Error())
		return
	}
	s.state.mu.RLock()
	plan, sub, usage, entitlementErr := s.entitlementSnapshotLocked(identity.UserID)
	s.state.mu.RUnlock()
	if entitlementErr != nil {
		fail(c, http.StatusPaymentRequired, "subscription_required", entitlementErr.Error())
		return
	}
	if !sub.CanUpload(time.Now().UTC()) {
		fail(c, http.StatusPaymentRequired, "subscription_not_active", "套餐已到期或处于 30 天保留期，禁止继续上传")
		return
	}
	if !s.userEmailVerified(identity.UserID) {
		fail(c, http.StatusForbidden, "email_not_verified", "请先完成邮箱验证再上传")
		return
	}
	quotaCheck := domain.CheckUpload(plan, usage, int64(len(body)))
	if !quotaCheck.Allowed {
		fail(c, http.StatusPaymentRequired, "quota_exceeded", quotaCheck.Message)
		return
	}
	publicID := "img_" + uuid.NewString()
	objectKey := "original/" + publicID + "-" + filepath.Base(file.Filename)
	contentType := file.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(body)
	}
	if err := s.store.PutObject(c.Request.Context(), storage.PutObjectInput{
		Key:         objectKey,
		Body:        bytes.NewReader(body),
		ContentType: contentType,
		Private:     true,
	}); err != nil {
		fail(c, http.StatusInternalServerError, "object_store_failed", err.Error())
		return
	}
	createdObjectKeys := []string{objectKey}
	processed, _ := s.processor.Process(c.Request.Context(), bytes.NewReader(body))
	var variants []imageproc.Variant
	var variantBytes int64
	for _, generated := range processed.Generated {
		variant := generated.Variant
		variant.ObjectKey = fmt.Sprintf("variants/%s/%s%s", publicID, variant.Kind, extensionForMime(variant.MimeType))
		if err := s.store.PutObject(c.Request.Context(), storage.PutObjectInput{
			Key:         variant.ObjectKey,
			Body:        bytes.NewReader(generated.Body),
			ContentType: variant.MimeType,
			Private:     true,
		}); err != nil {
			continue
		}
		variants = append(variants, variant)
		createdObjectKeys = append(createdObjectKeys, variant.ObjectKey)
		variantBytes += variant.Bytes
	}
	image := Image{
		ID:             uuid.NewString(),
		PublicID:       publicID,
		UserID:         identity.UserID,
		Filename:       file.Filename,
		ObjectKey:      objectKey,
		ContentType:    contentType,
		Bytes:          int64(len(body)),
		Private:        c.PostForm("private") == "true",
		Width:          processed.Width,
		Height:         processed.Height,
		PerceptualHash: processed.PerceptualHash,
		Variants:       variants,
		Status:         "active",
		CreatedAt:      time.Now().UTC(),
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.images[publicID] = image
	usage = s.state.usage[identity.UserID]
	usage.StorageBytes += image.Bytes + variantBytes
	usage.ImageProcessEvents++
	s.state.usage[identity.UserID] = usage
	s.state.mu.Unlock()
	if !s.persistOrRollbackWithCleanup(c, snap, func() {
		s.cleanupObjects(c.Request.Context(), createdObjectKeys)
	}) {
		return
	}
	if err := s.tasks.Enqueue(c.Request.Context(), queue.Task{
		Type: "image.process",
		Payload: map[string]any{
			"public_id":  publicID,
			"user_id":    identity.UserID,
			"object_key": objectKey,
			"filename":   file.Filename,
		},
	}); err != nil && s.cfg.AppEnv == "production" {
		restoreStateSnapshot(s.state, snap)
		s.cleanupObjects(c.Request.Context(), createdObjectKeys)
		if rollbackErr := s.persistState(c.Request.Context()); rollbackErr != nil {
			fail(c, http.StatusInternalServerError, "state_persist_failed", rollbackErr.Error())
			return
		}
		fail(c, http.StatusBadGateway, "queue_enqueue_failed", "图片处理任务投递失败")
		return
	}
	ok(c, gin.H{
		"image": image,
		"links": gin.H{
			"raw":      strings.TrimRight(s.cfg.ImagePublicBaseURL, "/") + "/i/" + publicID,
			"markdown": fmt.Sprintf("![%s](%s/i/%s)", image.Filename, strings.TrimRight(s.cfg.ImagePublicBaseURL, "/"), publicID),
			"html":     fmt.Sprintf(`<img src="%s/i/%s" alt="%s">`, strings.TrimRight(s.cfg.ImagePublicBaseURL, "/"), publicID, image.Filename),
			"variants": variantLinks(s.cfg.ImagePublicBaseURL, publicID, variants),
		},
	})
}

func (s *Server) ProcessTask(ctx context.Context, task queue.Task) error {
	switch task.Type {
	case "image.process":
		publicID, _ := task.Payload["public_id"].(string)
		if publicID == "" {
			return fmt.Errorf("image.process missing public_id")
		}
		return s.ProcessImageVariants(ctx, publicID)
	default:
		return fmt.Errorf("unsupported task type %q", task.Type)
	}
}

func (s *Server) ProcessImageVariants(ctx context.Context, publicID string) error {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return fmt.Errorf("public_id is required")
	}
	s.state.mu.RLock()
	image, exists := s.state.images[publicID]
	s.state.mu.RUnlock()
	if !exists {
		return fmt.Errorf("image %s not found", publicID)
	}
	if image.Status == "deleted" || image.Status == "frozen" {
		return nil
	}
	data, _, err := s.store.GetObject(ctx, image.ObjectKey)
	if err != nil {
		return fmt.Errorf("read original object: %w", err)
	}
	processed, err := s.processor.Process(ctx, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("process image: %w", err)
	}
	generated := make([]imageproc.Variant, 0, len(processed.Generated))
	for _, item := range processed.Generated {
		variant := item.Variant
		if variant.Kind == "" || len(item.Body) == 0 {
			continue
		}
		variant.ObjectKey = fmt.Sprintf("variants/%s/%s%s", publicID, variant.Kind, extensionForMime(variant.MimeType))
		if err := s.store.PutObject(ctx, storage.PutObjectInput{
			Key:         variant.ObjectKey,
			Body:        bytes.NewReader(item.Body),
			ContentType: variant.MimeType,
			Private:     true,
		}); err != nil {
			return fmt.Errorf("write %s variant: %w", variant.Kind, err)
		}
		generated = append(generated, variant)
	}

	s.state.mu.Lock()
	current, exists := s.state.images[publicID]
	if !exists {
		s.state.mu.Unlock()
		return fmt.Errorf("image %s disappeared during processing", publicID)
	}
	if current.Status == "deleted" || current.Status == "frozen" {
		s.state.mu.Unlock()
		return nil
	}
	oldVariantBytes := variantBytesTotal(current.Variants)
	if processed.Width > 0 {
		current.Width = processed.Width
	}
	if processed.Height > 0 {
		current.Height = processed.Height
	}
	if processed.PerceptualHash != "" {
		current.PerceptualHash = processed.PerceptualHash
	}
	current.Variants = mergeVariants(current.Variants, generated)
	moderationReason := s.moderationBlockReasonLocked(publicID, current)
	if moderationReason != "" {
		current.Status = "frozen"
		current.ModerationReason = moderationReason
		s.state.riskEvents = append(s.state.riskEvents, RiskEvent{
			ID:        "risk_" + uuid.NewString(),
			Type:      "image_moderation",
			Message:   moderationReason,
			CreatedAt: time.Now().UTC(),
		})
	}
	newVariantBytes := variantBytesTotal(current.Variants)
	usage := s.state.usage[current.UserID]
	switch {
	case newVariantBytes > oldVariantBytes:
		usage.StorageBytes += newVariantBytes - oldVariantBytes
	case oldVariantBytes > newVariantBytes && usage.StorageBytes >= oldVariantBytes-newVariantBytes:
		usage.StorageBytes -= oldVariantBytes - newVariantBytes
	}
	usage.ImageProcessEvents++
	s.state.usage[current.UserID] = usage
	s.state.images[publicID] = current
	s.auditLocked("worker", "image.process", publicID, map[string]any{
		"variant_count": len(generated),
		"engine":        processed.Engine,
		"status":        current.Status,
		"p_hash":        current.PerceptualHash,
	})
	if moderationReason != "" {
		s.auditLocked("worker", "image.moderation.freeze", publicID, map[string]any{
			"reason":  moderationReason,
			"user_id": current.UserID,
			"p_hash":  current.PerceptualHash,
		})
	}
	s.state.mu.Unlock()
	return s.persistState(ctx)
}

func (s *Server) listImages(c *gin.Context) {
	if !requireScopes(c, "images:read") {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	images := make([]Image, 0, len(s.state.images))
	for _, image := range s.state.images {
		if image.UserID == identity.UserID && image.Status != "deleted" {
			images = append(images, image)
		}
	}
	ok(c, images)
}

func (s *Server) serveImage(c *gin.Context) {
	publicID := c.Param("public_id")
	s.state.mu.RLock()
	image, exists := s.state.images[publicID]
	s.state.mu.RUnlock()
	if !exists {
		c.Data(http.StatusNotFound, "image/svg+xml; charset=utf-8", []byte(protectSVG("图片不存在")))
		return
	}
	if image.Status == "frozen" || image.Status == "deleted" {
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("图片已冻结")))
		return
	}
	expiresAt, _ := strconv.ParseInt(c.Query("exp"), 10, 64)
	hotlink := s.hotlinkSnapshot()
	policy := security.ImageAccessPolicy{
		AllowedDomains:    hotlink.AllowedDomains,
		BlockedDomains:    hotlink.BlockedDomains,
		AllowEmptyReferer: hotlink.AllowEmptyReferer,
		SigningSecret:     s.cfg.ImageSigningSecret,
	}
	result := policy.Evaluate(security.ImageAccessRequest{
		PublicID:  publicID,
		Referer:   c.GetHeader("Referer"),
		Token:     c.Query("sig"),
		ExpiresAt: expiresAt,
		Private:   image.Private,
		Now:       time.Now(),
	})
	if result.Decision != security.DecisionAllow {
		s.recordRisk("hotlink_protected", result.Reason, c.ClientIP(), c.GetHeader("Referer"))
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("悦享保护图")))
		return
	}
	data, contentType, err := s.store.GetObject(c.Request.Context(), image.ObjectKey)
	if err != nil {
		c.Data(http.StatusNotFound, "image/svg+xml; charset=utf-8", []byte(protectSVG("对象不存在")))
		return
	}
	if !s.recordImageDelivery(image.UserID, int64(len(data))) {
		s.recordRisk("quota_protected", "bandwidth_or_request_quota_exceeded", c.ClientIP(), c.GetHeader("Referer"))
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("套餐额度已用尽")))
		return
	}
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) serveImageVariant(c *gin.Context) {
	publicID := c.Param("public_id")
	variantName := strings.TrimSuffix(c.Param("variant"), filepath.Ext(c.Param("variant")))
	s.state.mu.RLock()
	image, exists := s.state.images[publicID]
	s.state.mu.RUnlock()
	if !exists {
		c.Data(http.StatusNotFound, "image/svg+xml; charset=utf-8", []byte(protectSVG("图片不存在")))
		return
	}
	if image.Status == "frozen" || image.Status == "deleted" {
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("图片已冻结")))
		return
	}
	var selected imageproc.Variant
	for _, variant := range image.Variants {
		if variant.Kind == variantName {
			selected = variant
			break
		}
	}
	if selected.ObjectKey == "" {
		c.Data(http.StatusNotFound, "image/svg+xml; charset=utf-8", []byte(protectSVG("派生图不存在")))
		return
	}
	expiresAt, _ := strconv.ParseInt(c.Query("exp"), 10, 64)
	hotlink := s.hotlinkSnapshot()
	policy := security.ImageAccessPolicy{
		AllowedDomains:    hotlink.AllowedDomains,
		BlockedDomains:    hotlink.BlockedDomains,
		AllowEmptyReferer: hotlink.AllowEmptyReferer,
		SigningSecret:     s.cfg.ImageSigningSecret,
	}
	result := policy.Evaluate(security.ImageAccessRequest{
		PublicID:  publicID,
		Referer:   c.GetHeader("Referer"),
		Token:     c.Query("sig"),
		ExpiresAt: expiresAt,
		Private:   image.Private,
		Now:       time.Now(),
	})
	if result.Decision != security.DecisionAllow {
		s.recordRisk("hotlink_protected", result.Reason, c.ClientIP(), c.GetHeader("Referer"))
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("悦享保护图")))
		return
	}
	data, contentType, err := s.store.GetObject(c.Request.Context(), selected.ObjectKey)
	if err != nil {
		c.Data(http.StatusNotFound, "image/svg+xml; charset=utf-8", []byte(protectSVG("对象不存在")))
		return
	}
	if !s.recordImageDelivery(image.UserID, int64(len(data))) {
		s.recordRisk("quota_protected", "bandwidth_or_request_quota_exceeded", c.ClientIP(), c.GetHeader("Referer"))
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(protectSVG("套餐额度已用尽")))
		return
	}
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) signImage(c *gin.Context) {
	if !requireScopes(c, "images:read") {
		return
	}
	publicID := c.Param("public_id")
	identity := currentIdentity(c)
	s.state.mu.RLock()
	image, exists := s.state.images[publicID]
	s.state.mu.RUnlock()
	if !exists || image.UserID != identity.UserID {
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	policy := security.ImageAccessPolicy{SigningSecret: s.cfg.ImageSigningSecret}
	token := policy.SignToken(publicID, expiresAt)
	ok(c, gin.H{
		"url":        fmt.Sprintf("%s/i/%s?exp=%d&sig=%s", strings.TrimRight(s.cfg.ImagePublicBaseURL, "/"), publicID, expiresAt, token),
		"expires_at": expiresAt,
	})
}

func (s *Server) updateImagePrivacy(c *gin.Context) {
	if !requireScopes(c, "images:write") {
		return
	}
	identity := currentIdentity(c)
	publicID := c.Param("public_id")
	var req struct {
		Private bool `json:"private"`
	}
	if bind(c, &req) {
		return
	}
	s.state.mu.Lock()
	image, exists := s.state.images[publicID]
	if !exists || image.UserID != identity.UserID || image.Status == "deleted" {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	image.Private = req.Private
	s.state.images[publicID] = image
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, image)
}

func (s *Server) deleteImage(c *gin.Context) {
	if !requireScopes(c, "images:write") {
		return
	}
	identity := currentIdentity(c)
	publicID := c.Param("public_id")
	s.state.mu.Lock()
	image, exists := s.state.images[publicID]
	if !exists || image.UserID != identity.UserID || image.Status == "deleted" {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	image.Status = "deleted"
	s.state.images[publicID] = image
	usage := s.state.usage[identity.UserID]
	storedBytes := image.Bytes + variantBytesTotal(image.Variants)
	if usage.StorageBytes >= storedBytes {
		usage.StorageBytes -= storedBytes
	} else {
		usage.StorageBytes = 0
	}
	s.state.usage[identity.UserID] = usage
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	_ = s.store.DeleteObject(c.Request.Context(), image.ObjectKey)
	for _, variant := range image.Variants {
		_ = s.store.DeleteObject(c.Request.Context(), variant.ObjectKey)
	}
	ok(c, gin.H{"deleted": true, "public_id": publicID})
}

func (s *Server) internalImageAuth(c *gin.Context) {
	originalURI := c.GetHeader("X-Original-URI")
	parsed, err := url.Parse(originalURI)
	if err != nil {
		fail(c, http.StatusForbidden, "invalid_image_uri", "图片地址无效")
		return
	}
	publicID := strings.TrimPrefix(parsed.Path, "/i/")
	if publicID == "" || publicID == parsed.Path {
		fail(c, http.StatusForbidden, "invalid_image_uri", "图片地址无效")
		return
	}
	s.state.mu.RLock()
	image, exists := s.state.images[publicID]
	s.state.mu.RUnlock()
	if !exists {
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	expiresAt, _ := strconv.ParseInt(parsed.Query().Get("exp"), 10, 64)
	hotlink := s.hotlinkSnapshot()
	policy := security.ImageAccessPolicy{
		AllowedDomains:    hotlink.AllowedDomains,
		BlockedDomains:    hotlink.BlockedDomains,
		AllowEmptyReferer: hotlink.AllowEmptyReferer,
		SigningSecret:     s.cfg.ImageSigningSecret,
	}
	result := policy.Evaluate(security.ImageAccessRequest{
		PublicID:  publicID,
		Referer:   c.GetHeader("X-Original-Referer"),
		Token:     parsed.Query().Get("sig"),
		ExpiresAt: expiresAt,
		Private:   image.Private,
		Now:       time.Now(),
	})
	if result.Decision != security.DecisionAllow {
		fail(c, http.StatusForbidden, "image_access_protected", result.Reason)
		return
	}
	ok(c, gin.H{"decision": result.Decision, "reason": result.Reason})
}

func (s *Server) listAlbums(c *gin.Context) {
	if !requireScopes(c, "albums:read") {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	albums := make([]Album, 0, len(s.state.albums))
	for _, album := range s.state.albums {
		if album.UserID == identity.UserID {
			albums = append(albums, album)
		}
	}
	ok(c, albums)
}

func (s *Server) createAlbum(c *gin.Context) {
	if !requireScopes(c, "albums:write") {
		return
	}
	var req struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	}
	if bind(c, &req) {
		return
	}
	identity := currentIdentity(c)
	album := Album{ID: "alb_" + uuid.NewString(), UserID: identity.UserID, Name: fallback(req.Name, "新相册"), Private: req.Private, CreatedAt: time.Now().UTC()}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.albums[album.ID] = album
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, album)
}

func (s *Server) listAPIKeys(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	keys := append([]APIKey(nil), s.state.apiKeys[identity.UserID]...)
	for idx := range keys {
		keys[idx].Secret = ""
	}
	ok(c, keys)
}

func (s *Server) createAPIKey(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if bind(c, &req) {
		return
	}
	identity := currentIdentity(c)
	if !s.userEmailVerified(identity.UserID) {
		fail(c, http.StatusForbidden, "email_not_verified", "请先完成邮箱验证再创建 API Key")
		return
	}
	secret, err := security.GenerateOpaqueToken("yx_", 36)
	if err != nil {
		fail(c, http.StatusInternalServerError, "api_key_generate_failed", "API Key 生成失败")
		return
	}
	scopes, err := normalizeAPIKeyScopes(req.Scopes)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_api_key_scope", err.Error())
		return
	}
	key := APIKey{
		ID:         "key_" + uuid.NewString(),
		UserID:     identity.UserID,
		Name:       fallback(req.Name, "默认 API Key"),
		Prefix:     secret[:10],
		Secret:     secret,
		SecretHash: security.HashSecret(secret, s.cfg.JWTSecret),
		Scopes:     scopes,
		CreatedAt:  time.Now().UTC(),
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.apiKeys[identity.UserID] = append(s.state.apiKeys[identity.UserID], key)
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"api_key": key, "warning": "密钥只显示一次，请妥善保存"})
}

func (s *Server) revokeAPIKey(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	identity := currentIdentity(c)
	keyID := c.Param("id")
	s.state.mu.Lock()
	keys := s.state.apiKeys[identity.UserID]
	for idx, key := range keys {
		if key.ID == keyID {
			snap := snapshotStateLocked(s.state)
			key.Revoked = true
			key.Secret = ""
			keys[idx] = key
			s.state.apiKeys[identity.UserID] = keys
			s.state.mu.Unlock()
			if !s.persistOrRollback(c, snap) {
				return
			}
			ok(c, gin.H{"revoked": true, "api_key": key})
			return
		}
	}
	s.state.mu.Unlock()
	fail(c, http.StatusNotFound, "api_key_not_found", "API Key 不存在")
}

func (s *Server) exportBackup(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.RLock()
	users := make([]User, 0, len(s.state.users))
	if user, exists := s.state.users[identity.UserID]; exists {
		users = append(users, user)
	}
	images := make([]Image, 0, len(s.state.images))
	for _, image := range s.state.images {
		if image.UserID == identity.UserID {
			images = append(images, image)
		}
	}
	s.state.mu.RUnlock()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifest := backup.NewManifest(len(users), len(images))
	files := map[string][]byte{}
	files["database/users.ndjson"] = ndjsonBytes(users)
	files["database/images.ndjson"] = ndjsonBytes(images)
	files["settings/public-config.json"] = []byte(`{"app":"yuexiang-image","secrets_exported":false}`)
	for _, image := range images {
		if image.Status == "deleted" {
			continue
		}
		data, _, err := s.store.GetObject(c.Request.Context(), image.ObjectKey)
		if err != nil {
			continue
		}
		objectPath := fmt.Sprintf("objects/%s/%s", image.PublicID, filepath.Base(image.Filename))
		files[objectPath] = data
		manifest.Objects = append(manifest.Objects, backup.ObjectInfo{
			Path:   objectPath,
			SHA256: backup.SHA256Hex(data),
			Bytes:  int64(len(data)),
		})
	}
	manifestData, _ := manifest.JSON()
	files["manifest.json"] = manifestData
	files["checksums.sha256"] = []byte(checksumFile(files))
	for _, name := range sortedFileNames(files) {
		writeZipBytes(zw, name, files[name])
	}
	_ = zw.Close()
	c.Header("Content-Disposition", `attachment; filename="yuexiang-backup.zip"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func (s *Server) importBackup(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	identity := currentIdentity(c)
	data, err := readZipUpload(c)
	if err != nil {
		fail(c, http.StatusBadRequest, "backup_required", err.Error())
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_backup_zip", "ZIP 文件无效")
		return
	}
	var manifestFound bool
	var checksumFound bool
	var fileNames []string
	fileData := map[string][]byte{}
	for _, file := range zr.File {
		fileNames = append(fileNames, file.Name)
		data, err := readZipFile(file)
		if err != nil {
			fail(c, http.StatusBadRequest, "backup_read_failed", "备份包文件读取失败")
			return
		}
		fileData[file.Name] = data
		if file.Name == "manifest.json" {
			manifestFound = true
		}
		if file.Name == "checksums.sha256" {
			checksumFound = true
		}
	}
	if !manifestFound {
		fail(c, http.StatusBadRequest, "manifest_missing", "备份包缺少 manifest.json")
		return
	}
	if checksumFound {
		if err := validateChecksumFile(string(fileData["checksums.sha256"]), fileData); err != nil {
			fail(c, http.StatusBadRequest, "checksum_invalid", err.Error())
			return
		}
	}
	s.state.mu.RLock()
	snap := snapshotStateLocked(s.state)
	s.state.mu.RUnlock()
	restored, restoredObjectKeys, err := s.restoreBackupImages(c.Request.Context(), identity.UserID, fileData)
	if err != nil {
		fail(c, http.StatusBadRequest, "backup_restore_failed", err.Error())
		return
	}
	if restored > 0 && !s.persistOrRollbackWithCleanup(c, snap, func() {
		s.cleanupObjects(c.Request.Context(), restoredObjectKeys)
	}) {
		return
	}
	ok(c, gin.H{
		"status":         map[bool]string{true: "imported", false: "validated"}[restored > 0],
		"manifest_found": manifestFound,
		"checksum_found": checksumFound,
		"file_count":     len(zr.File),
		"files":          fileNames,
		"restored":       restored,
	})
}

func (s *Server) redeemInvite(c *gin.Context) {
	if !requireSession(c) {
		return
	}
	code := c.Param("code")
	var req struct {
		DeviceID  string `json:"device_id"`
		IsNewUser bool   `json:"is_new_user"`
	}
	if bind(c, &req) {
		return
	}
	identity := currentIdentity(c)
	s.state.mu.Lock()
	user, userExists := s.state.users[identity.UserID]
	if !userExists {
		s.state.mu.Unlock()
		fail(c, http.StatusUnauthorized, "user_not_found", "用户不存在")
		return
	}
	campaign, exists := s.state.invites[code]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "invite_not_found", "邀请链接不存在")
		return
	}
	usage := s.inviteUsageLocked(code, user.ID, user.Email, c.ClientIP(), req.DeviceID)
	if err := campaign.ValidateRedeem(domain.InviteRedeemContext{
		Now:           time.Now().UTC(),
		IsNewUser:     req.IsNewUser,
		EmailVerified: user.EmailVerified,
		OAuthBound:    user.OAuthBound,
		Usage:         usage,
	}); err != nil {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "invite_rejected", err.Error())
		return
	}
	redemption := InviteRedemption{
		ID:        "red_" + uuid.NewString(),
		Code:      code,
		UserID:    user.ID,
		Email:     strings.ToLower(user.Email),
		IP:        c.ClientIP(),
		DeviceID:  req.DeviceID,
		PlanSlug:  campaign.PlanSlug,
		CreatedAt: time.Now().UTC(),
	}
	snap := snapshotStateLocked(s.state)
	s.state.redemptions = append(s.state.redemptions, redemption)
	user.PlanSlug = campaign.PlanSlug
	s.state.users[user.ID] = user
	s.state.subscriptions[user.ID] = domain.NewSubscription("sub_"+uuid.NewString(), user.ID, campaign.PlanSlug, time.Now().UTC(), time.Duration(campaign.GrantDays)*24*time.Hour)
	s.auditLocked("system", "invite.redeem", code, map[string]any{"user_id": user.ID, "plan_slug": campaign.PlanSlug})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"redemption": redemption, "plan_slug": campaign.PlanSlug, "grant_days": campaign.GrantDays})
}

func (s *Server) adminOverview(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	var storageBytes, bandwidthBytes, revenueCent int64
	var activeSubscriptions, pendingOrders, paidOrders, frozenImages, deletedImages int
	for _, image := range s.state.images {
		storageBytes += image.Bytes
		switch image.Status {
		case "frozen":
			frozenImages++
		case "deleted":
			deletedImages++
		}
	}
	now := time.Now().UTC()
	for _, usage := range s.state.usage {
		bandwidthBytes += usage.BandwidthBytes
	}
	for _, sub := range s.state.subscriptions {
		if sub.EffectiveStatus(now) == domain.SubscriptionActive {
			activeSubscriptions++
		}
	}
	for _, order := range s.state.orders {
		switch order.Status {
		case "paid":
			paidOrders++
			revenueCent += order.AmountCent
		case "pending":
			pendingOrders++
		}
	}
	ok(c, gin.H{
		"users":                len(s.state.users),
		"images":               len(s.state.images),
		"storage_bytes":        storageBytes,
		"bandwidth_bytes":      bandwidthBytes,
		"risk_events":          len(s.state.riskEvents),
		"orders":               len(s.state.orders),
		"paid_orders":          paidOrders,
		"pending_orders":       pendingOrders,
		"revenue_cent":         revenueCent,
		"active_subscriptions": activeSubscriptions,
		"frozen_images":        frozenImages,
		"deleted_images":       deletedImages,
	})
}

func (s *Server) restoreBackupImages(ctx context.Context, userID string, files map[string][]byte) (int, []string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, nil, fmt.Errorf("用户不存在")
	}
	s.state.mu.RLock()
	_, userExists := s.state.users[userID]
	s.state.mu.RUnlock()
	if !userExists {
		return 0, nil, fmt.Errorf("用户不存在")
	}
	rawImages := bytes.TrimSpace(files["database/images.ndjson"])
	if len(rawImages) == 0 {
		return 0, nil, nil
	}
	lines := bytes.Split(rawImages, []byte("\n"))
	type restoredImage struct {
		image       Image
		oldPublicID string
		objectPath  string
	}
	var restored []restoredImage
	var restoredObjectKeys []string
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var source Image
		if err := json.Unmarshal(line, &source); err != nil {
			s.cleanupObjects(ctx, restoredObjectKeys)
			return 0, nil, fmt.Errorf("images.ndjson 解析失败: %w", err)
		}
		if source.Status == "deleted" {
			continue
		}
		objectPath, objectBody := findBackupObject(files, source.PublicID)
		if len(objectBody) == 0 {
			continue
		}
		newPublicID := "img_" + uuid.NewString()
		objectKey := "restored/" + newPublicID + "-" + filepath.Base(source.Filename)
		contentType := source.ContentType
		if contentType == "" {
			contentType = http.DetectContentType(objectBody)
		}
		if err := s.store.PutObject(ctx, storage.PutObjectInput{
			Key:         objectKey,
			Body:        bytes.NewReader(objectBody),
			ContentType: contentType,
			Private:     true,
		}); err != nil {
			s.cleanupObjects(ctx, restoredObjectKeys)
			return 0, nil, err
		}
		restoredObjectKeys = append(restoredObjectKeys, objectKey)
		image := source
		image.ID = uuid.NewString()
		image.PublicID = newPublicID
		image.UserID = userID
		image.ObjectKey = objectKey
		image.ContentType = contentType
		image.Bytes = int64(len(objectBody))
		image.Status = "active"
		image.ModerationReason = ""
		image.Variants = nil
		image.CreatedAt = time.Now().UTC()
		restored = append(restored, restoredImage{image: image, oldPublicID: source.PublicID, objectPath: objectPath})
	}
	if len(restored) == 0 {
		return 0, nil, nil
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	usage := s.state.usage[userID]
	for _, item := range restored {
		image := item.image
		s.state.images[image.PublicID] = image
		usage.StorageBytes += image.Bytes
		s.auditLocked("system", "backup.restore_image", image.PublicID, map[string]any{
			"user_id":       userID,
			"old_public_id": item.oldPublicID,
			"object_path":   item.objectPath,
		})
	}
	s.state.usage[userID] = usage
	return len(restored), restoredObjectKeys, nil
}

func (s *Server) adminUsers(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	users := make([]gin.H, 0, len(s.state.users))
	for _, user := range s.state.users {
		users = append(users, gin.H{
			"id":             user.ID,
			"email":          user.Email,
			"nickname":       user.Nickname,
			"email_verified": user.EmailVerified,
			"oauth_bound":    user.OAuthBound,
			"plan_slug":      user.PlanSlug,
			"status":         user.Status,
			"created_at":     user.CreatedAt,
			"usage":          s.state.usage[user.ID],
			"subscription":   s.state.subscriptions[user.ID],
		})
	}
	ok(c, users)
}

func (s *Server) adminGrantPlan(c *gin.Context) {
	userID := c.Param("id")
	if decoded, err := url.PathUnescape(userID); err == nil {
		userID = decoded
	}
	var req struct {
		PlanSlug string `json:"plan_slug"`
		Days     int    `json:"days"`
		Reason   string `json:"reason"`
	}
	if bind(c, &req) {
		return
	}
	if req.Days <= 0 {
		fail(c, http.StatusBadRequest, "invalid_grant_days", "赠送天数必须大于 0")
		return
	}
	s.state.mu.Lock()
	user, exists := s.state.users[userID]
	if !exists {
		for _, candidate := range s.state.users {
			if strings.EqualFold(candidate.Email, userID) {
				user = candidate
				exists = true
				break
			}
		}
	}
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	}
	plan, exists := domain.FindPlan(s.state.plans, req.PlanSlug)
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "plan_not_found", "套餐不存在")
		return
	}
	if plan.Slug == "infinite-max" && strings.TrimSpace(req.Reason) == "" {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "grant_reason_required", "发放 Infinite Max 必须填写审计原因")
		return
	}
	snap := snapshotStateLocked(s.state)
	user.PlanSlug = plan.Slug
	s.state.users[user.ID] = user
	s.state.subscriptions[user.ID] = domain.NewSubscription("sub_"+uuid.NewString(), user.ID, plan.Slug, time.Now().UTC(), time.Duration(req.Days)*24*time.Hour)
	s.auditLocked("admin", "plan.grant", user.ID, map[string]any{
		"plan_slug": plan.Slug,
		"days":      req.Days,
		"reason":    req.Reason,
	})
	sub := s.state.subscriptions[user.ID]
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"user": user, "subscription": sub})
}

func (s *Server) adminPlans(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	ok(c, s.state.plans)
}

func (s *Server) adminUpsertPlan(c *gin.Context) {
	var req domain.Plan
	if bind(c, &req) {
		return
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	req.Name = strings.TrimSpace(req.Name)
	if req.Visibility == "" {
		req.Visibility = domain.PlanVisible
	}
	if err := validatePlan(req); err != nil {
		fail(c, http.StatusBadRequest, "invalid_plan", err.Error())
		return
	}
	s.state.mu.Lock()
	for i, plan := range s.state.plans {
		if plan.Slug == req.Slug {
			snap := snapshotStateLocked(s.state)
			s.state.plans[i] = req
			s.auditLocked("admin", "plan.update", req.Slug, nil)
			s.state.mu.Unlock()
			if !s.persistOrRollback(c, snap) {
				return
			}
			ok(c, req)
			return
		}
	}
	snap := snapshotStateLocked(s.state)
	s.state.plans = append(s.state.plans, req)
	s.auditLocked("admin", "plan.create", req.Slug, nil)
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, req)
}

func (s *Server) adminInvites(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	invites := make([]domain.InviteCampaign, 0, len(s.state.invites))
	for _, invite := range s.state.invites {
		invites = append(invites, invite)
	}
	ok(c, gin.H{"campaigns": invites, "redemptions": s.state.redemptions})
}

func (s *Server) adminCreateInvite(c *gin.Context) {
	var req domain.InviteCampaign
	if bind(c, &req) {
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.PlanSlug = strings.TrimSpace(req.PlanSlug)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "invalid_invite", "活动名称不能为空")
		return
	}
	if req.PlanSlug == "" {
		fail(c, http.StatusBadRequest, "invalid_invite", "邀请活动必须指定套餐")
		return
	}
	if req.GrantDays <= 0 {
		fail(c, http.StatusBadRequest, "invalid_invite", "赠送天数必须大于 0")
		return
	}
	if req.ID == "" {
		req.ID = "inv_" + uuid.NewString()
	}
	if req.Code == "" {
		req.Code = "invite-" + uuid.NewString()[:8]
	}
	if req.StartsAt.IsZero() {
		req.StartsAt = time.Now().UTC()
	}
	if req.EndsAt.IsZero() {
		req.EndsAt = req.StartsAt.Add(30 * 24 * time.Hour)
	}
	if req.Status == "" {
		req.Status = domain.InviteActive
	}
	s.state.mu.Lock()
	plan, exists := domain.FindPlan(s.state.plans, req.PlanSlug)
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "plan_not_found", "套餐不存在")
		return
	}
	if plan.Slug == "infinite-max" && !req.RequireAdminApproval {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "admin_approval_required", "Infinite Max 邀请必须启用管理员审批")
		return
	}
	if _, exists := s.state.invites[req.Code]; exists {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "invite_code_exists", "邀请码已存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	s.state.invites[req.Code] = req
	s.auditLocked("admin", "invite.create", req.Code, map[string]any{"plan_slug": req.PlanSlug})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, req)
}

func (s *Server) adminOrders(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	orders := make([]Order, 0, len(s.state.orders))
	for _, order := range s.state.orders {
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].CreatedAt.After(orders[j].CreatedAt) })
	ok(c, orders)
}

func (s *Server) adminMarkOrderPaid(c *gin.Context) {
	orderID := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if bind(c, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		fail(c, http.StatusBadRequest, "reason_required", "人工入账必须填写对账原因")
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	order, exists := s.state.orders[orderID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "order_not_found", "订单不存在")
		return
	}
	if order.Status == "cancelled" || order.Status == "refunded" {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_not_payable", "已取消或已退款订单不可入账")
		return
	}
	snap := snapshotStateLocked(s.state)
	order.OperatorNote = reason
	order, sub, err := s.activateOrderLocked(order, now, "admin", "order.mark_paid", map[string]any{"reason": reason})
	if err != nil {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_not_payable", err.Error())
		return
	}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"order": order, "subscription": sub})
}

func (s *Server) adminCancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if bind(c, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		fail(c, http.StatusBadRequest, "reason_required", "取消订单必须填写原因")
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	order, exists := s.state.orders[orderID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "order_not_found", "订单不存在")
		return
	}
	if order.Status == "paid" {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_paid", "已支付订单请走退款流程")
		return
	}
	if order.Status == "refunded" {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_refunded", "已退款订单不可取消")
		return
	}
	snap := snapshotStateLocked(s.state)
	order.OperatorNote = reason
	order, err := s.cancelOrderLocked(order, now, "admin", "order.cancel", reason, nil)
	if err != nil {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_not_cancellable", err.Error())
		return
	}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, order)
}

func (s *Server) adminRefundOrder(c *gin.Context) {
	orderID := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if bind(c, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		fail(c, http.StatusBadRequest, "reason_required", "退款必须填写原因")
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	order, exists := s.state.orders[orderID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "order_not_found", "订单不存在")
		return
	}
	if order.Status != "paid" {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_not_paid", "仅已支付订单可退款")
		return
	}
	snap := snapshotStateLocked(s.state)
	order.OperatorNote = reason
	order, sub, _, err := s.refundOrderLocked(order, now, "admin", "order.refund", reason, nil)
	if err != nil {
		s.state.mu.Unlock()
		fail(c, http.StatusConflict, "order_not_refundable", err.Error())
		return
	}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"order": order, "subscription": sub})
}

func (s *Server) adminRiskEvents(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	ok(c, s.state.riskEvents)
}

func (s *Server) findWebhookOrderLocked(resourceID string, payload map[string]any) (Order, bool) {
	orderID := stringFromMap(payload, "order_id")
	paymentID := fallback(strings.TrimSpace(resourceID), stringFromMap(payload, "payment_id"))
	for _, order := range s.state.orders {
		if order.ID == orderID || (paymentID != "" && order.IFPayPaymentID == paymentID) {
			return order, true
		}
	}
	return Order{}, false
}

func (s *Server) activateOrderLocked(order Order, now time.Time, actor, action string, metadata map[string]any) (Order, domain.Subscription, error) {
	if order.Status == "cancelled" || order.Status == "refunded" {
		return order, domain.Subscription{}, fmt.Errorf("订单状态 %s 不可入账", order.Status)
	}
	user, exists := s.state.users[order.UserID]
	if !exists {
		return order, domain.Subscription{}, fmt.Errorf("订单所属用户不存在")
	}
	existingSub, hasExistingSub := s.state.subscriptions[user.ID]
	alreadyPaid := order.Status == "paid"
	needsActivation := !alreadyPaid || !hasExistingSub
	order.Status = "paid"
	order.FailedAt = nil
	if order.PaidAt == nil {
		order.PaidAt = &now
	}
	s.state.orders[order.ID] = order

	sub := existingSub
	if needsActivation {
		user.PlanSlug = order.PlanSlug
		s.state.users[user.ID] = user
		sub = domain.NewSubscription("sub_"+uuid.NewString(), user.ID, order.PlanSlug, now, billingDuration(order.BillingCycle))
		s.state.subscriptions[user.ID] = sub
	}
	s.auditLocked(actor, action, order.ID, mergeMetadata(metadata, map[string]any{
		"user_id":       user.ID,
		"plan_slug":     order.PlanSlug,
		"billing_cycle": order.BillingCycle,
		"activated":     needsActivation,
	}))
	return order, sub, nil
}

func (s *Server) failOrderLocked(order Order, now time.Time, actor, action, reason string, metadata map[string]any) (Order, error) {
	if order.Status == "paid" || order.Status == "cancelled" || order.Status == "refunded" {
		return order, fmt.Errorf("订单状态 %s 不可标记失败", order.Status)
	}
	order.Status = "failed"
	if order.FailedAt == nil {
		order.FailedAt = &now
	}
	if reason != "" {
		order.OperatorNote = reason
	}
	s.state.orders[order.ID] = order
	s.auditLocked(actor, action, order.ID, mergeMetadata(metadata, map[string]any{
		"user_id":   order.UserID,
		"plan_slug": order.PlanSlug,
		"reason":    reason,
	}))
	return order, nil
}

func (s *Server) cancelOrderLocked(order Order, now time.Time, actor, action, reason string, metadata map[string]any) (Order, error) {
	if order.Status == "paid" {
		return order, fmt.Errorf("已支付订单请走退款流程")
	}
	if order.Status == "refunded" {
		return order, fmt.Errorf("已退款订单不可取消")
	}
	order.Status = "cancelled"
	if order.CancelledAt == nil {
		order.CancelledAt = &now
	}
	if reason != "" {
		order.OperatorNote = reason
	}
	s.state.orders[order.ID] = order
	s.auditLocked(actor, action, order.ID, mergeMetadata(metadata, map[string]any{
		"user_id":   order.UserID,
		"plan_slug": order.PlanSlug,
		"reason":    reason,
	}))
	return order, nil
}

func (s *Server) refundOrderLocked(order Order, now time.Time, actor, action, reason string, metadata map[string]any) (Order, domain.Subscription, bool, error) {
	if order.Status != "paid" {
		return order, domain.Subscription{}, false, fmt.Errorf("仅已支付订单可退款")
	}
	order.Status = "refunded"
	if order.RefundedAt == nil {
		order.RefundedAt = &now
	}
	if reason != "" {
		order.OperatorNote = reason
	}
	s.state.orders[order.ID] = order
	sub, hasSub := s.state.subscriptions[order.UserID]
	retained := hasSub && sub.PlanSlug == order.PlanSlug
	if retained {
		sub.Status = domain.SubscriptionPastDue
		sub.EndsAt = now.Add(-time.Second)
		sub.RetentionEndsAt = now.Add(30 * 24 * time.Hour)
		s.state.subscriptions[order.UserID] = sub
		if user, exists := s.state.users[order.UserID]; exists {
			user.PlanSlug = ""
			s.state.users[user.ID] = user
		}
	}
	s.auditLocked(actor, action, order.ID, mergeMetadata(metadata, map[string]any{
		"user_id":               order.UserID,
		"plan_slug":             order.PlanSlug,
		"reason":                reason,
		"subscription_retained": retained,
	}))
	return order, sub, retained, nil
}

func (s *Server) adminHotlinkConfig(c *gin.Context) {
	hotlink := s.hotlinkSnapshot()
	ok(c, gin.H{
		"allowed_domains":     hotlink.AllowedDomains,
		"blocked_domains":     hotlink.BlockedDomains,
		"allow_empty_referer": hotlink.AllowEmptyReferer,
		"signing_enabled":     s.cfg.ImageSigningSecret != "",
		"updated_at":          hotlink.UpdatedAt,
	})
}

func (s *Server) adminUpdateHotlinkConfig(c *gin.Context) {
	var req struct {
		AllowedDomains    []string `json:"allowed_domains"`
		BlockedDomains    []string `json:"blocked_domains"`
		AllowEmptyReferer bool     `json:"allow_empty_referer"`
	}
	if bind(c, &req) {
		return
	}
	hotlink := HotlinkConfig{
		AllowedDomains:    normalizeDomains(req.AllowedDomains),
		BlockedDomains:    normalizeDomains(req.BlockedDomains),
		AllowEmptyReferer: req.AllowEmptyReferer,
		UpdatedAt:         time.Now().UTC(),
	}
	if len(hotlink.AllowedDomains) == 0 && !hotlink.AllowEmptyReferer {
		fail(c, http.StatusBadRequest, "invalid_hotlink_policy", "拒绝空 Referer 时必须配置至少一个允许域名")
		return
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.hotlink = hotlink
	s.auditLocked("admin", "security.hotlink.update", "hotlink", map[string]any{
		"allowed_domains":      hotlink.AllowedDomains,
		"blocked_domain_count": len(hotlink.BlockedDomains),
		"allow_empty_referer":  hotlink.AllowEmptyReferer,
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, hotlink)
}

func (s *Server) adminStorageConfig(c *gin.Context) {
	ok(c, storage.S3CompatibleConfig{
		Endpoint:       s.cfg.S3Endpoint,
		Region:         s.cfg.S3Region,
		Bucket:         s.cfg.S3Bucket,
		ForcePathStyle: s.cfg.S3ForcePathStyle,
	})
}

func (s *Server) adminSystemConfig(c *gin.Context) {
	integration := s.integrationSnapshot()
	ok(c, gin.H{
		"app_env":               s.cfg.AppEnv,
		"public_base_url":       s.cfg.PublicBaseURL,
		"image_public_base_url": s.cfg.ImagePublicBaseURL,
		"cors_allowed_origins":  splitCSV(s.cfg.CORSAllowedOrigins),
		"storage_driver":        s.cfg.StorageDriver,
		"queue_driver":          s.cfg.QueueDriver,
		"queue_stream":          s.cfg.QueueStream,
		"queue_dead_letter":     s.cfg.QueueDeadLetterStream,
		"worker_retry_limit":    s.cfg.WorkerRetryLimit,
		"worker_claim_idle":     s.cfg.WorkerClaimIdle.String(),
		"database_enabled":      s.cfg.DatabaseURL != "",
		"redis_addr":            s.cfg.RedisAddr,
		"smtp_configured":       s.cfg.SMTPHost != "",
		"ifpay_configured":      strings.TrimSpace(integration.IFPayBaseURL) != "" && strings.TrimSpace(integration.IFPayClientID) != "",
		"moderation_hash_count": len(splitCSV(s.cfg.ModerationBlockedHashes)),
		"rate_limits": gin.H{
			"default_per_minute":        600,
			"image_upload_per_minute":   120,
			"ifpay_checkout_per_minute": 30,
			"login_per_minute":          20,
		},
	})
}

func (s *Server) adminQueueStatus(c *gin.Context) {
	base := gin.H{
		"reachable":          false,
		"driver":             s.cfg.QueueDriver,
		"stream":             s.cfg.QueueStream,
		"dead_letter_stream": s.cfg.QueueDeadLetterStream,
		"group":              s.cfg.QueueGroup,
		"worker_name":        s.cfg.WorkerName,
		"worker_retry_limit": s.cfg.WorkerRetryLimit,
		"worker_claim_idle":  s.cfg.WorkerClaimIdle.String(),
	}
	if s.cfg.QueueDriver != "redis" {
		base["error"] = "queue driver is not redis"
		ok(c, base)
		return
	}
	stats, err := queue.InspectRedis(c.Request.Context(), queue.RedisConfig{
		Addr:             s.cfg.RedisAddr,
		DB:               s.cfg.RedisDB,
		Stream:           s.cfg.QueueStream,
		DeadLetterStream: s.cfg.QueueDeadLetterStream,
	}, s.cfg.QueueGroup)
	if err != nil {
		base["error"] = err.Error()
		ok(c, base)
		return
	}
	ok(c, gin.H{
		"reachable":          stats.Reachable,
		"driver":             s.cfg.QueueDriver,
		"stream":             stats.Stream,
		"dead_letter_stream": stats.DeadLetterStream,
		"group":              stats.Group,
		"length":             stats.Length,
		"dead_letter_length": stats.DeadLetterLength,
		"pending":            stats.Pending,
		"lag":                stats.Lag,
		"last_generated_id":  stats.LastGeneratedID,
		"consumers":          stats.Consumers,
		"worker_name":        s.cfg.WorkerName,
		"worker_retry_limit": s.cfg.WorkerRetryLimit,
		"worker_claim_idle":  s.cfg.WorkerClaimIdle.String(),
	})
}

func (s *Server) adminQueueDeadLetters(c *gin.Context) {
	if s.cfg.QueueDriver != "redis" {
		fail(c, http.StatusBadRequest, "queue_not_redis", "当前未启用 Redis 队列")
		return
	}
	limit := int64(20)
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	messages, err := queue.ListDeadLetters(c.Request.Context(), queue.RedisConfig{
		Addr:             s.cfg.RedisAddr,
		DB:               s.cfg.RedisDB,
		Stream:           s.cfg.QueueStream,
		DeadLetterStream: s.cfg.QueueDeadLetterStream,
	}, limit)
	if err != nil {
		fail(c, http.StatusBadGateway, "queue_dead_letter_unavailable", err.Error())
		return
	}
	ok(c, gin.H{"messages": messages})
}

func (s *Server) adminRequeueDeadLetter(c *gin.Context) {
	if s.cfg.QueueDriver != "redis" {
		fail(c, http.StatusBadRequest, "queue_not_redis", "当前未启用 Redis 队列")
		return
	}
	id := c.Param("id")
	if err := queue.RequeueDeadLetter(c.Request.Context(), queue.RedisConfig{
		Addr:             s.cfg.RedisAddr,
		DB:               s.cfg.RedisDB,
		Stream:           s.cfg.QueueStream,
		DeadLetterStream: s.cfg.QueueDeadLetterStream,
	}, id); err != nil {
		fail(c, http.StatusBadGateway, "queue_requeue_failed", err.Error())
		return
	}
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.auditLocked("admin", "queue.dead_letter.requeue", id, map[string]any{
		"stream":      s.cfg.QueueStream,
		"dead_stream": s.cfg.QueueDeadLetterStream,
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"status": "requeued", "id": id})
}

func (s *Server) adminCDNConfig(c *gin.Context) {
	ok(c, gin.H{
		"image_public_base_url": s.cfg.ImagePublicBaseURL,
		"origin_auth_endpoint":  "/api/v1/internal/image-auth",
		"image_route":           "/i/{public_id}",
		"variant_route":         "/i/{public_id}/{variant}",
		"cache_strategy":        "public images cacheable, private signed images vary by query token",
		"cloudflare_rules_doc":  "deploy/cloudflare/rules.md",
		"nginx_config":          "deploy/nginx/yuexiang-image.conf",
	})
}

func (s *Server) adminAPIConfig(c *gin.Context) {
	integration := s.integrationSnapshot()
	ok(c, gin.H{
		"public_base_url":    s.cfg.PublicBaseURL,
		"openapi":            "/docs/openapi.yaml",
		"metrics":            "/metrics",
		"ifpay_oauth_start":  "/api/v1/oauth/ifpay/start",
		"ifpay_webhook":      "/api/v1/ifpay/webhooks/payments",
		"ifpay_configured":   strings.TrimSpace(integration.IFPayBaseURL) != "" && strings.TrimSpace(integration.IFPayClientID) != "",
		"default_api_scopes": []string{"images:read", "images:write", "albums:read", "albums:write"},
	})
}

func (s *Server) adminIFPayConfig(c *gin.Context) {
	ok(c, s.integrationSnapshot().Sanitized())
}

func (s *Server) adminUpdateIFPayConfig(c *gin.Context) {
	var req struct {
		IFPayBaseURL          *string `json:"ifpay_base_url"`
		IFPayPartnerAppID     *string `json:"ifpay_partner_app_id"`
		IFPayClientID         *string `json:"ifpay_client_id"`
		IFPayClientSecret     *string `json:"ifpay_client_secret"`
		IFPayPrivateKeyPEM    *string `json:"ifpay_private_key_pem"`
		IFPayPublicKeyPEM     *string `json:"ifpay_public_key_pem"`
		IFPayWebhookPublicKey *string `json:"ifpay_webhook_public_key_pem"`
		IFPayRedirectURI      *string `json:"ifpay_redirect_uri"`
	}
	if bind(c, &req) {
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	next := s.state.integrations.WithFallback(s.cfg)
	if req.IFPayBaseURL != nil {
		next.IFPayBaseURL = strings.TrimSpace(*req.IFPayBaseURL)
	}
	if req.IFPayPartnerAppID != nil {
		next.IFPayPartnerAppID = strings.TrimSpace(*req.IFPayPartnerAppID)
	}
	if req.IFPayClientID != nil {
		next.IFPayClientID = strings.TrimSpace(*req.IFPayClientID)
	}
	if req.IFPayClientSecret != nil && strings.TrimSpace(*req.IFPayClientSecret) != "" {
		next.IFPayClientSecret = strings.TrimSpace(*req.IFPayClientSecret)
	}
	if req.IFPayPrivateKeyPEM != nil && strings.TrimSpace(*req.IFPayPrivateKeyPEM) != "" {
		next.IFPayPrivateKeyPEM = strings.TrimSpace(*req.IFPayPrivateKeyPEM)
	}
	if req.IFPayPublicKeyPEM != nil && strings.TrimSpace(*req.IFPayPublicKeyPEM) != "" {
		next.IFPayPublicKeyPEM = strings.TrimSpace(*req.IFPayPublicKeyPEM)
	}
	if req.IFPayWebhookPublicKey != nil && strings.TrimSpace(*req.IFPayWebhookPublicKey) != "" {
		next.IFPayWebhookPublicKey = strings.TrimSpace(*req.IFPayWebhookPublicKey)
	}
	if req.IFPayRedirectURI != nil {
		next.IFPayRedirectURI = strings.TrimSpace(*req.IFPayRedirectURI)
	}
	if next.IFPayPartnerAppID == "" {
		next.IFPayPartnerAppID = "yuexiang-image"
	}
	if next.IFPayRedirectURI == "" {
		next.IFPayRedirectURI = s.cfg.IFPayRedirectURI
	}
	if next.IFPayBaseURL != "" && !(strings.HasPrefix(next.IFPayBaseURL, "https://") || strings.HasPrefix(next.IFPayBaseURL, "http://localhost") || strings.HasPrefix(next.IFPayBaseURL, "http://127.0.0.1")) {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "invalid_ifpay_base_url", "IF-Pay Base URL 必须是 HTTPS，或本地 localhost/127.0.0.1 开发地址")
		return
	}
	if next.IFPayRedirectURI != "" && !(strings.HasPrefix(next.IFPayRedirectURI, "https://") || strings.HasPrefix(next.IFPayRedirectURI, "http://localhost") || strings.HasPrefix(next.IFPayRedirectURI, "http://127.0.0.1")) {
		s.state.mu.Unlock()
		fail(c, http.StatusBadRequest, "invalid_ifpay_redirect_uri", "IF-Pay Redirect URI 必须是 HTTPS，或本地 localhost/127.0.0.1 开发地址")
		return
	}
	next.UpdatedAt = now
	s.state.integrations = next
	s.auditLocked("admin", "integration.ifpay.update", "ifpay", map[string]any{
		"base_url":                next.IFPayBaseURL,
		"client_id_configured":    next.IFPayClientID != "",
		"payment_signing":         next.IFPayPrivateKeyPEM != "",
		"webhook_verification":    next.IFPayWebhookPublicKey != "",
		"redirect_uri_configured": next.IFPayRedirectURI != "",
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, next.Sanitized())
}

func (s *Server) adminExportBackup(c *gin.Context) {
	s.state.mu.RLock()
	plans := append([]domain.Plan(nil), s.state.plans...)
	users := make([]User, 0, len(s.state.users))
	for _, user := range s.state.users {
		users = append(users, user)
	}
	images := make([]Image, 0, len(s.state.images))
	for _, image := range s.state.images {
		images = append(images, image)
	}
	albums := make([]Album, 0, len(s.state.albums))
	for _, album := range s.state.albums {
		albums = append(albums, album)
	}
	apiKeys := make([]APIKey, 0)
	for _, keys := range s.state.apiKeys {
		for _, key := range keys {
			key.Secret = ""
			apiKeys = append(apiKeys, key)
		}
	}
	orders := make([]Order, 0, len(s.state.orders))
	for _, order := range s.state.orders {
		orders = append(orders, order)
	}
	subscriptions := make([]domain.Subscription, 0, len(s.state.subscriptions))
	for _, sub := range s.state.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	usage := make(map[string]domain.Usage, len(s.state.usage))
	for userID, item := range s.state.usage {
		usage[userID] = item
	}
	invites := make([]domain.InviteCampaign, 0, len(s.state.invites))
	for _, invite := range s.state.invites {
		invites = append(invites, invite)
	}
	redemptions := append([]InviteRedemption(nil), s.state.redemptions...)
	riskEvents := append([]RiskEvent(nil), s.state.riskEvents...)
	auditLogs := append([]AuditLog(nil), s.state.auditLogs...)
	webhooks := make([]string, 0, len(s.state.webhooks))
	for id := range s.state.webhooks {
		webhooks = append(webhooks, id)
	}
	s.state.mu.RUnlock()

	files := map[string][]byte{
		"database/plans.ndjson":         ndjsonBytes(plans),
		"database/users.ndjson":         ndjsonBytes(users),
		"database/images.ndjson":        ndjsonBytes(images),
		"database/albums.ndjson":        ndjsonBytes(albums),
		"database/api_keys.ndjson":      ndjsonBytes(apiKeys),
		"database/orders.ndjson":        ndjsonBytes(orders),
		"database/subscriptions.ndjson": ndjsonBytes(subscriptions),
		"database/usage.json":           jsonPrettyBytes(usage),
		"database/invites.ndjson":       ndjsonBytes(invites),
		"database/redemptions.ndjson":   ndjsonBytes(redemptions),
		"database/webhooks.ndjson":      ndjsonBytes(webhooks),
		"database/risk_events.ndjson":   ndjsonBytes(riskEvents),
		"database/audit_logs.ndjson":    ndjsonBytes(auditLogs),
		"settings/public-config.json": jsonPrettyBytes(gin.H{
			"app":                    "yuexiang-image",
			"app_env":                s.cfg.AppEnv,
			"public_base_url":        s.cfg.PublicBaseURL,
			"image_public_base_url":  s.cfg.ImagePublicBaseURL,
			"allowed_referer_domain": splitCSV(s.cfg.AllowedRefererDomains),
			"blocked_referer_domain": splitCSV(s.cfg.BlockedRefererDomains),
			"secrets_exported":       false,
		}),
	}
	manifest := backup.NewManifest(len(users), len(images))
	for _, image := range images {
		if image.Status == "deleted" {
			continue
		}
		s.addBackupObject(c.Request.Context(), files, &manifest, image.PublicID, filepath.Base(image.Filename), image.ObjectKey)
		for _, variant := range image.Variants {
			if variant.ObjectKey == "" {
				continue
			}
			name := "variant-" + variant.Kind + extensionForMime(variant.MimeType)
			s.addBackupObject(c.Request.Context(), files, &manifest, image.PublicID, name, variant.ObjectKey)
		}
	}
	manifestData, _ := manifest.JSON()
	files["manifest.json"] = manifestData
	files["checksums.sha256"] = []byte(checksumFile(files))

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedFileNames(files) {
		writeZipBytes(zw, name, files[name])
	}
	_ = zw.Close()
	c.Header("Content-Disposition", `attachment; filename="yuexiang-admin-backup.zip"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func (s *Server) adminValidateBackup(c *gin.Context) {
	data, err := readZipUpload(c)
	if err != nil {
		fail(c, http.StatusBadRequest, "backup_required", err.Error())
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_backup_zip", "ZIP 文件无效")
		return
	}
	fileData := map[string][]byte{}
	var manifest backup.Manifest
	var manifestFound bool
	var checksumFound bool
	var objectCount int
	for _, file := range zr.File {
		content, err := readZipFile(file)
		if err != nil {
			fail(c, http.StatusBadRequest, "backup_read_failed", "备份包文件读取失败")
			return
		}
		fileData[file.Name] = content
		if file.Name == "manifest.json" {
			manifestFound = true
			_ = json.Unmarshal(content, &manifest)
		}
		if file.Name == "checksums.sha256" {
			checksumFound = true
		}
		if strings.HasPrefix(file.Name, "objects/") {
			objectCount++
		}
	}
	if !manifestFound {
		fail(c, http.StatusBadRequest, "manifest_missing", "备份包缺少 manifest.json")
		return
	}
	if checksumFound {
		if err := validateChecksumFile(string(fileData["checksums.sha256"]), fileData); err != nil {
			fail(c, http.StatusBadRequest, "checksum_invalid", err.Error())
			return
		}
	}
	ok(c, gin.H{
		"status":         "validated",
		"file_count":     len(zr.File),
		"object_count":   objectCount,
		"manifest":       manifest,
		"checksum_found": checksumFound,
	})
}

func (s *Server) adminAuditLogs(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	action := strings.ToLower(strings.TrimSpace(c.Query("action")))
	actor := strings.ToLower(strings.TrimSpace(c.Query("actor")))
	limit := boundedQueryInt(c.Query("limit"), 100, 1, 500)
	offset := boundedQueryInt(c.Query("offset"), 0, 0, 1000000)

	matches := make([]AuditLog, 0, len(s.state.auditLogs))
	for idx := len(s.state.auditLogs) - 1; idx >= 0; idx-- {
		log := s.state.auditLogs[idx]
		if actor != "" && !strings.Contains(strings.ToLower(log.Actor), actor) {
			continue
		}
		if action != "" && !strings.Contains(strings.ToLower(log.Action), action) {
			continue
		}
		if query != "" && !auditLogContains(log, query) {
			continue
		}
		matches = append(matches, log)
	}
	total := len(matches)
	if offset > total {
		matches = []AuditLog{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		matches = matches[offset:end]
	}
	ok(c, gin.H{
		"items":  matches,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) adminImages(c *gin.Context) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	limit := boundedQueryInt(c.Query("limit"), 200, 1, 500)
	offset := boundedQueryInt(c.Query("offset"), 0, 0, 1000000)
	images := make([]Image, 0, len(s.state.images))
	for _, image := range s.state.images {
		if status != "" && strings.ToLower(image.Status) != status {
			continue
		}
		if query != "" && !adminImageContains(image, query) {
			continue
		}
		images = append(images, image)
	}
	sort.Slice(images, func(i, j int) bool { return images[i].CreatedAt.After(images[j].CreatedAt) })
	total := len(images)
	if offset > total {
		images = []Image{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		images = images[offset:end]
	}
	ok(c, gin.H{
		"items":  images,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) adminFreezeImage(c *gin.Context) {
	publicID := c.Param("public_id")
	var req struct {
		Reason string `json:"reason"`
	}
	if bind(c, &req) {
		return
	}
	s.state.mu.Lock()
	image, exists := s.state.images[publicID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	image.Status = "frozen"
	image.ModerationReason = fallback(req.Reason, "管理员冻结")
	s.state.images[publicID] = image
	s.auditLocked("admin", "image.freeze", publicID, map[string]any{"reason": image.ModerationReason, "user_id": image.UserID})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, image)
}

func (s *Server) adminDeleteImage(c *gin.Context) {
	publicID := c.Param("public_id")
	s.state.mu.Lock()
	image, exists := s.state.images[publicID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "image_not_found", "图片不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	image.Status = "deleted"
	s.state.images[publicID] = image
	usage := s.state.usage[image.UserID]
	storedBytes := image.Bytes + variantBytesTotal(image.Variants)
	if usage.StorageBytes >= storedBytes {
		usage.StorageBytes -= storedBytes
	} else {
		usage.StorageBytes = 0
	}
	s.state.usage[image.UserID] = usage
	s.auditLocked("admin", "image.delete", publicID, map[string]any{"user_id": image.UserID})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	_ = s.store.DeleteObject(c.Request.Context(), image.ObjectKey)
	for _, variant := range image.Variants {
		_ = s.store.DeleteObject(c.Request.Context(), variant.ObjectKey)
	}
	ok(c, gin.H{"deleted": true, "public_id": publicID})
}

func (s *Server) adminBanUser(c *gin.Context) {
	s.adminSetUserStatus(c, "banned")
}

func (s *Server) adminUnbanUser(c *gin.Context) {
	s.adminSetUserStatus(c, "active")
}

func (s *Server) adminExpireSubscription(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Reason        string `json:"reason"`
		RetentionDays int    `json:"retention_days"`
		DeleteNow     bool   `json:"delete_now"`
	}
	if bind(c, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		fail(c, http.StatusBadRequest, "reason_required", "强制到期必须填写原因")
		return
	}
	if req.RetentionDays <= 0 {
		req.RetentionDays = 30
	}
	if req.RetentionDays > 365 {
		fail(c, http.StatusBadRequest, "retention_days_too_large", "保留期不能超过 365 天")
		return
	}
	now := time.Now().UTC()
	s.state.mu.Lock()
	user, userExists := s.state.users[userID]
	sub, subExists := s.state.subscriptions[userID]
	if !userExists || !subExists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "subscription_not_found", "用户或订阅不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	sub.EndsAt = now.Add(-time.Second)
	sub.RetentionEndsAt = now.Add(time.Duration(req.RetentionDays) * 24 * time.Hour)
	sub.Status = domain.SubscriptionPastDue
	if req.DeleteNow {
		sub.Status = domain.SubscriptionDeleted
		sub.RetentionEndsAt = now.Add(-time.Second)
	}
	s.state.subscriptions[userID] = sub
	user.PlanSlug = ""
	s.state.users[user.ID] = user
	s.auditLocked("admin", "subscription.expire", userID, map[string]any{
		"reason":         reason,
		"retention_days": req.RetentionDays,
		"delete_now":     req.DeleteNow,
		"plan_slug":      sub.PlanSlug,
	})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, gin.H{"user": user, "subscription": sub})
}

func (s *Server) adminSetUserStatus(c *gin.Context, status string) {
	userID := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	s.state.mu.Lock()
	user, exists := s.state.users[userID]
	if !exists {
		s.state.mu.Unlock()
		fail(c, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	}
	snap := snapshotStateLocked(s.state)
	user.Status = status
	s.state.users[userID] = user
	revokedKeys := 0
	if status == "banned" {
		keys := s.state.apiKeys[userID]
		for idx, key := range keys {
			if key.Revoked {
				continue
			}
			key.Revoked = true
			key.Secret = ""
			keys[idx] = key
			revokedKeys++
		}
		s.state.apiKeys[userID] = keys
	}
	s.auditLocked("admin", "user."+status, userID, map[string]any{"reason": req.Reason, "revoked_api_keys": revokedKeys})
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	ok(c, user)
}

func (s *Server) adminPurgeExpired(c *gin.Context) {
	now := time.Now().UTC()
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	type objectToDelete struct {
		publicID string
		key      string
	}
	var objects []objectToDelete
	var purgedUsers []string
	for userID, sub := range s.state.subscriptions {
		if sub.EffectiveStatus(now) != domain.SubscriptionDeleted {
			continue
		}
		purgedUsers = append(purgedUsers, userID)
		sub.Status = domain.SubscriptionDeleted
		s.state.subscriptions[userID] = sub
		for publicID, image := range s.state.images {
			if image.UserID != userID || image.Status == "deleted" {
				continue
			}
			image.Status = "deleted"
			s.state.images[publicID] = image
			objects = append(objects, objectToDelete{publicID: publicID, key: image.ObjectKey})
			for _, variant := range image.Variants {
				objects = append(objects, objectToDelete{publicID: publicID, key: variant.ObjectKey})
			}
		}
		s.state.usage[userID] = domain.Usage{}
		if user, exists := s.state.users[userID]; exists {
			user.PlanSlug = ""
			s.state.users[userID] = user
		}
		s.auditLocked("system", "subscription.purge_expired", userID, map[string]any{"object_count": len(objects)})
	}
	s.state.mu.Unlock()
	if !s.persistOrRollback(c, snap) {
		return
	}
	for _, object := range objects {
		_ = s.store.DeleteObject(c.Request.Context(), object.key)
	}
	ok(c, gin.H{
		"purged_users":    purgedUsers,
		"deleted_objects": len(objects),
	})
}

func (s *Server) inviteUsageLocked(code, userID, email, ip, deviceID string) domain.InviteUsageSnapshot {
	email = strings.ToLower(strings.TrimSpace(email))
	var usage domain.InviteUsageSnapshot
	for _, redemption := range s.state.redemptions {
		if redemption.Code != code {
			continue
		}
		usage.TotalRedeemed++
		if redemption.UserID == userID {
			usage.UserRedeemed++
		}
		if strings.ToLower(redemption.Email) == email {
			usage.EmailRedeemed++
		}
		if redemption.IP == ip {
			usage.IPRedeemed++
		}
		if redemption.DeviceID == deviceID {
			usage.DeviceRedeemed++
		}
	}
	return usage
}

func (s *Server) recordRisk(kind, message, ip, referer string) {
	s.state.mu.Lock()
	snap := snapshotStateLocked(s.state)
	s.state.riskEvents = append(s.state.riskEvents, RiskEvent{
		ID:        "risk_" + uuid.NewString(),
		Type:      kind,
		Message:   message,
		IP:        ip,
		Referer:   referer,
		CreatedAt: time.Now().UTC(),
	})
	s.state.mu.Unlock()
	if err := s.persistState(context.Background()); err != nil {
		restoreStateSnapshot(s.state, snap)
	}
}

func (s *Server) moderationBlockReasonLocked(publicID string, image Image) string {
	hash := strings.TrimSpace(image.PerceptualHash)
	if hash == "" {
		return ""
	}
	for _, blockedHash := range splitCSV(s.cfg.ModerationBlockedHashes) {
		if strings.EqualFold(hash, blockedHash) {
			return "pHash 命中违规特征库，已自动冻结"
		}
	}
	for otherPublicID, other := range s.state.images {
		if otherPublicID == publicID || other.PerceptualHash == "" || !strings.EqualFold(other.PerceptualHash, hash) {
			continue
		}
		if other.Status == "frozen" && strings.TrimSpace(other.ModerationReason) != "" {
			return "pHash 命中已冻结对象 " + otherPublicID + "，已自动冻结"
		}
	}
	return ""
}

func (s *Server) auditLocked(actor, action, target string, metadata map[string]any) {
	s.state.auditLogs = append(s.state.auditLogs, AuditLog{
		ID:        "audit_" + uuid.NewString(),
		Actor:     actor,
		Action:    action,
		Target:    target,
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	})
}

func (s *Server) entitlementSnapshotLocked(userID string) (domain.Plan, domain.Subscription, domain.Usage, error) {
	user, exists := s.state.users[userID]
	if !exists {
		return domain.Plan{}, domain.Subscription{}, domain.Usage{}, fmt.Errorf("用户不存在")
	}
	sub, exists := s.state.subscriptions[userID]
	if !exists {
		return domain.Plan{}, domain.Subscription{}, domain.Usage{}, fmt.Errorf("没有有效套餐，请先购买或兑换套餐")
	}
	planSlug := sub.PlanSlug
	if planSlug == "" {
		planSlug = user.PlanSlug
	}
	plan, exists := domain.FindPlan(s.state.plans, planSlug)
	if !exists {
		return domain.Plan{}, domain.Subscription{}, domain.Usage{}, fmt.Errorf("套餐不存在")
	}
	return plan, sub, s.state.usage[userID], nil
}

func (s *Server) userEmailVerified(userID string) bool {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	user, exists := s.state.users[userID]
	return exists && user.EmailVerified
}

func (s *Server) recordImageDelivery(userID string, bytesSent int64) bool {
	s.state.mu.Lock()
	plan, _, usage, err := s.entitlementSnapshotLocked(userID)
	if err != nil {
		s.state.mu.Unlock()
		return false
	}
	snap := snapshotStateLocked(s.state)
	usage.ImageRequests++
	usage.BandwidthBytes += bytesSent
	check := domain.CheckUsage(plan, usage)
	if !check.Allowed {
		s.state.mu.Unlock()
		return false
	}
	s.state.usage[userID] = usage
	s.state.mu.Unlock()
	if err := s.persistState(context.Background()); err != nil {
		restoreStateSnapshot(s.state, snap)
		return false
	}
	return true
}

func (s *Server) hotlinkSnapshot() HotlinkConfig {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	return HotlinkConfig{
		AllowedDomains:    append([]string(nil), s.state.hotlink.AllowedDomains...),
		BlockedDomains:    append([]string(nil), s.state.hotlink.BlockedDomains...),
		AllowEmptyReferer: s.state.hotlink.AllowEmptyReferer,
		UpdatedAt:         s.state.hotlink.UpdatedAt,
	}
}

func (s *Server) integrationSnapshot() IntegrationConfig {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	return s.state.integrations.WithFallback(s.cfg)
}

func (cfg IntegrationConfig) WithFallback(base config.Config) IntegrationConfig {
	if strings.TrimSpace(cfg.IFPayPartnerAppID) == "" {
		cfg.IFPayPartnerAppID = base.IFPayPartnerAppID
	}
	if strings.TrimSpace(cfg.IFPayRedirectURI) == "" {
		cfg.IFPayRedirectURI = base.IFPayRedirectURI
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = time.Now().UTC()
	}
	return cfg
}

func (cfg IntegrationConfig) IFPayClient() ifpay.Client {
	return ifpay.Client{
		BaseURL:       strings.TrimSpace(cfg.IFPayBaseURL),
		PartnerAppID:  strings.TrimSpace(cfg.IFPayPartnerAppID),
		ClientID:      strings.TrimSpace(cfg.IFPayClientID),
		ClientSecret:  cfg.IFPayClientSecret,
		PrivateKeyPEM: cfg.IFPayPrivateKeyPEM,
	}
}

func (cfg IntegrationConfig) Sanitized() gin.H {
	return gin.H{
		"ifpay_base_url":                        cfg.IFPayBaseURL,
		"ifpay_partner_app_id":                  cfg.IFPayPartnerAppID,
		"ifpay_client_id":                       cfg.IFPayClientID,
		"ifpay_redirect_uri":                    cfg.IFPayRedirectURI,
		"ifpay_client_secret_configured":        strings.TrimSpace(cfg.IFPayClientSecret) != "",
		"ifpay_private_key_configured":          strings.TrimSpace(cfg.IFPayPrivateKeyPEM) != "",
		"ifpay_public_key_configured":           strings.TrimSpace(cfg.IFPayPublicKeyPEM) != "",
		"ifpay_webhook_public_key_configured":   strings.TrimSpace(cfg.IFPayWebhookPublicKey) != "",
		"ifpay_oauth_start":                     "/api/v1/oauth/ifpay/start",
		"ifpay_oauth_callback":                  "/api/v1/oauth/ifpay/callback",
		"ifpay_webhook":                         "/api/v1/ifpay/webhooks/payments",
		"ifpay_configured":                      strings.TrimSpace(cfg.IFPayBaseURL) != "" && strings.TrimSpace(cfg.IFPayClientID) != "",
		"ifpay_payment_signing_configured":      strings.TrimSpace(cfg.IFPayPrivateKeyPEM) != "",
		"ifpay_webhook_verification_configured": strings.TrimSpace(cfg.IFPayWebhookPublicKey) != "",
		"updated_at":                            cfg.UpdatedAt,
	}
}

func billingDuration(cycle string) time.Duration {
	if cycle == "yearly" {
		return 365 * 24 * time.Hour
	}
	return 31 * 24 * time.Hour
}

func seedInviteCampaigns() map[string]domain.InviteCampaign {
	now := time.Now().UTC()
	return map[string]domain.InviteCampaign{
		"seed-plus": {
			ID:                   "inv_seed_plus",
			Code:                 "seed-plus",
			Name:                 "种子用户 Plus 试用",
			PlanSlug:             "plus",
			GrantDays:            30,
			TotalLimit:           1000,
			PerUserLimit:         1,
			PerEmailLimit:        1,
			PerIPLimit:           3,
			PerDeviceLimit:       1,
			NewUsersOnly:         true,
			RequireEmailVerified: true,
			StartsAt:             now.Add(-24 * time.Hour),
			EndsAt:               now.Add(365 * 24 * time.Hour),
			Status:               domain.InviteActive,
		},
		"infinite-max-internal": {
			ID:                   "inv_infinite_max_internal",
			Code:                 "infinite-max-internal",
			Name:                 "内部 Infinite Max 授权",
			PlanSlug:             "infinite-max",
			GrantDays:            3650,
			TotalLimit:           10,
			PerUserLimit:         1,
			PerEmailLimit:        1,
			PerIPLimit:           1,
			PerDeviceLimit:       1,
			NewUsersOnly:         false,
			RequireEmailVerified: true,
			RequireOAuthBinding:  true,
			RequireAdminApproval: true,
			StartsAt:             now.Add(-24 * time.Hour),
			EndsAt:               now.Add(3650 * 24 * time.Hour),
			Status:               domain.InviteActive,
			Notes:                "隐藏无限权益，仅内部、合作方、种子用户、特殊授权账号使用。",
		},
	}
}

func bind(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return true
	}
	return false
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"ok": false, "error": gin.H{"code": code, "message": message}})
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ifpayAPIBase(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/api")
}

func boundedQueryInt(raw string, fallback, min, max int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func auditLogContains(log AuditLog, query string) bool {
	metadata, _ := json.Marshal(log.Metadata)
	haystack := strings.ToLower(strings.Join([]string{
		log.ID,
		log.Actor,
		log.Action,
		log.Target,
		string(metadata),
		log.CreatedAt.Format(time.RFC3339),
	}, " "))
	return strings.Contains(haystack, query)
}

func adminImageContains(image Image, query string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		image.ID,
		image.PublicID,
		image.UserID,
		image.Filename,
		image.ObjectKey,
		image.ContentType,
		image.PerceptualHash,
		image.Status,
		image.ModerationReason,
	}, " "))
	return strings.Contains(haystack, query)
}

func normalizeAPIKeyScopes(input []string) ([]string, error) {
	allowed := map[string]bool{
		"images:read":  true,
		"images:write": true,
		"albums:read":  true,
		"albums:write": true,
	}
	if len(input) == 0 {
		input = []string{"images:read", "images:write", "albums:read", "albums:write"}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, scope := range input {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" {
			continue
		}
		if !allowed[scope] {
			return nil, fmt.Errorf("不支持的 scope: %s", scope)
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("至少需要一个有效 scope")
	}
	return out, nil
}

func urlQuery(value string) string {
	return strings.ReplaceAll(value, " ", "%20")
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		if value != nil && value != "" {
			out[key] = value
		}
	}
	for key, value := range extra {
		if value != nil && value != "" {
			out[key] = value
		}
	}
	return out
}

func currentIdentity(c *gin.Context) AuthIdentity {
	value, exists := c.Get("auth_identity")
	if !exists {
		return AuthIdentity{}
	}
	identity, _ := value.(AuthIdentity)
	return identity
}

func currentAdminIdentity(c *gin.Context) AdminIdentity {
	value, exists := c.Get("admin_identity")
	if !exists {
		return AdminIdentity{}
	}
	identity, _ := value.(AdminIdentity)
	return identity
}

func adminTokenFromRequest(c *gin.Context) string {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func (s *Server) issueAdminSessionLocked(adminID string, c *gin.Context, now time.Time) (string, AdminSession, error) {
	token, err := security.SignAdminToken(s.cfg.JWTSecret, adminID, 12*time.Hour, now)
	if err != nil {
		return "", AdminSession{}, err
	}
	session := AdminSession{
		ID:        "as_" + uuid.NewString(),
		AdminID:   adminID,
		TokenHash: security.HashSecret(token, s.cfg.JWTSecret),
		CreatedAt: now,
		ExpiresAt: now.Add(12 * time.Hour),
		UserAgent: c.GetHeader("User-Agent"),
		IP:        c.ClientIP(),
	}
	s.state.adminSessions[session.TokenHash] = session
	return token, session, nil
}

func hasRequiredScopes(actual []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, scope := range actual {
		set[strings.TrimSpace(scope)] = true
	}
	if set["*"] {
		return true
	}
	for _, scope := range required {
		if !set[scope] {
			return false
		}
	}
	return true
}

func validatePlan(plan domain.Plan) error {
	if plan.Slug == "" {
		return fmt.Errorf("套餐 slug 不能为空")
	}
	if plan.Name == "" {
		return fmt.Errorf("套餐名称不能为空")
	}
	for _, r := range plan.Slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("套餐 slug 仅允许小写字母、数字和连字符")
	}
	if plan.MonthlyPriceCent < 0 || plan.YearlyPriceCent < 0 {
		return fmt.Errorf("套餐价格不能为负数")
	}
	if plan.Visibility == "" {
		plan.Visibility = domain.PlanVisible
	}
	if plan.Visibility != domain.PlanVisible && plan.Visibility != domain.PlanHidden {
		return fmt.Errorf("套餐 visibility 无效")
	}
	if plan.Visibility == domain.PlanHidden && plan.Purchasable {
		return fmt.Errorf("隐藏套餐不能设置为公开可购买")
	}
	if !plan.Unlimited && allQuotaUnlimited(plan.Quota) {
		return fmt.Errorf("非无限套餐至少需要配置一项额度")
	}
	return nil
}

func allQuotaUnlimited(quota domain.Quota) bool {
	return quota.StorageBytes == nil &&
		quota.BandwidthBytes == nil &&
		quota.ImageRequests == nil &&
		quota.APICalls == nil &&
		quota.ImageProcessEvents == nil &&
		quota.SingleFileBytes == nil
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func normalizeDomains(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "https://")
		value = strings.TrimPrefix(value, "http://")
		value = strings.TrimSuffix(value, "/")
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func requireSession(c *gin.Context) bool {
	identity := currentIdentity(c)
	if identity.Source != "session" {
		fail(c, http.StatusForbidden, "session_required", "该操作需要用户登录态，不能使用 API Key")
		return false
	}
	return true
}

func requireScopes(c *gin.Context, scopes ...string) bool {
	identity := currentIdentity(c)
	if identity.Source != "api_key" {
		return true
	}
	if !hasRequiredScopes(identity.Scopes, scopes) {
		fail(c, http.StatusForbidden, "api_key_scope_denied", "API Key 权限不足")
		return false
	}
	return true
}

func shortCode() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:6])
}

func writeZipJSON(zw *zip.Writer, name string, value any) {
	data, _ := json.MarshalIndent(value, "", "  ")
	writeZipText(zw, name, string(data))
}

func writeZipText(zw *zip.Writer, name string, value string) {
	writeZipBytes(zw, name, []byte(value))
}

func writeZipBytes(zw *zip.Writer, name string, value []byte) {
	w, _ := zw.Create(name)
	_, _ = w.Write(value)
}

func ndjsonBytes[T any](items []T) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, item := range items {
		_ = enc.Encode(item)
	}
	return buf.Bytes()
}

func checksumFile(files map[string][]byte) string {
	var lines []string
	for _, name := range sortedFileNames(files) {
		if name == "checksums.sha256" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s  %s", backup.SHA256Hex(files[name]), name))
	}
	return strings.Join(lines, "\n") + "\n"
}

func jsonPrettyBytes(value any) []byte {
	data, _ := json.MarshalIndent(value, "", "  ")
	return data
}

func (s *Server) addBackupObject(ctx context.Context, files map[string][]byte, manifest *backup.Manifest, publicID, filename, objectKey string) {
	data, _, err := s.store.GetObject(ctx, objectKey)
	if err != nil || len(data) == 0 {
		return
	}
	baseName := filepath.Base(filename)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = filepath.Base(objectKey)
	}
	path := fmt.Sprintf("objects/%s/%s", publicID, baseName)
	files[path] = data
	manifest.Objects = append(manifest.Objects, backup.ObjectInfo{
		Path:   path,
		SHA256: backup.SHA256Hex(data),
		Bytes:  int64(len(data)),
	})
}

func sortedFileNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findBackupObject(files map[string][]byte, publicID string) (string, []byte) {
	prefix := "objects/" + publicID + "/"
	for _, name := range sortedFileNames(files) {
		if strings.HasPrefix(name, prefix) {
			return name, files[name]
		}
	}
	return "", nil
}

func extensionForMime(mimeType string) string {
	switch mimeType {
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return ".bin"
	}
}

func variantLinks(baseURL, publicID string, variants []imageproc.Variant) map[string]string {
	links := map[string]string{}
	base := strings.TrimRight(baseURL, "/")
	for _, variant := range variants {
		if variant.ObjectKey == "" {
			continue
		}
		links[variant.Kind] = fmt.Sprintf("%s/i/%s/%s%s", base, publicID, variant.Kind, extensionForMime(variant.MimeType))
	}
	return links
}

func variantBytesTotal(variants []imageproc.Variant) int64 {
	var total int64
	for _, variant := range variants {
		total += variant.Bytes
	}
	return total
}

func mergeVariants(existing, generated []imageproc.Variant) []imageproc.Variant {
	if len(generated) == 0 {
		return existing
	}
	byKind := map[string]imageproc.Variant{}
	var order []string
	for _, variant := range existing {
		if variant.Kind == "" {
			continue
		}
		if _, seen := byKind[variant.Kind]; !seen {
			order = append(order, variant.Kind)
		}
		byKind[variant.Kind] = variant
	}
	for _, variant := range generated {
		if variant.Kind == "" {
			continue
		}
		if _, seen := byKind[variant.Kind]; !seen {
			order = append(order, variant.Kind)
		}
		byKind[variant.Kind] = variant
	}
	out := make([]imageproc.Variant, 0, len(order))
	for _, kind := range order {
		out = append(out, byKind[kind])
	}
	return out
}

func readZipUpload(c *gin.Context) ([]byte, error) {
	if file, err := c.FormFile("file"); err == nil {
		src, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer src.Close()
		return io.ReadAll(src)
	}
	if c.Request.Body == nil {
		return nil, fmt.Errorf("请上传 ZIP 备份文件")
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("请上传 ZIP 备份文件")
	}
	return data, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func validateChecksumFile(checksums string, files map[string][]byte) error {
	for _, line := range strings.Split(checksums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("checksum 行格式无效: %s", line)
		}
		expected := parts[0]
		name := parts[1]
		data, exists := files[name]
		if !exists {
			return fmt.Errorf("checksum 指向不存在的文件: %s", name)
		}
		if actual := backup.SHA256Hex(data); actual != expected {
			return fmt.Errorf("文件 checksum 不匹配: %s", name)
		}
	}
	return nil
}

func protectSVG(text string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="g" x1="0" x2="1" y1="0" y2="1">
      <stop offset="0" stop-color="#07111f"/>
      <stop offset="1" stop-color="#0e7490"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#g)"/>
  <circle cx="980" cy="110" r="180" fill="#67e8f9" opacity=".16"/>
  <text x="80" y="285" fill="#ecfeff" font-size="72" font-family="sans-serif" font-weight="800">%s</text>
  <text x="84" y="365" fill="#a5f3fc" font-size="34" font-family="sans-serif">Yuexiang Image Security</text>
</svg>`, text)
}
