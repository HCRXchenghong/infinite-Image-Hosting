# 悦享图床

悦享图床是一套面向商业化 SaaS 的图片托管、分发、订阅计费与后台治理系统。项目采用 monorepo 组织，包含 Go 后端、用户端控制台、管理员后台、图片处理 Worker、生产 Compose、Nginx/Cloudflare/Prometheus 部署资料。

## 功能特性

- 用户端：注册登录、邮箱验证、图片上传、公开/私有读、短期签名链接、相册、API Key、资源包订阅、订单状态、备份导入导出、账号销毁工单。
- 管理端：首次使用初始化管理员、扫码绑定 TOTP 2FA、运营总览、用户管理、套餐管理、隐藏 Infinite Max 权益、邀请活动、图片审核、WAF/防盗链、订单对账、备份预检、审计日志、队列 dead-letter 重放。
- 支付与 OAuth：IF-Pay OAuth 授权、支付下单、Webhook 摘要/RSA 验签、事件去重、订单生命周期、后台运行时配置支付/OAuth 参数。
- 图片链路：S3/R2/MinIO 私有桶、Referer 防盗链、私有图鉴权、派生图分发、WebP/AVIF/缩略图处理、违规 pHash 冻结。
- 生产可靠性：PostgreSQL 快照持久化、Redis Stream Worker、pending 回收、失败重试、dead-letter 队列、Prometheus 指标、上线检查清单与 Runbook。

## 仓库结构

```text
.
├── yuexiang-image-backend      # Go/Gin API、Worker、迁移、部署文档
├── yuexiang-image-user-web     # React/Vite 用户端
├── yuexiang-image-admin-web    # React/Vite 管理端
├── docker-compose.full.yml     # API、Worker、双前端、Postgres、Redis、MinIO 一体化编排
├── Makefile                    # 本地验证入口
└── .github/workflows/ci.yml    # GitHub Actions CI
```

## 技术栈

- Backend：Go 1.25、Gin、PostgreSQL、Redis Stream、MinIO/S3、Prometheus metrics
- Frontend：React、TypeScript、Vite、Tailwind CSS、lucide-react
- Image pipeline：libvips 可用时生成 WebP、AVIF 与缩略图；无 libvips 时不会伪造派生图元数据
- Deployment：Docker、Docker Compose、Nginx、Cloudflare、Prometheus

## 快速启动

### 1. 启动后端

默认不配置 `DATABASE_URL` 时使用内存仓储，适合快速体验。

```bash
cd yuexiang-image-backend
go mod download
go run ./cmd/api
```

API 默认监听 `http://localhost:8080`。

### 2. 启动用户端

```bash
cd yuexiang-image-user-web
npm ci
npm run dev
```

用户端默认地址：`http://localhost:5173`。

### 3. 启动管理端

```bash
cd yuexiang-image-admin-web
npm ci
npm run dev
```

管理端默认地址：`http://localhost:5174`。首次进入管理端需要创建首个管理员账号，并使用 Microsoft Authenticator、Google Authenticator 等应用扫描二维码绑定 2FA；后续登录需要邮箱、密码和动态验证码。

## 环境变量

前端默认通过 `/api/v1` 访问后端。若本地前端需要直连后端，可在对应前端目录创建 `.env.local`：

```bash
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

后端开发环境可参考：

```bash
cp yuexiang-image-backend/.env.example yuexiang-image-backend/.env
```

生产环境必须参考并替换所有占位值：

```bash
cp yuexiang-image-backend/.env.production.example yuexiang-image-backend/.env
```

生产关键配置包括 `JWT_SECRET`、`IMAGE_SIGNING_SECRET`、`DATABASE_URL`、`REDIS_ADDR`、`S3_*`、`SMTP_*`、`IFPAY_*`、`PUBLIC_BASE_URL`、`IMAGE_PUBLIC_BASE_URL`、`CORS_ALLOWED_ORIGINS` 和防盗链域名。生产模式会拒绝默认开发密钥、`REPLACE_WITH_*`、`change-me`、`minioadmin`、localhost/127.0.0.1 公网地址和非 HTTPS 回调。

IF-Pay 支付与 OAuth 参数支持在管理端 `API 接口管理` 页面运行时配置；环境变量仍可作为初始默认值。

## Docker Compose

一体化编排会启动 API、Worker、用户端、管理端、PostgreSQL、Redis 与 MinIO。运行前必须准备生产 `.env` 并替换占位密钥。

```bash
cp yuexiang-image-backend/.env.production.example yuexiang-image-backend/.env
# 编辑 yuexiang-image-backend/.env，替换所有 REPLACE_WITH_* 和默认弱密钥
docker compose -f docker-compose.full.yml up -d --build
```

独立后端生产编排：

```bash
cd yuexiang-image-backend
cp .env.production.example .env
docker compose -f docker-compose.prod.yml up -d --build
```

生产至少保留一个 `worker` 实例消费 `image.process` 队列。

## 常用命令

```bash
make verify
```

`make verify` 会执行：

- `go test ./...`
- `go vet ./...`
- 用户端生产构建
- 管理端生产构建
- Docker Compose 配置校验

单独执行：

```bash
make backend-test
make backend-vet
make user-build
make admin-build
make compose-config
```

## API 与文档

- API 概览：[yuexiang-image-backend/docs/api.md](yuexiang-image-backend/docs/api.md)
- 生产上线检查清单：[yuexiang-image-backend/docs/production-checklist.md](yuexiang-image-backend/docs/production-checklist.md)
- 生产 Runbook：[yuexiang-image-backend/docs/runbook.md](yuexiang-image-backend/docs/runbook.md)
- Nginx 图片鉴权配置：[yuexiang-image-backend/deploy/nginx/yuexiang-image.conf](yuexiang-image-backend/deploy/nginx/yuexiang-image.conf)
- Cloudflare 规则：[yuexiang-image-backend/deploy/cloudflare/rules.md](yuexiang-image-backend/deploy/cloudflare/rules.md)
- Prometheus 告警：[yuexiang-image-backend/deploy/prometheus/alerts.yml](yuexiang-image-backend/deploy/prometheus/alerts.yml)

运行后，用户端技术文档页面会在 `/docs` 展示 OpenAPI YAML 与 Markdown 文档内容。

## 管理端安全说明

- 管理端不读取 `ADMIN_TOKEN` 等前端环境变量，避免密钥进入浏览器构建产物。
- 首个管理员通过后台首次使用流程创建，并强制绑定 TOTP 2FA。
- 管理员会话使用独立 token，敏感操作会写入审计日志。
- 套餐发放、订单入账/取消/退款、封禁/解封、图片冻结/删除、队列重放等操作均要求后台鉴权。

## 生产注意事项

- PostgreSQL 开启自动备份、PITR 与慢查询日志。
- Redis 开启 AOF，限制内网访问或配置 ACL。
- S3/R2/MinIO Bucket 默认私有，禁止匿名列目录。
- 图片域名建议统一经 Nginx `auth_request` 或等价边缘鉴权进入后端校验。
- Cloudflare 建议启用 Managed Ruleset、Bot Fight、Rate Limiting 和 Cache Rules。
- Prometheus 抓取 `/metrics`，并监控队列 pending/lag、dead-letter 增长、HTTP 错误率、对象数量、存储用量和风险事件。

## GitHub 准备

仓库已配置 `.gitignore`，不会提交 `node_modules`、`dist`、`.env`、`.DS_Store`、`*.tsbuildinfo`、日志和本地运行产物。提交前建议执行：

```bash
make verify
find . -maxdepth 3 -type f \( -name ".env" -o -name ".DS_Store" -o -name "*.log" \)
```

如果第二条命令有输出，请确认不是需要清理的本地文件。
