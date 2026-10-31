# GitHub Repository Metadata & Branding Guidelines

Tài liệu này chuẩn hóa metadata, mô tả (description), chủ đề (topics) và tài nguyên hiển thị (social preview) cho repository chính thức của **Injel Framework** trên GitHub:

- **Repository:** [https://github.com/Injel-Framework/Injel](https://github.com/Injel-Framework/Injel)
- **Tác giả & Người duy trì:** Phạm Trương Thông ([@TogPam](https://github.com/TogPam))

---

## 1. Repository Description

Đoạn mô tả ngắn (About Description) được tối ưu hóa cho công cụ tìm kiếm và lập trình viên:

```text
Injel is a Go (Golang) backend framework featuring dependency injection, IoC container, environment configuration, HTTP server, middleware, and graceful application lifecycle management.
```

### Tiêu chí:
- Giới hạn dưới 350 ký tự (chuẩn hiển thị của GitHub About box).
- Nêu rõ ngôn ngữ (Go/Golang), phân loại (Backend Framework), và các tính năng cốt lõi đã có thực tế trong code.
- Tuyệt đối không nhồi nhét từ khóa phi tự nhiên.

---

## 2. GitHub Topics (Repository Tags)

Danh sách các topics chính thức được đề xuất gắn trên GitHub:

```text
go
golang
go-framework
backend
backend-framework
web-framework
dependency-injection
ioc-container
rest-api
http-server
middleware
configuration
application-lifecycle
developer-tools
```

### Tiêu chí lựa chọn:
- Tất cả các topic đều phản ánh đúng 100% tính năng đã triển khai qua Phase 1, Phase 2, Phase 3, và Phase 4.
- Giúp lập trình viên Go dễ dàng tìm thấy Injel qua GitHub Explore và các chủ đề liên quan.

---

## 3. Website & About Settings

- **Website URL:** Để trống nếu chưa có landing page hoặc domain tài liệu riêng. (Tuyệt đối không điền link giả hoặc placeholder).
- **Include in home page:**
  - [x] Releases
  - [x] Packages
  - [x] Environments

---

## 4. Social Preview Asset (Open Graph Image)

Ảnh hiển thị khi chia sẻ link repository trên Twitter/X, LinkedIn, Facebook, Slack, Discord.

- **Kích thước chuẩn:** 1280 × 640 pixels (tỉ lệ 2:1).
- **Định dạng:** PNG hoặc JPEG.
- **Dung lượng:** < 1 MB.
- **Nội dung hiển thị:**
  - Tên thương hiệu: **Injel**
  - Định vị sản phẩm: **A Go (Golang) Backend Framework**
  - Triết lý / Tagline: *Spring-inspired developer experience. Native Go simplicity.*
  - Tác giả: *By Phạm Trương Thông (@TogPam)*
  - Phong cách: Tối giản, hiện đại, màu sắc chủ đạo gợi nhớ hệ sinh thái Go (Cyan/Blue #00ADD8 kết hợp Dark/Slate background).

---

## 5. Hướng dẫn thiết lập thủ công trên GitHub Web

Do môi trường dòng lệnh không có sẵn GitHub CLI (`gh`), người quản trị repository có thể thực hiện thiết lập trực tiếp trên web theo các bước:

1. Truy cập [https://github.com/Injel-Framework/Injel](https://github.com/Injel-Framework/Injel).
2. Tại trang chủ repository, nhìn sang góc trên bên phải của phần **About** (bên dưới số sao/fork), nhấn vào biểu tượng **bánh răng (Edit repository details)**.
3. **Description:** Dán chuỗi mô tả ở Mục 1.
4. **Topics:** Lần lượt gõ và chọn các từ khóa ở Mục 2.
5. Nhấn **Save changes**.
6. Vào tab **Settings** của repository -> mục **General** -> cuộn xuống phần **Social preview** -> nhấn **Edit** và tải lên hình ảnh 1280x640 đã chuẩn bị.
