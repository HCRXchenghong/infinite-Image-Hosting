# Cloudflare 防护规则基线

第一版建议开启：

- WAF 托管规则集：开启 Cloudflare Managed Ruleset。
- Bot Fight / Super Bot Fight：对明显自动化请求挑战或阻断。
- Rate Limiting：对 `/i/*`、`/api/v1/images`、`/api/v1/checkout/ifpay` 分别设置阈值。
- Cache Rules：公开图 `/i/*` 可缓存；私密签名图按 query token 缓存或绕过缓存。
- Transform Rules：回源时透传 `CF-Connecting-IP`、`Referer`、`User-Agent`。
- Hotlink：不要只依赖 Cloudflare Hotlink Protection，仍以悦享后端/Nginx 鉴权为准。

推荐阈值：

- `/api/v1/images`：每账号每分钟 300 次，匿名 IP 每分钟 30 次。
- `/api/v1/checkout/ifpay`：每账号每分钟 20 次。
- `/i/*`：按套餐和风险分层；异常 IP/ASN 触发 JS Challenge 或 Managed Challenge。

