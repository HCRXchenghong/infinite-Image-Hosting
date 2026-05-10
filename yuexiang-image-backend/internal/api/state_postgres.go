package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yuexiang/image-backend/internal/domain"
)

type statePersister interface {
	Ping(context.Context) error
	Load(context.Context, *memoryState) error
	Save(context.Context, *memoryState) error
	Close() error
}

type postgresStateStore struct {
	pool *pgxpool.Pool
}

type stateSnapshot struct {
	plans          []domain.Plan
	users          []User
	admins         []AdminUser
	adminSessions  []AdminSession
	adminBootstrap []PendingAdminBootstrap
	images         []Image
	albums         []Album
	apiKeys        []APIKey
	orders         []Order
	subscriptions  []domain.Subscription
	usage          map[string]domain.Usage
	invites        []domain.InviteCampaign
	redemptions    []InviteRedemption
	webhooks       []string
	riskEvents     []RiskEvent
	auditLogs      []AuditLog
	hotlink        HotlinkConfig
	integrations   IntegrationConfig
}

func newPostgresStateStore(ctx context.Context, databaseURL string) (*postgresStateStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	store := &postgresStateStore{pool: pool}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.applyMigrations(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (p *postgresStateStore) Close() error {
	p.pool.Close()
	return nil
}

func (p *postgresStateStore) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *postgresStateStore) applyMigrations(ctx context.Context) error {
	for _, name := range []string{"001_initial.sql", "002_order_operations.sql", "003_order_failed_at.sql", "004_admin_auth.sql", "005_user_avatar.sql"} {
		sql, err := loadMigrationSQL(name)
		if err != nil {
			return err
		}
		if _, err := p.pool.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

func (p *postgresStateStore) Load(ctx context.Context, state *memoryState) error {
	plans, err := p.loadPlans(ctx)
	if err != nil {
		return err
	}
	users, err := p.loadUsers(ctx)
	if err != nil {
		return err
	}
	admins, err := p.loadAdmins(ctx)
	if err != nil {
		return err
	}
	adminSessions, err := p.loadAdminSessions(ctx)
	if err != nil {
		return err
	}
	subscriptions, err := p.loadSubscriptions(ctx)
	if err != nil {
		return err
	}
	usage, err := p.loadUsage(ctx)
	if err != nil {
		return err
	}
	images, err := p.loadImages(ctx)
	if err != nil {
		return err
	}
	albums, err := p.loadAlbums(ctx)
	if err != nil {
		return err
	}
	apiKeys, err := p.loadAPIKeys(ctx)
	if err != nil {
		return err
	}
	orders, err := p.loadOrders(ctx)
	if err != nil {
		return err
	}
	invites, err := p.loadInvites(ctx)
	if err != nil {
		return err
	}
	redemptions, err := p.loadRedemptions(ctx)
	if err != nil {
		return err
	}
	webhooks, err := p.loadWebhooks(ctx)
	if err != nil {
		return err
	}
	riskEvents, err := p.loadRiskEvents(ctx)
	if err != nil {
		return err
	}
	auditLogs, err := p.loadAuditLogs(ctx)
	if err != nil {
		return err
	}
	hotlink, hotlinkLoaded, err := p.loadHotlinkConfig(ctx)
	if err != nil {
		return err
	}
	integrations, integrationsLoaded, err := p.loadIntegrationConfig(ctx)
	if err != nil {
		return err
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(plans) > 0 {
		state.plans = plans
	}
	state.users = users
	state.admins = admins
	state.adminSessions = adminSessions
	state.subscriptions = subscriptions
	state.usage = usage
	state.images = images
	state.albums = albums
	state.apiKeys = apiKeys
	state.orders = orders
	if len(invites) > 0 {
		state.invites = invites
	}
	for code, invite := range seedInviteCampaigns() {
		if _, exists := state.invites[code]; !exists {
			state.invites[code] = invite
		}
	}
	state.redemptions = redemptions
	state.webhooks = webhooks
	state.riskEvents = riskEvents
	state.auditLogs = auditLogs
	if hotlinkLoaded {
		state.hotlink = hotlink
	}
	if integrationsLoaded {
		state.integrations = integrations
	}
	return nil
}

func (p *postgresStateStore) Save(ctx context.Context, state *memoryState) error {
	snap := snapshotState(state)
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `TRUNCATE TABLE
		usage_counters,
		api_keys,
		albums,
		images,
		subscriptions,
		orders,
		admin_sessions,
		admin_users,
		users,
		invite_redemptions,
		invite_campaigns,
		ifpay_webhook_events,
		risk_events,
		audit_logs,
		system_settings,
		plans
		RESTART IDENTITY CASCADE`); err != nil {
		return err
	}

	for _, plan := range snap.plans {
		if _, err := tx.Exec(ctx, `INSERT INTO plans
			(slug, name, monthly_price_cent, yearly_price_cent, visibility, purchasable, invite_only, unlimited, quota_json)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			plan.Slug, plan.Name, plan.MonthlyPriceCent, plan.YearlyPriceCent, string(plan.Visibility),
			plan.Purchasable, plan.InviteOnly, plan.Unlimited, jsonBytes(plan.Quota)); err != nil {
			return err
		}
	}
	for _, user := range snap.users {
		if _, err := tx.Exec(ctx, `INSERT INTO users
			(id, email, password_hash, nickname, avatar_url, email_verified, oauth_bound, plan_slug, status, email_verification_code, password_reset_code, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			user.ID, user.Email, user.PasswordHash, user.Nickname, user.AvatarURL, user.EmailVerified, user.OAuthBound,
			user.PlanSlug, user.Status, user.EmailVerificationCode, user.PasswordResetCode, user.CreatedAt); err != nil {
			return err
		}
	}
	for _, admin := range snap.admins {
		if _, err := tx.Exec(ctx, `INSERT INTO admin_users
			(id, email, name, role, status, password_hash, totp_secret, created_at, last_login_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			admin.ID, admin.Email, admin.Name, admin.Role, admin.Status, admin.PasswordHash, admin.TOTPSecret, admin.CreatedAt, admin.LastLoginAt); err != nil {
			return err
		}
	}
	for _, session := range snap.adminSessions {
		if _, err := tx.Exec(ctx, `INSERT INTO admin_sessions
			(id, admin_id, token_hash, created_at, expires_at, user_agent, ip)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			session.ID, session.AdminID, session.TokenHash, session.CreatedAt, session.ExpiresAt, session.UserAgent, session.IP); err != nil {
			return err
		}
	}
	for _, sub := range snap.subscriptions {
		if _, err := tx.Exec(ctx, `INSERT INTO subscriptions
			(id, user_id, plan_slug, status, starts_at, ends_at, retention_ends_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			sub.ID, sub.UserID, sub.PlanSlug, string(sub.Status), sub.StartsAt, sub.EndsAt, sub.RetentionEndsAt); err != nil {
			return err
		}
	}
	for userID, usage := range snap.usage {
		if _, err := tx.Exec(ctx, `INSERT INTO usage_counters
			(user_id, storage_bytes, bandwidth_bytes, image_requests, api_calls, image_process_events)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			userID, usage.StorageBytes, usage.BandwidthBytes, usage.ImageRequests, usage.APICalls, usage.ImageProcessEvents); err != nil {
			return err
		}
	}
	for _, image := range snap.images {
		if _, err := tx.Exec(ctx, `INSERT INTO images
			(id, public_id, user_id, filename, object_key, content_type, bytes, private, width, height, perceptual_hash, variants_json, status, moderation_reason, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			image.ID, image.PublicID, image.UserID, image.Filename, image.ObjectKey, image.ContentType, image.Bytes,
			image.Private, image.Width, image.Height, image.PerceptualHash, jsonBytes(image.Variants), image.Status, image.ModerationReason, image.CreatedAt); err != nil {
			return err
		}
	}
	for _, album := range snap.albums {
		if _, err := tx.Exec(ctx, `INSERT INTO albums (id, user_id, name, private, created_at) VALUES ($1,$2,$3,$4,$5)`,
			album.ID, album.UserID, album.Name, album.Private, album.CreatedAt); err != nil {
			return err
		}
	}
	for _, key := range snap.apiKeys {
		if _, err := tx.Exec(ctx, `INSERT INTO api_keys
			(id, user_id, name, prefix, secret_hash, scopes_json, revoked, created_at, last_used_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			key.ID, key.UserID, key.Name, key.Prefix, key.SecretHash, jsonBytes(key.Scopes), key.Revoked, key.CreatedAt, key.LastUsedAt); err != nil {
			return err
		}
	}
	for _, order := range snap.orders {
		if _, err := tx.Exec(ctx, `INSERT INTO orders
			(id, user_id, plan_slug, billing_cycle, amount_cent, status, ifpay_payment_id, ifpay_sub_method, created_at, paid_at, failed_at, cancelled_at, refunded_at, operator_note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			order.ID, order.UserID, order.PlanSlug, order.BillingCycle, order.AmountCent, order.Status,
			order.IFPayPaymentID, order.IFPaySubMethod, order.CreatedAt, order.PaidAt, order.FailedAt,
			order.CancelledAt, order.RefundedAt, order.OperatorNote); err != nil {
			return err
		}
	}
	for _, invite := range snap.invites {
		if _, err := tx.Exec(ctx, `INSERT INTO invite_campaigns
			(id, code, name, plan_slug, grant_days, total_limit, per_user_limit, per_email_limit, per_ip_limit, per_device_limit,
			 new_users_only, require_email_verified, require_oauth_binding, require_admin_approval, allow_stacking, starts_at, ends_at, status, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			invite.ID, invite.Code, invite.Name, invite.PlanSlug, invite.GrantDays, invite.TotalLimit, invite.PerUserLimit,
			invite.PerEmailLimit, invite.PerIPLimit, invite.PerDeviceLimit, invite.NewUsersOnly, invite.RequireEmailVerified,
			invite.RequireOAuthBinding, invite.RequireAdminApproval, invite.AllowStacking, invite.StartsAt, invite.EndsAt,
			string(invite.Status), invite.Notes); err != nil {
			return err
		}
	}
	for _, redemption := range snap.redemptions {
		if _, err := tx.Exec(ctx, `INSERT INTO invite_redemptions
			(id, code, user_id, email, ip, device_id, plan_slug, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			redemption.ID, redemption.Code, redemption.UserID, redemption.Email, redemption.IP, redemption.DeviceID,
			redemption.PlanSlug, redemption.CreatedAt); err != nil {
			return err
		}
	}
	for _, eventID := range snap.webhooks {
		if _, err := tx.Exec(ctx, `INSERT INTO ifpay_webhook_events (event_id, event_type, processed) VALUES ($1,$2,$3)`,
			eventID, "processed", true); err != nil {
			return err
		}
	}
	for _, event := range snap.riskEvents {
		if _, err := tx.Exec(ctx, `INSERT INTO risk_events (id, type, message, ip, referer, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			event.ID, event.Type, event.Message, event.IP, event.Referer, event.CreatedAt); err != nil {
			return err
		}
	}
	for _, log := range snap.auditLogs {
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (id, actor, action, target, metadata_json, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			log.ID, log.Actor, log.Action, log.Target, jsonBytes(log.Metadata), log.CreatedAt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO system_settings (key, value_json, updated_at) VALUES ($1,$2,now())`,
		"hotlink", jsonBytes(snap.hotlink)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO system_settings (key, value_json, updated_at) VALUES ($1,$2,now())`,
		"integrations", jsonBytes(snap.integrations)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *postgresStateStore) loadPlans(ctx context.Context) ([]domain.Plan, error) {
	rows, err := p.pool.Query(ctx, `SELECT slug, name, monthly_price_cent, yearly_price_cent, visibility, purchasable, invite_only, unlimited, quota_json FROM plans ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []domain.Plan
	for rows.Next() {
		var plan domain.Plan
		var visibility string
		var quota []byte
		if err := rows.Scan(&plan.Slug, &plan.Name, &plan.MonthlyPriceCent, &plan.YearlyPriceCent, &visibility, &plan.Purchasable, &plan.InviteOnly, &plan.Unlimited, &quota); err != nil {
			return nil, err
		}
		plan.Visibility = domain.PlanVisibility(visibility)
		_ = json.Unmarshal(quota, &plan.Quota)
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (p *postgresStateStore) loadUsers(ctx context.Context) (map[string]User, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, email, nickname, avatar_url, email_verified, oauth_bound, plan_slug, password_hash, email_verification_code, password_reset_code, status, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := map[string]User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Nickname, &user.AvatarURL, &user.EmailVerified, &user.OAuthBound, &user.PlanSlug, &user.PasswordHash, &user.EmailVerificationCode, &user.PasswordResetCode, &user.Status, &user.CreatedAt); err != nil {
			return nil, err
		}
		users[user.ID] = user
	}
	return users, rows.Err()
}

func (p *postgresStateStore) loadAdmins(ctx context.Context) (map[string]AdminUser, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, email, name, role, status, password_hash, totp_secret, created_at, last_login_at FROM admin_users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	admins := map[string]AdminUser{}
	for rows.Next() {
		var admin AdminUser
		if err := rows.Scan(&admin.ID, &admin.Email, &admin.Name, &admin.Role, &admin.Status, &admin.PasswordHash, &admin.TOTPSecret, &admin.CreatedAt, &admin.LastLoginAt); err != nil {
			return nil, err
		}
		admins[admin.ID] = admin
	}
	return admins, rows.Err()
}

func (p *postgresStateStore) loadAdminSessions(ctx context.Context) (map[string]AdminSession, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, admin_id, token_hash, created_at, expires_at, COALESCE(user_agent,''), COALESCE(ip,'') FROM admin_sessions WHERE expires_at > now()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := map[string]AdminSession{}
	for rows.Next() {
		var session AdminSession
		if err := rows.Scan(&session.ID, &session.AdminID, &session.TokenHash, &session.CreatedAt, &session.ExpiresAt, &session.UserAgent, &session.IP); err != nil {
			return nil, err
		}
		sessions[session.TokenHash] = session
	}
	return sessions, rows.Err()
}

func (p *postgresStateStore) loadSubscriptions(ctx context.Context) (map[string]domain.Subscription, error) {
	rows, err := p.pool.Query(ctx, `SELECT DISTINCT ON (user_id) id, user_id, plan_slug, status, starts_at, ends_at, retention_ends_at FROM subscriptions ORDER BY user_id, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	subs := map[string]domain.Subscription{}
	for rows.Next() {
		var sub domain.Subscription
		var status string
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.PlanSlug, &status, &sub.StartsAt, &sub.EndsAt, &sub.RetentionEndsAt); err != nil {
			return nil, err
		}
		sub.Status = domain.SubscriptionStatus(status)
		subs[sub.UserID] = sub
	}
	return subs, rows.Err()
}

func (p *postgresStateStore) loadUsage(ctx context.Context) (map[string]domain.Usage, error) {
	rows, err := p.pool.Query(ctx, `SELECT user_id, storage_bytes, bandwidth_bytes, image_requests, api_calls, image_process_events FROM usage_counters`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usage := map[string]domain.Usage{}
	for rows.Next() {
		var userID string
		var value domain.Usage
		if err := rows.Scan(&userID, &value.StorageBytes, &value.BandwidthBytes, &value.ImageRequests, &value.APICalls, &value.ImageProcessEvents); err != nil {
			return nil, err
		}
		usage[userID] = value
	}
	return usage, rows.Err()
}

func (p *postgresStateStore) loadImages(ctx context.Context) (map[string]Image, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, public_id, user_id, filename, object_key, content_type, bytes, private, width, height, perceptual_hash, variants_json, status, moderation_reason, created_at FROM images`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	images := map[string]Image{}
	for rows.Next() {
		var image Image
		var variants []byte
		if err := rows.Scan(&image.ID, &image.PublicID, &image.UserID, &image.Filename, &image.ObjectKey, &image.ContentType, &image.Bytes, &image.Private, &image.Width, &image.Height, &image.PerceptualHash, &variants, &image.Status, &image.ModerationReason, &image.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(variants, &image.Variants)
		images[image.PublicID] = image
	}
	return images, rows.Err()
}

func (p *postgresStateStore) loadAlbums(ctx context.Context) (map[string]Album, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, user_id, name, private, created_at FROM albums`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	albums := map[string]Album{}
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.ID, &album.UserID, &album.Name, &album.Private, &album.CreatedAt); err != nil {
			return nil, err
		}
		albums[album.ID] = album
	}
	return albums, rows.Err()
}

func (p *postgresStateStore) loadAPIKeys(ctx context.Context) (map[string][]APIKey, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, user_id, name, prefix, secret_hash, scopes_json, revoked, created_at, last_used_at FROM api_keys`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := map[string][]APIKey{}
	for rows.Next() {
		var key APIKey
		var scopes []byte
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.Prefix, &key.SecretHash, &scopes, &key.Revoked, &key.CreatedAt, &key.LastUsedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(scopes, &key.Scopes)
		keys[key.UserID] = append(keys[key.UserID], key)
	}
	return keys, rows.Err()
}

func (p *postgresStateStore) loadOrders(ctx context.Context) (map[string]Order, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, user_id, plan_slug, billing_cycle, amount_cent, status, COALESCE(ifpay_payment_id,''), COALESCE(ifpay_sub_method,''), created_at, paid_at, failed_at, cancelled_at, refunded_at, COALESCE(operator_note,'') FROM orders`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := map[string]Order{}
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.PlanSlug, &order.BillingCycle, &order.AmountCent, &order.Status, &order.IFPayPaymentID, &order.IFPaySubMethod, &order.CreatedAt, &order.PaidAt, &order.FailedAt, &order.CancelledAt, &order.RefundedAt, &order.OperatorNote); err != nil {
			return nil, err
		}
		orders[order.ID] = order
	}
	return orders, rows.Err()
}

func (p *postgresStateStore) loadInvites(ctx context.Context) (map[string]domain.InviteCampaign, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, code, name, plan_slug, grant_days, total_limit, per_user_limit, per_email_limit, per_ip_limit, per_device_limit, new_users_only, require_email_verified, require_oauth_binding, require_admin_approval, allow_stacking, starts_at, ends_at, status, notes FROM invite_campaigns`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invites := map[string]domain.InviteCampaign{}
	for rows.Next() {
		var invite domain.InviteCampaign
		var status string
		if err := rows.Scan(&invite.ID, &invite.Code, &invite.Name, &invite.PlanSlug, &invite.GrantDays, &invite.TotalLimit, &invite.PerUserLimit, &invite.PerEmailLimit, &invite.PerIPLimit, &invite.PerDeviceLimit, &invite.NewUsersOnly, &invite.RequireEmailVerified, &invite.RequireOAuthBinding, &invite.RequireAdminApproval, &invite.AllowStacking, &invite.StartsAt, &invite.EndsAt, &status, &invite.Notes); err != nil {
			return nil, err
		}
		invite.Status = domain.InviteCampaignStatus(status)
		invites[invite.Code] = invite
	}
	return invites, rows.Err()
}

func (p *postgresStateStore) loadRedemptions(ctx context.Context) ([]InviteRedemption, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, code, user_id, email, ip, device_id, plan_slug, created_at FROM invite_redemptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var redemptions []InviteRedemption
	for rows.Next() {
		var redemption InviteRedemption
		if err := rows.Scan(&redemption.ID, &redemption.Code, &redemption.UserID, &redemption.Email, &redemption.IP, &redemption.DeviceID, &redemption.PlanSlug, &redemption.CreatedAt); err != nil {
			return nil, err
		}
		redemptions = append(redemptions, redemption)
	}
	return redemptions, rows.Err()
}

func (p *postgresStateStore) loadWebhooks(ctx context.Context) (map[string]bool, error) {
	rows, err := p.pool.Query(ctx, `SELECT event_id FROM ifpay_webhook_events`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	webhooks := map[string]bool{}
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, err
		}
		webhooks[eventID] = true
	}
	return webhooks, rows.Err()
}

func (p *postgresStateStore) loadRiskEvents(ctx context.Context) ([]RiskEvent, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, type, message, ip, referer, created_at FROM risk_events ORDER BY created_at DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []RiskEvent
	for rows.Next() {
		var event RiskEvent
		if err := rows.Scan(&event.ID, &event.Type, &event.Message, &event.IP, &event.Referer, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (p *postgresStateStore) loadAuditLogs(ctx context.Context) ([]AuditLog, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, actor, action, target, metadata_json, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var metadata []byte
		if err := rows.Scan(&log.ID, &log.Actor, &log.Action, &log.Target, &metadata, &log.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &log.Metadata)
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (p *postgresStateStore) loadHotlinkConfig(ctx context.Context) (HotlinkConfig, bool, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `SELECT value_json FROM system_settings WHERE key = $1`, "hotlink").Scan(&raw)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return HotlinkConfig{}, false, nil
		}
		return HotlinkConfig{}, false, err
	}
	var cfg HotlinkConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return HotlinkConfig{}, false, err
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = time.Now().UTC()
	}
	return cfg, true, nil
}

func (p *postgresStateStore) loadIntegrationConfig(ctx context.Context) (IntegrationConfig, bool, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `SELECT value_json FROM system_settings WHERE key = $1`, "integrations").Scan(&raw)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return IntegrationConfig{}, false, nil
		}
		return IntegrationConfig{}, false, err
	}
	var cfg IntegrationConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return IntegrationConfig{}, false, err
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = time.Now().UTC()
	}
	return cfg, true, nil
}

func snapshotState(state *memoryState) stateSnapshot {
	state.mu.RLock()
	defer state.mu.RUnlock()
	snap := snapshotStateLocked(state)
	now := time.Now().UTC()
	activeSessions := snap.adminSessions[:0]
	for _, session := range snap.adminSessions {
		if session.ExpiresAt.After(now) {
			activeSessions = append(activeSessions, session)
		}
	}
	snap.adminSessions = activeSessions
	return snap
}

func snapshotStateLocked(state *memoryState) stateSnapshot {
	snap := stateSnapshot{
		plans:       append([]domain.Plan(nil), state.plans...),
		usage:       map[string]domain.Usage{},
		redemptions: append([]InviteRedemption(nil), state.redemptions...),
		hotlink: HotlinkConfig{
			AllowedDomains:    append([]string(nil), state.hotlink.AllowedDomains...),
			BlockedDomains:    append([]string(nil), state.hotlink.BlockedDomains...),
			AllowEmptyReferer: state.hotlink.AllowEmptyReferer,
			UpdatedAt:         state.hotlink.UpdatedAt,
		},
		integrations: state.integrations,
	}
	for _, user := range state.users {
		snap.users = append(snap.users, user)
	}
	for _, admin := range state.admins {
		snap.admins = append(snap.admins, admin)
	}
	for _, session := range state.adminSessions {
		snap.adminSessions = append(snap.adminSessions, session)
	}
	for _, pending := range state.adminBootstrap {
		snap.adminBootstrap = append(snap.adminBootstrap, pending)
	}
	for _, image := range state.images {
		snap.images = append(snap.images, image)
	}
	for _, album := range state.albums {
		snap.albums = append(snap.albums, album)
	}
	for _, keys := range state.apiKeys {
		snap.apiKeys = append(snap.apiKeys, keys...)
	}
	for _, order := range state.orders {
		snap.orders = append(snap.orders, order)
	}
	for _, sub := range state.subscriptions {
		snap.subscriptions = append(snap.subscriptions, sub)
	}
	for userID, usage := range state.usage {
		snap.usage[userID] = usage
	}
	for _, invite := range state.invites {
		snap.invites = append(snap.invites, invite)
	}
	for eventID, processed := range state.webhooks {
		if processed {
			snap.webhooks = append(snap.webhooks, eventID)
		}
	}
	snap.riskEvents = append([]RiskEvent(nil), state.riskEvents...)
	snap.auditLogs = append([]AuditLog(nil), state.auditLogs...)
	sort.Slice(snap.users, func(i, j int) bool { return snap.users[i].ID < snap.users[j].ID })
	sort.Slice(snap.admins, func(i, j int) bool { return snap.admins[i].ID < snap.admins[j].ID })
	sort.Slice(snap.adminSessions, func(i, j int) bool { return snap.adminSessions[i].ID < snap.adminSessions[j].ID })
	sort.Slice(snap.adminBootstrap, func(i, j int) bool { return snap.adminBootstrap[i].ID < snap.adminBootstrap[j].ID })
	sort.Slice(snap.images, func(i, j int) bool { return snap.images[i].PublicID < snap.images[j].PublicID })
	sort.Slice(snap.invites, func(i, j int) bool { return snap.invites[i].Code < snap.invites[j].Code })
	sort.Strings(snap.webhooks)
	return snap
}

func restoreStateSnapshot(state *memoryState, snap stateSnapshot) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.plans = append([]domain.Plan(nil), snap.plans...)
	state.users = make(map[string]User, len(snap.users))
	for _, user := range snap.users {
		state.users[user.ID] = user
	}
	state.admins = make(map[string]AdminUser, len(snap.admins))
	for _, admin := range snap.admins {
		state.admins[admin.ID] = admin
	}
	state.adminSessions = make(map[string]AdminSession, len(snap.adminSessions))
	for _, session := range snap.adminSessions {
		state.adminSessions[session.TokenHash] = session
	}
	state.adminBootstrap = make(map[string]PendingAdminBootstrap, len(snap.adminBootstrap))
	for _, pending := range snap.adminBootstrap {
		state.adminBootstrap[pending.ID] = pending
	}
	state.images = make(map[string]Image, len(snap.images))
	for _, image := range snap.images {
		state.images[image.PublicID] = image
	}
	state.albums = make(map[string]Album, len(snap.albums))
	for _, album := range snap.albums {
		state.albums[album.ID] = album
	}
	state.apiKeys = make(map[string][]APIKey)
	for _, key := range snap.apiKeys {
		state.apiKeys[key.UserID] = append(state.apiKeys[key.UserID], key)
	}
	state.orders = make(map[string]Order, len(snap.orders))
	for _, order := range snap.orders {
		state.orders[order.ID] = order
	}
	state.subscriptions = make(map[string]domain.Subscription, len(snap.subscriptions))
	for _, sub := range snap.subscriptions {
		state.subscriptions[sub.UserID] = sub
	}
	state.usage = make(map[string]domain.Usage, len(snap.usage))
	for userID, usage := range snap.usage {
		state.usage[userID] = usage
	}
	state.invites = make(map[string]domain.InviteCampaign, len(snap.invites))
	for _, invite := range snap.invites {
		state.invites[invite.Code] = invite
	}
	state.redemptions = append([]InviteRedemption(nil), snap.redemptions...)
	state.webhooks = make(map[string]bool, len(snap.webhooks))
	for _, eventID := range snap.webhooks {
		state.webhooks[eventID] = true
	}
	state.riskEvents = append([]RiskEvent(nil), snap.riskEvents...)
	state.auditLogs = append([]AuditLog(nil), snap.auditLogs...)
	state.hotlink = HotlinkConfig{
		AllowedDomains:    append([]string(nil), snap.hotlink.AllowedDomains...),
		BlockedDomains:    append([]string(nil), snap.hotlink.BlockedDomains...),
		AllowEmptyReferer: snap.hotlink.AllowEmptyReferer,
		UpdatedAt:         snap.hotlink.UpdatedAt,
	}
	state.integrations = snap.integrations
}

func jsonBytes(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func loadMigrationSQL(name string) (string, error) {
	candidates := []string{}
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, name))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "migrations", name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "migrations", name))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "migrations", name))
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("migration %s not found in %v", name, candidates)
}
