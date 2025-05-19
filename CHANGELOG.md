# 更新记录

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
