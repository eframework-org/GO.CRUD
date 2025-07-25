# 更新记录

## [0.1.1] - 
### 变更
- 新增 XOrm/model_base.go 模块 handleInOperator 函数对单个参数的兼容
- 新增 XOrm/model_base.go 模块 getFieldValue 函数对字段名大小写的兼容

## [0.1.0] - 2025-07-08
### 修复
- 修复 XOrm 模块的会话对象 sessionObjectPool 回收异常

## [0.0.9] - 2025-07-05
### 修复
- 修复 XOrm/model_base.go 模块 Max 和 Min 函数处理空表产生的异常问题

## [0.0.8] - 2025-07-05
### 变更
- 新增 XOrm/orm_init.go 模块的 Source 配置字段根据环境变量求值的功能
- 新增 XOrm/model_base.go 模块的 OnQuery 接口用于实现自定义的查询逻辑
- 优化 XOrm/context_list.go 模块查询远端大体量数据的处理性能
- 更新依赖库版本

### 修复
- 修复 XOrm/model_base.go 模块 toInt64 函数的类型断言问题
- 修复 XOrm 模块 setGlobalCache/setSessionCache 不适合的堆栈输出问题

## [0.0.7] - 2025-06-19
### 修复
- 修复 XOrm 使用 IN 操作大量数据的卡顿问题（无状态遍历 -> 上下文缓存）

## [0.0.6] - 2025-06-16
### 变更
- 优化 XOrm 模块 context_commit.go 的首选项配置，支持禁用提交队列及配置项的校验

### 修复
- 修复 XOrm 模块 context.go -> Defer 调用时 XCollect.Map 的并发读写问题

## [0.0.5] - 2025-05-24
### 变更
- 移除 XOrm 模块 context_cache.go 的缓存实现，使用 XCollect.Map 替代之，提高大数据的遍历效率
- 更新依赖库版本

### 新增
- 新增 XOrm.List（缓存查找） 和 XOrm.Model.List（回源查找）的基准测试

## [0.0.4] - 2025-05-22
### 修复
- 修复 XOrm 模型注册时元数据存储错误的问题

## [0.0.3] - 2025-05-19
### 变更
- 调整 XOrm 模块数据源配置键前缀为 Orm/Source
- 调整 XOrm 模块提交队列容量的配置键为 Orm/Commit/Queue/Capacity
- 优化 XOrm 模块 model_base.go 的日志输出，读取操作（Read/List/..）若发生错误，使用 Warn 级别输出，更新操作（Write/Delete/..）若发生错误，使用 Error 级别输出

## [0.0.2] - 2025-05-06
### 变更
- 优化 XOrm 模块 context_cache.go 的缓存数据读取效率（数据分治 & 并发处理）
- 优化 XOrm 模块 globalObject、sessionObject 的高频分配（allocate）缓存，移除了 globalObject 结构体
- 重构 XOrm 模块 Dump 函数的名称为 Print（输出缓存的文本），重构 Dump 函数的实现，用于清除数据模型的缓存数据
- 重构 XOrm 模块 context_index.go 为 context_incre.go，明确了该函数职责
- 重构 XOrm 模块 model_info.go 为 model_meta.go，拓展了 beegoModelInfo 结构体的参数（cache、writable）
- 重构 XOrm 模块 Register 函数为 Meta，移除了 persist 参数，明确了该函数的职责
- 移除 XOrm 模块 Cleanup 函数，模型的注册/注销使用 beego/orm 的 RegisterModel/ResetModelCache 函数，不再进行拓展
- 重构 XOrm 模块 condition 结构体为 Condition，公开了该接口以供业务层使用
- 重构 XOrm 模块 Condition 函数为 Cond，优化了该函数的实现
- 重构 XOrm 模块 contex_commit.go 模块，优化了提交的性能，简化了事务监控的实现
- 移除 XOrm 模块 context_count.go 模块，可以使用 List 函数替代之
- 重构 XOrm 模块所有的单元测试，提高了测试覆盖率及代码质量，新增了若干并发测试

### 修复
- 修复 XOrm 模块在高并发环境下潜在的数据读写错误

### 新增
- 新增 XOrm 模块 context_commit.go 的指标度量（Prometheus）
- 新增 XOrm 模块 Defer 函数的调试日志输出

## [0.0.1] - 2025-03-23
### 新增
- 首次发布
