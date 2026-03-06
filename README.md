# Telegram 翻译机器人（Go）

基于 Go 1.21+、Telegram Bot API、Google Cloud Translation API v3、PostgreSQL 与 Redis 实现的生产级翻译机器人。

## 功能概览

- 双向翻译模式
  - 自动模式：识别非中文并翻译为简体中文。
  - 设定模式：用户设置目标语种后，中文 -> 目标语种，目标语种 -> 中文。
- 支持语种：`en`、`ru`、`fr`、`de`、`it`、`ja`、`ko`、`th`、`vi`
- 使用 `DetectLanguage API` 进行语种识别
- PostgreSQL 持久化用户设置：`UserID`、目标语种、机器人开关、自动模式
- Redis 翻译缓存，减少 Google API 消耗
- 月度额度控制（50 万字符）
  - 80%（40 万）自动管理员预警
  - 90%（45 万）自动熔断并暂停翻译服务
- Inline Keyboard 全中文菜单
- 白名单访问控制（`ALLOWED_USERS`）
- 每条翻译请求独立 goroutine 并发处理
- 支持优雅关闭，确保连接释放

## 项目结构

```text
.
├── main.go
├── bot/
├── cache/
├── config/
├── database/
├── quota/
├── translator/
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

## 环境变量

| 变量名 | 必填 | 默认示例值 | 说明 |
|---|---|---|---|
| `BOT_TOKEN` | 是 | `123456789:AAEXAMPLE_TOKEN` | Telegram Bot Token |
| `ALLOWED_USERS` | 是 | `123456789,987654321` | 可使用机器人用户 ID 白名单（逗号分隔） |
| `POSTGRES_DSN` | 是 | `postgres://postgres:postgres@postgres:5432/tg_translate?sslmode=disable` | PostgreSQL 连接串 |
| `REDIS_ADDR` | 是 | `redis:6379` | Redis 地址 |
| `GOOGLE_APPLICATION_CREDENTIALS` | 是 | `/secrets/google-credentials.json` | Google 服务账号凭据文件路径 |
| `GOOGLE_PROJECT_ID` | 是 | `your-gcp-project-id` | Google Cloud Project ID |
| `GOOGLE_LOCATION` | 否 | `global` | Translation API 地域 |
| `ADMIN_USERS` | 否 | `123456789` | 管理员 ID，用于额度预警/熔断通知；为空时回退到 `ALLOWED_USERS` |
| `REDIS_PASSWORD` | 否 | 空 | Redis 密码 |
| `REDIS_DB` | 否 | `0` | Redis DB |
| `DEFAULT_TARGET_LANGUAGE` | 否 | `en` | 新用户默认目标语种 |
| `REQUEST_TIMEOUT` | 否 | `10s` | 单次请求超时 |
| `CACHE_TTL` | 否 | `720h` | 翻译缓存 TTL |

## 本地运行

1. 安装 Go 1.21+。
2. 开启 Google Cloud Translation API，并准备服务账号 JSON。
3. 配置环境变量：

```bash
cp .env.example .env
# 按实际值修改 .env
```

4. 启动依赖（PostgreSQL + Redis）：

```bash
docker compose up -d postgres redis
```

5. 启动机器人：

```bash
export $(grep -v '^#' .env | xargs)
go mod tidy
go run ./main.go
```

## Docker / Docker Compose 部署（1Panel 适配）

1. 将凭据文件放入 `./credentials/google-credentials.json`。
2. 配置 `.env`。
3. 启动全部服务：

```bash
docker compose up -d --build
```

1Panel 中可直接导入本 `docker-compose.yml`，并在面板内配置环境变量与挂载卷。

## Telegram 使用说明

- `/start` 或 `/menu`：打开主菜单
- `/set <语种代码>`：切换设定模式目标语种（示例：`/set ja`）
- `/auto on|off`：开启/关闭自动模式
- `/status`：查看个人设置与额度
- 菜单按钮（Inline Keyboard）：
  - 切换语种
  - 自动模式
  - 本月额度
  - 个人设置

## 配额与熔断策略

- 月额度基线：`500000` 字符
- 预警阈值：`400000`（80%）
- 熔断阈值：`450000`（90%）
- 熔断后用户收到提示：`本月额度已耗尽，翻译服务已暂停，将于下月自动恢复。`

## 生产建议

- 建议通过私有网络连接 PostgreSQL / Redis。
- 建议开启日志采集（如 Loki、ELK）以审计翻译失败率和熔断行为。
- `ALLOWED_USERS` 请仅配置可信账号，避免 Token 泄漏后被滥用。
