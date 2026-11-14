# Protobuf 编译依赖

`google/api/annotations.proto` 和 `google/api/http.proto` 来自 grpc-gateway v1.16.0 随附的 Google APIs proto，保留上游版权与许可证声明。[上游文件](https://github.com/grpc-ecosystem/grpc-gateway/tree/v1.16.0/third_party/googleapis/google/api)

`google/protobuf/descriptor.proto` 和 `google/protobuf/struct.proto` 来自本机 protoc 分发包，保留上游版权与许可证声明。[Protocol Buffers](https://github.com/protocolbuffers/protobuf)

这些文件用于 `make proto` 的 import 解析。
