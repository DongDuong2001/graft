[English](README.md) | [Tiếng Việt](README-vi.md)

# Graft: Cầu nối Webhook tự lưu trữ (Self-Hosted Webhook Bridge)

<a href="https://forg.to/products/graft" target="_blank" rel="noopener">
  <img src="https://forg.to/api/badges/upvote/graft?theme=dark&shape=square" alt="Graft - Upvote on Forg on forg." height="40" />
</a>
<a href="https://unikorn.vn/p/graft?ref=embed-graft" target="_blank">
  <img src="https://unikorn.vn/api/widgets/badge/graft?theme=light" alt="Graft trên Unikorn.vn" height="40" />
</a>

<a href="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml" target="_blank">
  <img src="https://github.com/DongDuong2001/graft/actions/workflows/ci.yml/badge.svg" alt="CI" height="40" />
</a>

Graft là một cầu nối webhook-to-anything nhẹ và an toàn được viết bằng Go. Nó nhận webhook đến, xác thực chữ ký (từ các nhà cung cấp như GitHub hoặc Stripe), tùy chọn thay đổi JSON payload thông qua template, và chuyển tiếp kết quả đến một đích khác. Nó được thiết kế để dễ dàng tự lưu trữ và chạy trong Docker hoặc Kubernetes.


## Tính năng (Features)

- **Đầu vào (Ingress)**: Chấp nhận các webhook POST tại `/hook/{path}`.
- **Định tuyến (Routing)**: Khớp các yêu cầu dựa trên đường dẫn URL.
- **Bảo mật (Security)**:
  - Xác thực chữ ký webhook (HMAC-SHA256, Stripe v1).
  - Admin API được bảo vệ bằng API key.
  - Các thông tin bí mật (như webhook signing keys) được mã hóa tại chỗ (encrypted at rest) bằng AES-GCM.
- **Chuyển đổi (Transformation)**: Hỗ trợ `text/template` để sửa đổi payload trước khi chuyển tiếp.
- **Tính khả dụng (Resilience)**: Có thể cấu hình số lần thử lại (retries) và thời gian chờ (timeouts) cho các yêu cầu chuyển tiếp.
- **Khả năng quan sát (Observability)**: Endpoint `/metrics` sẵn sàng tích hợp với Prometheus và log được thu thập dưới dạng JSON có cấu trúc.
- **Lưu trữ (Storage)**: Sử dụng SQLite để duy trì các quy tắc (rules) và lịch sử quá trình gửi.


## Yêu cầu Hệ thống (Prerequisites)

- **Go 1.26+** (cho môi trường phát triển cục bộ)
- **Docker** & **Docker Compose** (được khuyến nghị để triển khai)
- **OpenSSL** (tùy chọn, dùng để tạo khóa truy cập)
- **Make** (tùy chọn, hỗ trợ gọi lệnh dễ dàng)

### Cài đặt Make trên Windows

Nếu bạn chưa cài đặt `make`, bạn có thể cài đặt thông qua biểu mẫu Chocolatey hoặc Scoop:

**Chocolatey:**
```powershell
choco install make
```

**Scoop:**
```powershell
scoop install make
```

Hoặc bạn có thể chạy trực tiếp các lệnh `go` như mô tả bên dưới.


## Cấu hình (Configuration)

Graft được cấu hình qua các biến môi trường (environment variables). Xem [`configs/example.env`](configs/example.env) để lấy file mẫu.

### Các tham số quan trọng

| Biến (Variable) | Mô tả | Bắt buộc | Mặc định |
| :--- | :--- | :--- | :--- |
| `MASTER_KEY` | Chuỗi 32 byte dạng hex dùng để mã hóa các thông tin lưu trữ. | **Có** | - |
| `ADMIN_API_KEY` | Khóa bí mật dùng để truy cập Admin API. | **Có** | - |
| `DB_PATH` | Đường dẫn tới tệp database SQLite. | Không | `./rules.db` |
| `PORT` | Cổng HTTP để ứng dụng lắng nghe. | Không | `8080` |

### Tạo một Master Key

```bash
openssl rand -hex 32
```


## Chạy trên máy cụ bộ (Running Locally)

### Sử dụng Make

Nếu bạn đã cài `make`, bạn có thể sử dụng file `Makefile` có sẵn cho các tác vụ phổ biến:

- `make build`: Biên dịch mã nguồn thành file thực thi `bin/graft`.
- `make run`: Chạy trực tiếp ứng dụng.
- `make test`: Chạy toàn bộ các bài unit/integration test.
- `make vet`: Phân tích lỗi cú pháp qua `go vet`.
- `make docker-build`: Tiến hành tải về và Build Docker Image.
- `make clean`: Xóa đi các tệp đã build trước đó.

### Các bước cài đặt thủ công

1.  **Clone mã nguồn (Clone the repository):**
    ```bash
    git clone https://github.com/your-org/graft.git
    cd graft
    ```

2.  **Thiết lập môi trường:**
    Sao chép `configs/example.env` thành tệp `.env` và điền thông số của bạn vào.
    ```bash
    cp configs/example.env .env
    # Mở và tùy chỉnh .env để khai báo MASTER_KEY và ADMIN_API_KEY
    ```

3.  **Khởi động ứng dụng:**
    ```bash
    # Đọc các biến môi trường (ví dụ sử dụng lệnh export)
    # Trên Linux/Mac:
    export $(grep -v '^#' .env | xargs)
    go run cmd/graft/main.go
    ```
    
    *Lưu ý: Đảm bảo `CGO_ENABLED=1` đã được cài đặt nếu trong môi trường không bật mặc định, bởi vì SQLite yêu cầu dùng CGO.*

4.  **Kiểm tra xem nó đã hoạt động chưa:**
    ```bash
    curl http://localhost:8080/healthz
    # Out put (Hợp lệ): {"status":"ok"}
    ```


## Chạy sử dụng Docker (Running with Docker)

Sử dụng `docker-compose.yml` có sẵn để thiết lập nhanh Graft.

1.  **Cấu hình `.env`:**
    Đảm bảo `configs/example.env` (hoặc tệp `.env` của bạn) chứa các key hợp lệ.

2.  **Bật dịch vụ Service:**
    ```bash
    docker-compose -f deployments/docker-compose.yml up -d --build
    ```

    Graft ngay lúc này sẽ hoạt động tại địa chỉ `http://localhost:8080`. Toàn bộ dữ liệu của bạn sẽ được lưu giữ vĩnh viễn trong Volume có tên `graft-data`.


## Cách thức dùng API (API Usage)

### Admin API

Toàn bộ các API cho Admin đều nằm tại phân vùng `/api/v1/` và yêu cầu header cần có `Authorization: Bearer <ADMIN_API_KEY>`.

#### Tạo một quy tắc (Create a Rule)

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

#### Liệt kê các quy tắc (List Rules)

```bash
curl http://localhost:8080/api/v1/rules -H "Authorization: Bearer YOUR_ADMIN_KEY"
```

### Gửi thông điệp Webhooks

Một khi Rule bên trên đã được thiết lập thành công, bạn có thể gọi webhook cho nó qua địa chỉ config `listen_path`.

```bash
# Ví dụ cho cấu hình như bên trên
curl -X POST http://localhost:8080/hook/github-push \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=generated-signature..." \
  -d '{"ref": "refs/heads/main", ...}'
```


## Kiểm thử (Testing)

Khởi động các bài test tự động thông qua câu lệnh `go test`. Hãy lưu ý rằng Integration Tests cần có CGO được bật để SQLite có thể thực thi thành công.

```bash
go test ./...
# Kích hoạt Output chi tiết
go test -v ./...
```

Chạy riêng dòng các bài unit test (bỏ qua những bài đòi hỏi Setup bên ngoài và Data nặng lớn):
```bash
go test -short ./...
```


## Giấy phép (License)

Nền tảng của chúng tôi áp dụng giấy phép [MIT License](LICENSE)


## Đóng góp (Contributing)

Vui lòng tham khảo bộ [Hướng dẫn Đóng góp (Contributing Guidelines)](CONTRIBUTING.md) chặt chẽ của chúng tôi để biết về quy tắc ứng xử, chiến lược chia nhánh, tiêu chuẩn viết commit, và quy trình thực hiện pull request.
