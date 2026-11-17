# 数据模型 — 商户 API_KEY

**规格** [spec.md](spec.md) | **契约** [contracts/http-api.md](contracts/http-api.md)

## 商户

复用 `models.PayMerchant` 和 `pay_merchant` 表。

| 字段      | 类型                    | 规则                      |
|---------|-----------------------|-------------------------|
| id      | int64 / BIGINT        | 商户主键，密钥绑定此 ID           |
| account | string / VARCHAR(128) | 外部商户账号，精确匹配，沿用原有长度与排序规则 |
| status  | int64 / BIGINT        | 1 正常，2 禁用               |

商户账号唯一为现有业务前提。签发查询通过 BINARY account 精确匹配，最多读取两条匹配记录，仅在匹配一条且状态正常时签发，失败统一返回 503。联调及上线数据检查需识别重复账号。

## 商户密钥

新增 `models.PayMerchantAPIKey`，表名 `pay_merchant_api_key`。

| 字段         | Go 类型     | MySQL 类型                                                | 规则                        |
|------------|-----------|---------------------------------------------------------|---------------------------|
| ID         | int64     | BIGINT                                                  | 自增主键                      |
| MerchantID | int64     | BIGINT NOT NULL                                         | 关联现有商户 ID，普通索引            |
| KeyHash    | string    | CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL | 完整密钥的 SHA-256 十六进制摘要，唯一索引 |
| CreateTime | time.Time | DATETIME(3) NOT NULL                                    | 创建时间，GORM autoCreateTime  |

模型字段使用明确 gorm 标签并提供 `TableName()`。初始化 SQL 与增量迁移保持相同字段、索引及默认值。关系校验由主库查询和事务完成，沿用当前表间关系的处理方式。

原文在签发 service 内生成，提交成功后进入响应。repo 的输入、模型及日志使用摘要。一个商户对应多条密钥记录，密钥长期有效。

## repo 请求结构

- `MySQLAPIKey` 持有私有 `db *gorm.DB`，由 `NewMySQLAPIKey(db)` 构造。
- `ReqAPIKey` 持有私有 `keyHash`，用于按摘要读取密钥及当前商户身份，也支持管理认证辨识已有商户密钥。
- `ReqCreateAPIKey` 持有私有 `account`、`keyHash`，用于签发事务。
- 查询与签发请求均采用 `NewReqXxx(opts ...func(*ReqXxx)) ReqXxx`，通过单字段 `WithReqXxx...` 选项构造。
- repo 方法使用 `context.Context` 并返回 error。签发事务返回绑定商户，供 service 构造响应。记录未找到、商户禁用、账号歧义、摘要冲突及连接错误保留可识别的错误类别。

## 身份与状态

| 当前状态或操作         | 结果                    |
|-----------------|-----------------------|
| 管理认证成功，正常商户首次签发 | 创建一条摘要记录，返回密钥         |
| 同一商户再次签发或并发签发   | 每个成功请求各创建一条独立记录       |
| 摘要碰撞            | 唯一索引拒绝写入，返回签发失败，可重新签发 |
| 商户正常且摘要匹配       | 返回可信商户账号，HTTP 身份校验成功  |
| 商户禁用或已删除        | 业务认证返回 401            |
| 商户删除后以同账号重建     | 旧密钥仍关联原 ID，业务认证返回 401 |
| 商户恢复正常          | 对应原 ID 的既有密钥可继续使用     |
| 数据库操作失败         | 终止当前操作，按错误类别返回结果      |

签发事务在主库内锁定目标商户后检查状态并插入密钥摘要，提交成功后响应。并发禁用操作与商户行锁串行执行，已开始的业务请求按其认证时读取的状态处理。

事务确定回滚时没有新增有效记录。提交结果不确定或响应丢失时可能存在未交付的摘要记录，调用方重新签发即可，成功签发产生的新密钥可独立使用。

## 请求中的商户账号

商户账号使用现有下单请求中的 `merchant_account` 字段。支付分组启用商户参数校验，中间件读取通用 JSON 商户参数，将请求商户账号与密钥所属商户账号精确比对。请求指定账号时，还需核对现有商户缓存的 ID 与主库认证 ID 一致，错配返回 503，等待缓存刷新。权限通过后，商户账号随原有请求参数传给 controller 和 service。

中间件读取请求体后恢复原始字节，再放行给 controller。通用商户参数兼容 merchant_account 与 merchantAccount 两种命名，controller 按原有业务契约绑定完整请求。微信登录请求按其原有契约传递参数，中间件校验其密钥及所属商户状态。

## 配置实体

新增 `config.Auth`，包含 `AdminKey string`，由 `Config.Auth` 通过 YAML `auth.admin_key` 加载。非空值必须为单个不含空白及逗号的凭证。空值允许启动，签发接口返回 503。热更新格式错误时保留旧快照，凭证变化成功发布后由下一次管理请求读取。
