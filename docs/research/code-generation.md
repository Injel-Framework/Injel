# Báo cáo Nghiên cứu: Compile-Time DI Code Generation vs. Runtime Reflection trong Injel

- **Dự án:** Injel Framework
- **Tác giả & Người duy trì:** Phạm Trương Thông (@TogPam)
- **Ngày:** 31/10/2026
- **Trạng thái:** Hoàn thành khảo sát — Quyết định: **Trì hoãn triển khai Compile-time DI Codegen cho MVP; Ưu tiên Startup Reflection + Project Scaffolding.**

---

## 1. Mục tiêu khảo sát

Trong hệ sinh thái Go, có hai trường phái chính để giải quyết bài toán Dependency Injection (DI):
1. **Runtime Reflection tại Startup:** Ví dụ: Uber Fx, Injel (hiện tại).
2. **Compile-time Code Generation:** Ví dụ: Google Wire (đã archived).

Tài liệu này đánh giá chi tiết tính khả thi, ưu nhược điểm, chi phí bảo trì và trải nghiệm phát triển (DX) của việc bổ sung một cơ chế sinh code DI tự động tại thời điểm biên dịch cho Injel.

---

## 2. So sánh Kỹ thuật

| Tiêu chí | Injel (Startup Reflection) | Compile-time Code Generation (kiểu Wire) |
|---|---|---|
| **Thời điểm phân tích dependency** | Startup (`app.Start`) — Chạy một lần duy nhất lúc khởi động ứng dụng | Build time (khi chạy công cụ CLI trước khi biên dịch) |
| **Chi phí trên HTTP Request path** | **0% (Zero)** — Instance đã được cache dạng singleton | **0% (Zero)** |
| **Quy trình phát triển (DX)** | Đơn giản: Khai báo constructor -> `app.Provide(NewService)` -> Chạy trực tiếp `go run` | Phức tạp: Mỗi khi đổi chữ ký hàm hoặc thêm dependency phải chạy thêm lệnh codegen |
| **Tính tự nhiên với Go toolchain** | 100% tuân thủ `go run`, `go build`, `go test` tiêu chuẩn, không cần bước trung gian | Cần cấu hình thêm `go:generate` hoặc bước build riêng biệt trong CI/CD |
| **Phát hiện lỗi (Fail-fast)** | Kiểm tra chu trình (cycle), dependency thiếu, provider trùng lúc startup trước khi bind cổng HTTP | Kiểm tra lúc sinh code hoặc compile code |
| **Chất lượng thông báo lỗi** | Thông báo lỗi ngữ cảnh rõ ràng, in đường dẫn chu trình chi tiết (`A -> B -> C -> A`) | Phụ thuộc vào AST parser; nếu code chưa compile được thì generator dễ bị crash |
| **Độ phức tạp mã nguồn Framework** | Module DI nhỏ gọn (~500 dòng code Go trong `di/`), dễ kiểm thử, ít bug | Cần viết AST parser, type checker, code printer phức tạp (> 2.000 dòng code) |
| **Hiệu năng khởi động thực tế** | **~0.1ms đến 1.5ms** cho ứng dụng 50 - 100 components | Gần 0ms lúc chạy (nhưng tốn thời gian lúc build) |

---

## 3. Đánh giá Thực tế trên Injel

1. **Hiệu năng Reflection tại Startup:**
   - Qua các bài benchmark và kiểm thử thực tế ở Phase 1 đến Phase 4, thời gian khởi tạo container và resolve toàn bộ dependency graph cho một ứng dụng REST API thông thường chỉ mất dưới **1 mili-giây**.
   - Đối với một backend server chạy liên tục nhiều tuần/tháng, chi phí 1ms tại thời điểm khởi động là hoàn toàn không đáng kể và không ảnh hưởng đến throughput hay latency của request.
2. **Trải nghiệm Lập trình viên (Developer Experience):**
   - Triết lý cốt lõi của Injel là *"Spring-inspired developer experience. Native Go simplicity."*
   - Nếu áp dụng compile-time codegen, lập trình viên sẽ phải:
     - Nhớ chạy lệnh generate mỗi khi thêm service mới.
     - Xử lý các file generated (thêm vào gitignore hoặc commit làm nhiễu pull request diff).
     - Đối mặt với lỗi IDE khi file generated chưa được cập nhật.
3. **Bài học từ Hệ sinh thái Go:**
   - Dự án Google Wire từng là đại diện tiêu biểu cho DI codegen trong Go nhưng hiện đã bị đưa vào trạng thái **Archived** do chi phí bảo trì AST quá lớn khi ngôn ngữ Go cập nhật các tính năng mới (Generics, Type Parameters, v.v.).
   - Ngược lại, Uber Fx (dựa trên reflection tại startup) vẫn là framework DI được sử dụng rộng rãi và thành công nhất trong cộng đồng Go production quy mô lớn.

---

## 4. Quyết định Chiến lược

1. **Giữ vững DI Core hiện tại:** Tiếp tục sử dụng cơ chế Reflection tại startup với Singleton Cache và Fail-fast Graph Validation. Không thay đổi API hay phá vỡ tính tương thích của `injel.App` và `di.Container`.
2. **Tập trung vào Project Scaffolding:** CLI ở Phase 5 tập trung sinh cấu trúc project khởi đầu (`injel new`), file cấu hình môi trường, route mẫu và graceful shutdown để lập trình viên có thể bắt đầu dự án ngay lập tức.
3. **Kế hoạch Tương lai:** Chỉ xem xét lại DI Code Generation khi:
   - Có kịch bản thực tế yêu cầu thời gian khởi động cực hạn (như AWS Lambda Cold Start < 5ms).
   - Có bài kiểm tra benchmark chứng minh reflection startup là điểm nghẽn thực sự.
   - Khi đó, codegen sẽ được phát triển dưới dạng một công cụ bổ trợ tùy chọn (opt-in tool), không thay thế core framework.
