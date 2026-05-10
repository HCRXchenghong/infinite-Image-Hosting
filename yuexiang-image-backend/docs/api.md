# 悦享图床 API 概览

## 用户侧

- `GET /api/v1/plans`：公开套餐，不返回 `infinite-max`。
- `POST /api/v1/auth/register`：邮箱注册。
- `POST /api/v1/auth/login`：邮箱登录。
- `POST /api/v1/auth/forgot-password`：忘记密码。
- `POST /api/v1/auth/reset-password`：使用验证码重置密码。
- `POST /api/v1/auth/verify-email`：邮箱验证码验证。
- `POST /api/v1/auth/resend-verification`：重新发送邮箱验证码。
- `GET /api/v1/auth/me`：当前用户、订阅、用量与鉴权来源。
- `GET /api/v1/oauth/ifpay/start`：发起 IF-Pay/OIDC 登录，用户端会跳转到授权地址。
- `GET /api/v1/oauth/ifpay/callback`：IF-Pay/OIDC 登录回调，返回用户 session token 和 `ifpay_access_token`，前端用于创建支付订单。
- `POST /api/v1/ifpay/webhooks/payments`：IF-Pay 支付 webhook，支持摘要/RSA 验签、事件去重和订单生命周期处理；当前处理 `payment.succeeded`、`payment.failed`/`payment.expired`、`payment.cancelled`/`payment.canceled`、`payment.refunded`/`payment.refund.succeeded`，未匹配订单会写入审计日志并返回 `matched=false`。
- `POST /api/v1/images`：上传图片，multipart 字段名为 `file`。
- `GET /api/v1/images`：图片列表。
- `GET /api/v1/images/{public_id}/sign`：生成 15 分钟短期签名访问链接。
- `GET /i/{public_id}`：图片原图分发，执行 Referer/签名/私有读策略。
- `GET /i/{public_id}/{variant}`：图片派生图分发，例如 `thumbnail.webp`、`webp.webp`、`avif.avif`。
- `PATCH /api/v1/images/{public_id}/privacy`：切换公开/私有读取。
- `DELETE /api/v1/images/{public_id}`：软删除图片并删除对象存储文件。
- `GET /api/v1/albums` / `POST /api/v1/albums`：相册读取与创建。
- `GET /api/v1/api-keys` / `POST /api/v1/api-keys` / `DELETE /api/v1/api-keys/{id}`：API Key 管理，密钥只展示一次，服务端仅存 HMAC；创建前必须完成邮箱验证，scope 使用白名单。
- `POST /api/v1/checkout/ifpay`：创建 IF-Pay 订单；用户必须完成邮箱验证，生产环境必须传入 IF-Pay access token。
- `GET /api/v1/orders`：当前用户订单列表，按创建时间倒序返回，用于资源包管理页展示支付状态；订单包含 `paid_at`、`failed_at`、`cancelled_at`、`refunded_at` 等时间线字段。
- `GET /api/v1/backups/export`：导出 ZIP 备份。
- `POST /api/v1/backups/import`：校验 ZIP 备份、manifest 和 checksum，并将对象恢复到当前账号。
- `POST /api/v1/invites/{code}/redeem`：兑换邀请权益。
- `PATCH /api/v1/settings/profile`：更新用户昵称与 Avatar。`avatar_url` 支持 HTTPS 地址、站内路径或 512KB 内图片 data URL。
- `POST /api/v1/settings/account-destroy-request`：提交账号销毁人工复核工单。

## 管理侧

管理端首次使用需要调用 bootstrap 接口创建首个管理员并绑定 TOTP 2FA；后续管理接口需要 `Authorization: Bearer yx_admin_...` 管理员会话。

- `GET /api/v1/admin/auth/status`：返回是否需要首次初始化，以及当前管理员会话。
- `POST /api/v1/admin/auth/bootstrap/start`：提交管理员邮箱、显示名和密码，返回 TOTP 二维码 data URL 与手动密钥。
- `POST /api/v1/admin/auth/bootstrap/complete`：提交初始化令牌和 2FA 动态验证码，创建首个管理员并返回管理员会话。
- `POST /api/v1/admin/auth/login`：管理员邮箱、密码、2FA 动态验证码登录。
- `POST /api/v1/admin/auth/logout`：销毁当前管理员会话。
- `GET /api/v1/admin/overview`
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/plans`
- `POST /api/v1/admin/plans`
- `GET /api/v1/admin/invites`
- `POST /api/v1/admin/invites`
- `GET /api/v1/admin/orders`
- `POST /api/v1/admin/orders/{id}/mark-paid`：人工入账/支付对账，必须提供 `reason`，会发放对应订阅并写审计。
- `POST /api/v1/admin/orders/{id}/cancel`：取消未支付订单，必须提供 `reason`。
- `POST /api/v1/admin/orders/{id}/refund`：退款已支付订单，必须提供 `reason`，当前订阅匹配时切入 30 天只读保留期。
- `GET /api/v1/admin/security/events`
- `GET /api/v1/admin/security/hotlink`
- `PATCH /api/v1/admin/security/hotlink`
- `GET /api/v1/admin/storage/config`
- `GET /api/v1/admin/system/config`
- `GET /api/v1/admin/queue/status`
- `GET /api/v1/admin/queue/dead-letters`
- `POST /api/v1/admin/queue/dead-letters/{id}/requeue`
- `GET /api/v1/admin/cdn/config`
- `GET /api/v1/admin/api/config`
- `GET /api/v1/admin/integrations/ifpay`：读取 IF-Pay/OAuth 运行时配置，只返回密钥是否已配置，不回显密钥明文。
- `PATCH /api/v1/admin/integrations/ifpay`：更新 IF-Pay/OAuth 配置。支持 `ifpay_base_url`、`ifpay_partner_app_id`、`ifpay_client_id`、`ifpay_redirect_uri`、`ifpay_client_secret`、`ifpay_private_key_pem`、`ifpay_public_key_pem`、`ifpay_webhook_public_key_pem`；密钥字段留空表示保持不变。
- `GET /api/v1/admin/backups/export`
- `POST /api/v1/admin/backups/import/validate`
- `GET /api/v1/admin/audit-logs?q=&action=&actor=&limit=100&offset=0`：分页筛选审计日志，返回 `{ items, total, limit, offset }`。
- `GET /api/v1/admin/images?q=&status=&limit=200&offset=0`：分页筛选图片审核队列。
- `POST /api/v1/admin/images/{public_id}/freeze`
- `DELETE /api/v1/admin/images/{public_id}`
- `POST /api/v1/admin/users/{id}/grant-plan`
- `POST /api/v1/admin/users/{id}/ban`
- `POST /api/v1/admin/users/{id}/unban`
- `POST /api/v1/admin/users/{id}/subscription/expire`：强制订阅到期，默认进入 30 天只读保留期；`delete_now=true` 可配合清理任务执行删除。
- `POST /api/v1/admin/jobs/purge-expired`

## 部署模式

- 开发模式：不配置 `DATABASE_URL` 时使用内存仓储，适合本地测试。
- 生产模式：配置 `DATABASE_URL` 后自动迁移并使用 PostgreSQL 快照持久化，必须配置 `QUEUE_DRIVER=redis` 让上传投递 `image.process` 任务并由 `cmd/worker` 消费，配置 `STORAGE_DRIVER=s3` 后文件进入私有对象桶。
- Worker 可靠性：`WORKER_CLAIM_IDLE` 后自动回收 pending 任务，`WORKER_RETRY_LIMIT` 次失败后写入 `QUEUE_DEAD_LETTER_STREAM` 并 ACK 原任务，便于告警、人工排查和重放。
- 观测模式：`GET /metrics` 输出 Prometheus 文本指标，可接入 Grafana/Alertmanager。
