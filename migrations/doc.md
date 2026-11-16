1. 数据模型字段添加明确的 gorm 标签，表模型提供 TableName() 方法。
2. 数据库访问使用持有 db *gorm.DB 的 MySQLXxx 对象，通过 NewMySQLXxx(db) 构造。
3. 查询条件使用 ReqXxx 结构体，字段保持私有，通过 Functional Options 构造：
    - NewReqXxx(opts ...func(*ReqXxx)) ReqXxx
    - WithReqXxxBrandCode(brandCode string) func(*ReqXxx)
    - WithReqXxxPage(page, size int32) func(*ReqXxx)
4. 少定义辅助函数，不机械拆分 normalizePage、encodeCursor、decodeCursor、postDTO 等函数。
5. 实现代码之前先设计，遵循设计模式，避免写一个函数写大量代码，可以抽象接口、面向结构体编程、但是要避免过度设计
6. 减少不必要的参数校验，没有必要的校验可以不用写，代码命名要简短可以表达语意