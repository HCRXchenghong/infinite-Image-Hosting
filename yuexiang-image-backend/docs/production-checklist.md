# 生产上线检查清单

## 必填密钥

- `JWT_SECRET`：至少 32 位随机值。
- `IMAGE_SIGNING_SECRET`：至少 32 位随机值。
- `DATABASE_URL`：PostgreSQL 主库连接串。
- `PUBLIC_BASE_URL`、`IMAGE_PUBLIC_BASE_URL`：必须是 HTTPS 公网地址。
- `CORS_ALLOWED_ORIGINS`：显式前端域名列表，生产禁止 `*`。
- `ALLOWED_REFERER_DOMAINS`、`BLOCKED_REFERER_DOMAINS`、`ALLOW_EMPTY_REFERER`：图片防盗链策略。
- `S3_BUCKET`、`S3_ACCESS_KEY`、`S3_SECRET_KEY`：私有对象桶配置。
- `IFPAY_PRIVATE_KEY_PEM`、`IFPAY_WEBHOOK_PUBLIC_KEY_PEM`：IF-Pay 出站签名和入站验签。
- `SMTP_HOST`、`SMTP_USERNAME`、`SMTP_PASSWORD`：邮箱验证和密码重置邮件。
- 上线进程会拒绝 `change-me`、`minioadmin`、`REPLACE_WITH_*`、localhost/127.0.0.1 和非 HTTPS 公网回调。
- 管理端首次使用必须创建首个管理员账号，并使用 Authenticator 扫描二维码绑定 2FA；不要在管理端构建环境中注入任何 `VITE_*TOKEN*` 或密钥变量。

## 基础设施

- PostgreSQL 开启自动备份、PITR 和慢查询日志。
- Redis 开启 AOF，并设置内网访问或 ACL。
- 至少部署 1 个 `yuexiang-worker`，并监控 Redis Stream `yuexiang:image:tasks` 的 pending/lag，以及 `yuexiang:image:tasks:dead` 的增长。
- 设置 `WORKER_RETRY_LIMIT` 和 `WORKER_CLAIM_IDLE`；dead-letter 任务必须进入告警渠道，可通过管理端 `任务队列状态` 页面排查并在确认修复后重放。
- S3/R2/MinIO Bucket 默认私有，禁止匿名列目录。
- Nginx 启用 `auth_request`，图片访问统一走后端鉴权。
- Cloudflare 开启 Managed Ruleset、Bot Fight、Rate Limiting 和 Cache Rules。
- Prometheus 抓取 `/metrics`，至少告警用户数、对象数、存储字节、风险事件增长、API 错误率、队列 pending/lag 和 dead-letter 增长；规则示例见 `deploy/prometheus/alerts.yml`。
- 负载均衡或 K8s readiness 使用 `/readyz`；该接口会检查 Redis、PostgreSQL 和对象存储依赖。

## 业务规则

- `infinite-max` 只能通过后台发放或内部邀请兑换，必须写入审计原因。
- 防盗链策略可在管理端实时更新；生产建议关闭空 Referer，并保留至少一个允许域名。
- 套餐过期进入 30 天只读保留期，保留期后由 `POST /api/v1/admin/jobs/purge-expired` 清理。
- 支付异常必须走后台订单动作闭环：`mark-paid` 用于人工对账入账，`cancel` 用于取消未支付订单，`refund` 用于退款并终止匹配订阅；每次操作都必须填写 `reason` 并写入审计日志。
- API Key 创建后只展示一次，服务端仅保存 HMAC 哈希。
- IF-Pay webhook 必须配置摘要和 RSA 验签，事件 ID 必须去重。
- IF-Pay OAuth callback 必须走授权码换 token 和 userinfo 绑定账号，不允许生产假用户。
- 已确认违规内容的 pHash 应加入 `MODERATION_BLOCKED_HASHES`；Worker 会在图片处理时自动比对，命中后冻结对象、写入风险事件和审计日志。
- 备份 ZIP 必须包含 `manifest.json` 和 `checksums.sha256`，导入前先校验完整性；恢复时为当前账号生成新 Public ID，不覆盖线上对象。
- 管理端全量备份可通过 `GET /api/v1/admin/backups/export` 导出；线上恢复前必须先用 `POST /api/v1/admin/backups/import/validate` 预检，并在离线环境演练。
- 图片派生图必须由 API 或 Worker 真实写入对象桶；没有 libvips 时不能伪造 pending 元数据。

## 验证命令

```bash
go test ./...
go vet ./...
npm run build
docker compose -f docker-compose.prod.yml config
```
