# 研究记录 — 商户 API_KEY 接口鉴权

**日期** 2026-11-17 | **规格** [spec.md](spec.md)

## 1. HTTP 认证及商户归属

**决策** 在独立的 `internal/middleware` 包提供 Auth 和 AdminAuth，由 router 的公开、商户、支付及管理分组选择挂载。中间件校验密钥及所属商户状态，从请求参数读取商户号并完成权限比对，通过后调用 `ctx.Next()`。商户号通过现有请求类型字段传递给业务处理。

**依据** 用户明确要求中间件完成权限校验后放行，商户号由参数传递。中间件通过 encoding/json 读取通用 merchant_account 字段，兼容 merchantAccount。其他业务字段作为原始 JSON 保留，由 controller 按各自接口契约绑定。解析后通过 `io.NopCloser(bytes.NewReader(body))` 恢复原始请求体，供 controller 再次读取。

**评估方案** 中间件读取通用商户参数、controller 绑定后授权、service 执行归属检查。采用中间件完成权限检查及请求体恢复的方案。

## 2. 首把密钥及签发接口

**决策** 使用 `auth.admin_key` 作为平台管理凭证，通过相同的 Bearer 请求头调用 `POST /v1/auth/api-keys`。普通有效商户密钥访问此接口返回 403，未知凭证返回 401，未配置管理凭证返回 503。

**依据** 现有微信登录只返回 openid，缺少管理员身份体系。独立管理凭证可直接签发指定商户的首把密钥，符合规格默认假设。管理请求首先比较当前配置凭证，使用固定长度 SHA-256 摘要进行 `subtle.ConstantTimeCompare`。比对失败后查询商户密钥摘要，以区分 403 与 401，查询故障返回 503。[Go subtle](https://pkg.go.dev/crypto/subtle)

**评估方案** 独立管理凭证、已有商户密钥签发、部署网络访问控制。采用规格默认的独立管理凭证。

## 3. 随机密钥及保存形式

**决策** 使用 32 字节安全随机数据，经 `base64.RawURLEncoding` 形成 43 字符密钥。新表保存完整密钥的 SHA-256 十六进制摘要、商户 ID、签发时间，摘要建立唯一索引。

**依据** 标准库已提供安全随机源和摘要能力，项目当前 Go 1.26.0 支持这些接口。当前 GORM logger 会记录 SQL 参数，repo 接收摘要可满足密钥明文交付与日志约束。[Go rand](https://pkg.go.dev/crypto/rand@go1.26.0)、[Go sha256](https://pkg.go.dev/crypto/sha256)

**评估方案** 摘要查询、明文保存、可逆加密、携带商户声明的自包含凭证。采用摘要查询。

## 4. 主库读取及事务

**决策** 使用 `Clauses(dbresolver.Write)` 查询密钥和商户实时状态。签发事务先锁定商户、验证状态，再插入摘要，提交成功后返回原文。

**依据** `internal/pkg/gormhelper/pool.go` 已启用读写分离。商户缓存每 3 分钟刷新。主库查询能够在多个实例间立即识别新密钥及最新商户状态，GORM 支持在事务开始前选择主库连接。[GORM DBResolver](https://gorm.io/docs/dbresolver.html)

**评估方案** 实时主库、读副本、现有商户缓存、额外密钥缓存。采用实时主库。

## 5. 商户归属及历史数据

**决策** 密钥保存 `pay_merchant.id`，校验时读取当前账号和状态。签发发现同账号多条商户记录时返回统一的签发失败。正常业务数据以商户账号唯一为前提，联调前核对所用商户账号。

**依据** 现有商户表的账号字段没有唯一索引，但业务按账号选取商户。主键关联能准确区分删除后重建的商户。账号歧义作为签发冲突处理，可避免把新密钥绑定到任意一条历史记录。

**评估方案** 商户 ID 关联、账号冗余关联、为历史商户表增加唯一约束。采用 ID 关联及签发冲突检测。

## 6. 配置与日志

**决策** Auth 配置沿用 YAML 和 `config.Get()`，管理请求每次读取当前快照。空管理密钥表示签发暂不可用，非空凭证的格式在发布新快照之前校验。错误消息只描述字段。Gin 访问日志启用 `SkipQueryString`。

**依据** `config.reload` 在校验及 apply 成功后发布新配置，现有流程可继续处理热更新失败。Gin 1.12.0 已提供查询串日志开关，源码中的 recovery 也会遮蔽 Authorization 请求头。[Gin LoggerConfig](https://pkg.go.dev/github.com/gin-gonic/gin@v1.12.0#LoggerConfig)

**评估方案** 现有配置快照、独立环境变量读取、额外管理组件。采用现有配置快照。

## 7. Protobuf 契约及服务注册

**决策** 按用户补充要求，在现有 `api/auth/v1/auth.proto` 的 `AuthService` 增加 `GenerateAPIKey`，请求和响应分别命名为 `GenerateAPIKeyReq`、`GenerateAPIKeyResp`，添加 HTTP 注解并手动注册 HTTP 路由。复用 `service.Auth` 实现签发业务，中间件负责身份和权限校验，实施阶段通过 `make proto` 更新生成代码。

**依据** 用户明确要求复用现有 proto service。当前 `AuthService` 已注册 gRPC，签发方法会通过现有注册同时对外提供。HTTP 的 AdminAuth 直接校验 Authorization 请求头，RPC 入口针对签发方法校验 incoming metadata。二者调用同一 middleware.AuthorizeAdmin(ctx, values)，通过后进入 Auth.GenerateAPIKey 处理业务，沿用规格 FR-2 的签发规则。

**评估方案** 扩展现有 AuthService、只增加消息定义。采用现有 AuthService 中增加方法和消息的方案。

## 研究结论

全部技术决策已确定。设计依赖商户账号唯一的现有业务前提，并记录了账号重复时的明确签发错误。默认签发授权及密钥有效期见规格假设章节。
