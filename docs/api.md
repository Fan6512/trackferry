# 接口

管理页使用的 JSON 接口，也可以给脚本用。接口可能随版本调整。

## 鉴权

- 浏览器：登录后的会话 Cookie，写操作加请求头 `X-Requested-With: trackferry`。
- 脚本：`Authorization: Bearer <data/private/admin_token 的内容>`。

错误返回 `{"error": "代号", "message": "给人看的说明"}`。

## 无需登录

| 接口 | 说明 |
| --- | --- |
| `GET /healthz/live` | 进程在运行 |
| `GET /healthz/ready` | 数据库与目录可用，含版本 |
| `GET /api/session` | 登录状态与版本 |
| `POST /api/setup` · `/api/login` · `/api/logout` | 初始化、登录、退出 |
| `GET /stream/<令牌>` | 音箱取歌 |

## 点歌与音箱

| 接口 | 说明 |
| --- | --- |
| `GET /api/commands?limit=&before=` | 点歌记录 |
| `PATCH /api/commands/{id}` | 标注小爱当时的反应 |
| `POST /api/route/try` `{"text":"播放晴天"}` | 试一句，不出声 |
| `GET /api/intent` · `POST /api/intent` | 说法示例；解释一句话 |
| `GET /api/devices` · `POST /api/devices/sync` · `PATCH /api/devices/{id}` | 音箱列表、同步、改模式或别名 |
| `POST /api/devices/{id}/takeover-test` | 接管测试，会出声 |
| `GET /api/takeover/runs?limit=` | 接管经过 |
| `GET /api/observer` | 监听状态 |
| `GET /api/live` | 实时事件（SSE）：`hello` 全量，`flow` 一条点歌的变化，`pulse` 一次监听 |

## 曲库与音源

| 接口 | 说明 |
| --- | --- |
| `GET /api/library` · `POST /api/library/scan` | 状态；同步 |
| `GET /api/library/search?q=` · `/shelf` · `/albums/{id}` · `/artist` | 搜索与浏览 |
| `GET /api/library/tracks/{id}/lyrics` | 歌词 |
| `GET·PUT /api/library/tracks/{id}/favorite` | 收藏 |
| `GET /api/cover/{id}` | 封面 |
| `PATCH /api/sources/{kuwo\|kugou\|migu}` | 音源开关 |
| `GET·POST·DELETE /api/feiniu` · `GET /api/feiniu/discover` | 飞牛音乐登录与查找 |

## 账号与系统

| 接口 | 说明 |
| --- | --- |
| `GET·DELETE /api/xiaomi` | 关联状态、解除 |
| `/api/xiaomi/qr`、`/qr.png` | 扫码登录 |
| `/api/xiaomi/password`、`/captcha`、`/verify` | 密码登录 |
| `POST /api/password` | 改管理密码 |
| `GET /api/status` | 运行状态 |
| `GET /api/update` · `POST /api/update` | 上次检查结果与安装进度；立即检查 |
| `POST /api/update/install` | 安装最新版本并重启 |
| `PUT /api/update/proxy` | 设置更新用的代理 `{"proxy": "http://192.168.1.2:7890"}`，空字符串为直接连接 |
