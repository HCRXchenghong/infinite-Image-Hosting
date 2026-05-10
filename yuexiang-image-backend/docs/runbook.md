# 悦享图床生产 Runbook

## 快速判断

- `/healthz` 只代表进程存活；`/readyz` 会检查 PostgreSQL、Redis 队列和对象存储，负载均衡应以 `/readyz` 为准。
- `/metrics` 输出业务、HTTP 和队列指标；Prometheus 告警规则示例位于 `deploy/prometheus/alerts.yml`。
- 管理端 `任务队列状态` 页面用于查看 Redis Stream length、pending、lag、dead-letter，并支持确认修复后的 dead-letter 重新投递。

## Dead-letter 处理流程

1. 打开管理端 `任务队列状态`，确认 dead-letter 数量、任务类型和 payload。
2. 查看 worker 日志，先修复根因：常见原因包括对象桶临时不可达、libvips 缺失、图片源对象被删除、权限错误。
3. 对同类型任务抽样确认 payload 合法，避免把不可恢复任务无限回放。
4. 点击 `重新投递`，观察主队列 lag/pending 是否下降，图片元数据是否补齐。
5. 如果同一任务再次进入 dead-letter，停止回放并升级为代码/数据修复，不要批量重试。

## 支付与订阅异常

- 用户端资源包页会先引导完成 IF-Pay OAuth 授权，再调用 `POST /api/v1/checkout/ifpay` 创建订单；用户可在同页看到最近订单状态。
- IF-Pay webhook 会根据事件类型驱动订单状态流转：`payment.succeeded` 置为 `paid` 并发放订阅，`payment.failed`/`payment.expired` 置为 `failed`，`payment.cancelled`/`payment.canceled` 置为 `cancelled`，`payment.refunded`/`payment.refund.succeeded` 置为 `refunded` 并在命中当前订阅时切入只读保留期。
- webhook 丢失但财务确认收款时，在管理端 `订单与订阅` 使用 `入账`，填写对账原因；接口是幂等的，重复点击不会重复发放有效订阅。
- 未支付重复订单使用 `取消`。
- 退款使用 `退款`，匹配当前订阅时会切入只读保留期，并写审计日志。
- 争议或违规账号可在 `用户管理` 使用 `到期`，立即停止写入并进入 30 天保留期。

## 内容安全

- 已确认违规图片的 pHash 可加入 `MODERATION_BLOCKED_HASHES`，Worker 后续处理命中后会自动冻结对象。
- 自动冻结会同时写入风险事件和审计日志；管理端 `图片内容审核` 可按 `frozen` 状态、Public ID、用户 ID 或 pHash 检索。
- 对外溢副本无法远程删除，只能依赖 pHash、盲水印、Referer/WAF 和法务下架链路做追踪与处置。

## 备份与恢复

- 定期通过 `GET /api/v1/admin/backups/export` 导出管理备份包，并保存到异地介质。
- 恢复前先在管理端上传 ZIP 做预检；预检只校验 manifest/checksum 和对象数量，不会覆盖线上数据。
- 生产覆盖恢复必须在维护窗口、离线演练通过后执行。
