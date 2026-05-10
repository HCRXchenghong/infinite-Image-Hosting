CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT,
  nickname TEXT NOT NULL DEFAULT '',
  email_verified BOOLEAN NOT NULL DEFAULT FALSE,
  oauth_bound BOOLEAN NOT NULL DEFAULT FALSE,
  plan_slug TEXT NOT NULL DEFAULT 'go',
  email_verification_code TEXT NOT NULL DEFAULT '',
  password_reset_code TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'super_admin',
  status TEXT NOT NULL DEFAULT 'active',
  password_hash TEXT NOT NULL,
  totp_secret TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS admin_sessions (
  id TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  user_agent TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT ''
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_code TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_code TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS plans (
  slug TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  monthly_price_cent BIGINT NOT NULL DEFAULT 0,
  yearly_price_cent BIGINT NOT NULL DEFAULT 0,
  visibility TEXT NOT NULL DEFAULT 'visible',
  purchasable BOOLEAN NOT NULL DEFAULT TRUE,
  invite_only BOOLEAN NOT NULL DEFAULT FALSE,
  unlimited BOOLEAN NOT NULL DEFAULT FALSE,
  quota_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS subscriptions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_slug TEXT NOT NULL,
  status TEXT NOT NULL,
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  retention_ends_at TIMESTAMPTZ NOT NULL,
  source TEXT NOT NULL DEFAULT 'payment',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usage_counters (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  storage_bytes BIGINT NOT NULL DEFAULT 0,
  bandwidth_bytes BIGINT NOT NULL DEFAULT 0,
  image_requests BIGINT NOT NULL DEFAULT 0,
  api_calls BIGINT NOT NULL DEFAULT 0,
  image_process_events BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS images (
  id TEXT PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  filename TEXT NOT NULL,
  object_key TEXT NOT NULL,
  content_type TEXT NOT NULL,
  bytes BIGINT NOT NULL DEFAULT 0,
  private BOOLEAN NOT NULL DEFAULT FALSE,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  perceptual_hash TEXT NOT NULL DEFAULT '',
  variants_json JSONB NOT NULL DEFAULT '[]',
  status TEXT NOT NULL DEFAULT 'active',
  moderation_reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE images ADD COLUMN IF NOT EXISTS moderation_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_images_user_created ON images(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_images_phash ON images(perceptual_hash);

CREATE TABLE IF NOT EXISTS albums (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  private BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  prefix TEXT NOT NULL,
  secret_hash TEXT NOT NULL,
  scopes_json JSONB NOT NULL DEFAULT '[]',
  revoked BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS orders (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_slug TEXT NOT NULL,
  billing_cycle TEXT NOT NULL,
  amount_cent BIGINT NOT NULL,
  status TEXT NOT NULL,
  ifpay_payment_id TEXT,
  ifpay_sub_method TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  refunded_at TIMESTAMPTZ,
  operator_note TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_ifpay_payment ON orders(ifpay_payment_id) WHERE ifpay_payment_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS invite_campaigns (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  plan_slug TEXT NOT NULL,
  grant_days INTEGER NOT NULL,
  total_limit INTEGER NOT NULL DEFAULT 0,
  per_user_limit INTEGER NOT NULL DEFAULT 1,
  per_email_limit INTEGER NOT NULL DEFAULT 1,
  per_ip_limit INTEGER NOT NULL DEFAULT 0,
  per_device_limit INTEGER NOT NULL DEFAULT 0,
  new_users_only BOOLEAN NOT NULL DEFAULT TRUE,
  require_email_verified BOOLEAN NOT NULL DEFAULT TRUE,
  require_oauth_binding BOOLEAN NOT NULL DEFAULT FALSE,
  require_admin_approval BOOLEAN NOT NULL DEFAULT FALSE,
  allow_stacking BOOLEAN NOT NULL DEFAULT FALSE,
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invite_redemptions (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL REFERENCES invite_campaigns(code),
  user_id TEXT NOT NULL,
  email TEXT NOT NULL,
  ip TEXT NOT NULL,
  device_id TEXT NOT NULL DEFAULT '',
  plan_slug TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ifpay_webhook_events (
  event_id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  resource_type TEXT NOT NULL DEFAULT '',
  resource_id TEXT NOT NULL DEFAULT '',
  payload_json JSONB NOT NULL DEFAULT '{}',
  processed BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS risk_events (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  message TEXT NOT NULL,
  ip TEXT NOT NULL DEFAULT '',
  referer TEXT NOT NULL DEFAULT '',
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id TEXT PRIMARY KEY,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS system_settings (
  key TEXT PRIMARY KEY,
  value_json JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS backup_jobs (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  object_key TEXT,
  manifest_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

INSERT INTO plans (slug, name, monthly_price_cent, yearly_price_cent, visibility, purchasable, invite_only, unlimited, quota_json)
VALUES
('go', 'Go', 1200, 12000, 'visible', TRUE, FALSE, FALSE, '{"storage_bytes":10737418240,"bandwidth_bytes":107374182400,"image_requests":1000000,"api_calls":50000,"image_process_events":3000,"single_file_bytes":20971520}'),
('plus', 'Plus', 2900, 29900, 'visible', TRUE, FALSE, FALSE, '{"storage_bytes":53687091200,"bandwidth_bytes":536870912000,"image_requests":10000000,"api_calls":500000,"image_process_events":20000,"single_file_bytes":52428800}'),
('pro', 'Pro', 7900, 79900, 'visible', TRUE, FALSE, FALSE, '{"storage_bytes":214748364800,"bandwidth_bytes":2199023255552,"image_requests":50000000,"api_calls":5000000,"image_process_events":100000,"single_file_bytes":104857600}'),
('ultra', 'Ultra', 12000, 120000, 'visible', TRUE, FALSE, FALSE, '{"storage_bytes":536870912000,"bandwidth_bytes":5497558138880,"image_requests":120000000,"api_calls":15000000,"image_process_events":300000,"single_file_bytes":104857600}'),
('infinite-max', 'Infinite Max', 0, 0, 'hidden', FALSE, TRUE, TRUE, '{"storage_bytes":null,"bandwidth_bytes":null,"image_requests":null,"api_calls":null,"image_process_events":null,"single_file_bytes":524288000}')
ON CONFLICT (slug) DO NOTHING;
