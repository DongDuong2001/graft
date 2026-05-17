[English](README.md) | [Tiếng Việt](README-vi.md)

# Graft: Cầu nối Webhook tự lưu trữ (Self-Hosted Webhook Bridge)

<a href="https://forg.to/products/graft" target="_blank" rel="noopener">
  <img src="https://forg.to/api/badges/upvote/graft?theme=dark&shape=square" alt="Graft - Upvote on Forg" height="40" />
</a>
<a href="https://unikorn.vn/p/graft?ref=embed-graft" target="_blank" rel="noopener">
  <img src="https://unikorn.vn/api/widgets/badge/graft?theme=light" alt="Graft trên Unikorn.vn" height="40" />
</a>
<a href="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml" target="_blank" rel="noopener">
  <img src="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml/badge.svg" alt="CI" height="40" />
</a>

Graft là một cầu nối webhook-to-anything viết bằng Go. Ứng dụng nhận webhook tại `/hook/{path}`, có thể xác thực chữ ký (tùy cấu hình), biến đổi JSON, và chuyển tiếp kết quả tới một hoặc nhiều đích. Graft được thiết kế để tự lưu trữ và chạy trên Docker hoặc Kubernetes.

## Khởi động nhanh (Docker)

1. Tạo `.env` từ file mẫu và điền giá trị:

```bash
cp configs/example.env .env
```

2. Khởi chạy dịch vụ:

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

3. Kiểm tra:

```bash
curl http://localhost:8080/healthz
```

## Tính năng

- Ingress: nhận webhook POST tại `/hook/{path}`
- Routing: định tuyến theo đường dẫn URL
- Bảo mật: xác thực chữ ký, Admin API key, mã hóa AES-GCM khi lưu trữ
- Biến đổi: `text/template` và pipeline steps
- Độ bền: retries và timeouts có thể cấu hình cho outbound
- Quan sát: endpoint metrics (`/metrics`) và log JSON có cấu trúc
- Lưu trữ: SQLite cho rules và lịch sử delivery

## Yêu cầu

- Go 1.25+ (phát triển local)
- Docker với Compose v2 (`docker compose`) (triển khai)
- Tùy chọn: OpenSSL (tạo key), Make (tiện lệnh)

## Cấu hình

Graft đọc cấu hình qua biến môi trường. Xem `configs/example.env`.

| Biến | Mô tả | Bắt buộc | Mặc định |
| :--- | :--- | :--- | :--- |
| `MASTER_KEY` | Chuỗi hex 32-byte (64 ký tự hex) để mã hóa khi lưu trữ. | Có | - |
| `ADMIN_API_KEY` | Khóa truy cập Admin API. | Có | - |
| `DB_PATH` | Đường dẫn file SQLite. | Không | `./rules.db` |
| `PORT` | Cổng HTTP lắng nghe. | Không | `8080` |
| `FORWARD_TIMEOUT` | Timeout cho outbound request. | Không | `30s` |
| `FORWARD_MAX_RETRIES` | Số lần retry outbound. | Không | `3` |
| `FORWARD_RETRY_BASE` | Backoff cơ sở cho retry. | Không | `200ms` |

Tạo master key:

```bash
openssl rand -hex 32
```

## Chạy local

SQLite dùng `github.com/mattn/go-sqlite3` và cần CGO. Nếu máy bạn bị skip test storage/httpapi/integration, hãy chạy với C toolchain và `CGO_ENABLED=1`.

```bash
cp configs/example.env .env
go run cmd/graft/main.go
```

## Sử dụng API

Tất cả Admin API nằm dưới `/api/v1/` và yêu cầu header:

```
Authorization: Bearer <ADMIN_API_KEY>
```

Tạo rule:

```bash
curl -X POST http://localhost:8080/api/v1/rules \
  -H "Authorization: Bearer YOUR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Push to Slack",
    "listen_path": "/hook/github-push",
    "required_signature": true,
    "signature_header": "X-Hub-Signature-256",
    "signature_format": "hex",
    "signature_secret": "your-github-webhook-secret",
    "destination_url": "https://hooks.slack.com/services/...",
    "destination_method": "POST"
  }'
```

Liệt kê rules:

```bash
curl http://localhost:8080/api/v1/rules \
  -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

Gửi webhook:

```bash
curl -X POST http://localhost:8080/hook/github-push \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=generated-signature..." \
  -d '{"ref":"refs/heads/main"}'
```

## Kiểm thử

```bash
go test ./...
go test -v ./...
```

## Quy trình AI

Repo này dùng PUDO (Plan, Understand, Develop, Optimize). Xem `AGENTS.md` và `.agents/workflows/pudo.md`.

## Giấy phép

[MIT License](LICENSE)

## Đóng góp

Xem `CONTRIBUTING.md`.
