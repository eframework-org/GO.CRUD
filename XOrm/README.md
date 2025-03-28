# XOrm

[![Reference](https://pkg.go.dev/badge/github.com/eframework-org/GO.CRUD/XOrm.svg)](https://pkg.go.dev/github.com/eframework-org/GO.CRUD/XOrm)
[![Release](https://img.shields.io/github/v/tag/eframework-org/GO.CRUD)](https://github.com/eframework-org/GO.CRUD/tags)
[![Report](https://goreportcard.com/badge/github.com/eframework-org/GO.CRUD)](https://goreportcard.com/report/github.com/eframework-org/GO.CRUD)

XOrm 拓展了 Beego 的 ORM 功能，同时实现了基于上下文的缓存机制，提高了数据操作的效率。

## 功能特性

- 多源配置：通过解析资源首选项中的配置自动初始化数据库连接
- 数据模型：提供了面向对象的模型设计及常用的数据操作
- 上下文操作：基于上下文的缓存机制，支持事务和并发控制

## 使用手册

### 1. 多源配置

通过解析资源首选项中的配置自动初始化数据库连接。

配置项说明：
- 配置键名：`Orm/<数据库类型>/<数据库别名>`
  - 支持 MySQL、PostgreSQL、SQLite3 等（Beego ORM 支持的类型）
- 配置参数：
  - Addr：数据源地址
  - Pool：连接池大小
  - Conn：最大连接数

配置示例：
```json
{
    "Orm/MySQL/Main": {
        "Addr": "root:123456@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&loc=Local",
        "Pool": 1,
        "Conn": 1
    },
    "Orm/PostgreSQL/Log": {
        "Addr": "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
        "Pool": 2,
        "Conn": 10
    },
    "Orm/SQLite3/Type": {
        "Addr": "file:data.db?cache=shared&mode=rwc",
        "Pool": 1,
        "Conn": 1
    }
}
```

### 2. 数据模型

提供了面向对象的模型设计及常用的数据操作。

#### 2.1 模型定义
```go
// 定义用户模型
type User struct {
    Model[User]           // 继承基础模型
    ID        int        `orm:"column(id);pk"` // 主键，自增
    Name      string     `orm:"column(name)"` // 字符串字段
    Age       int        `orm:"column(age)"` // 整型字段
}

// 实现必要的接口方法
func (u *User) AliasName() string {
    return "mydb" // 数据库别名
}

func (u *User) TableName() string {
    return "user" // 数据库表名
}

// 创建模型实例的工厂方法
func NewUser() *User {
    return XObject.New[User]()
}
```

#### 2.2 模型接口

模型接口定义了以下核心方法：

1. 生命周期：
```go
Ctor(obj any)               // 构造初始化
OnEncode()                  // 编码前回调
OnDecode()                  // 解码后回调
```

2. 基础信息：
```go
AliasName() string          // 数据库别名
TableName() string          // 数据表名称
ModelUnique() string        // 模型唯一标识
DataUnique() string         // 数据唯一标识
DataValue(field string) any // 获取字段值
```

3. 数据操作：
```go
Count(cond ...*condition) int                  // 统计记录数
Max(column ...string) int                      // 获取最大值
Min(column ...string) int                      // 获取最小值
Delete() int                                   // 删除记录
Write() int                                    // 写入记录
Read(cond ...*condition) bool                  // 读取记录
List(rets any, cond ...*condition) int         // 查询列表
Clear(cond ...*condition) int                  // 清理记录
```

4. 工具方法：
```go
IsValid(value ...bool) bool // 检查/设置有效性
Clone() IModel             // 深度拷贝
Json() string              // JSON序列化
Equals(model IModel) bool  // 对象比较
Matchs(cond ...*condition) bool // 条件匹配
```

#### 2.3 模型注册

参数说明：
- cache：是否启用缓存，启用后支持会话缓存和全局缓存
- persist：是否持久化存储，启用后数据会保存到数据库
- writable：是否可写，启用后支持写入和删除操作

应用场景：

| 启用缓存 | 是否持久化 | 是否可写 | 应用场景 |
|---------|-----------|----------|---------|
| true    | true    | true     | 适用于需要频繁读取、持久化存储、可写且可控数量的模型，如用户信息、产品信息等。可以快速读取和更新数据。 |
| true    | true    | false    | 适用于需要频繁读取且持久化存储但不需要写入的模型，如只读配置等。可以快速读取数据。 |
| true    | false   | true     | -- |
| true    | false   | false    | 适用于需要频繁读取但不需要持久化和写入的模型，如临时计算结果、缓存查询等。适合快速读取。 |
| false   | true    | true     | 适用于需要持久化存储且可写的模型，但不需要频繁读取或者数据量不可控的场景，如日志记录等。       |
| false   | true    | false    | 适用于需要持久化存储但不需要频繁读取和写入的模型，如系统版本信息等。               |
| false   | false   | true     | -- |
| false   | false   | false    | -- |

注意：选择参数时除了考虑应用场景外，还需结合实际业务运行情况，如是否存在多个实例同时读写的情况。

示例代码：
```go
// 用户模型：频繁读取、持久化存储、可写且可控数量
// cache=true, persist=true, writable=true
XOrm.Register(NewUser(), true, true, true)

// 配置模型：频繁读取、持久化存储、只读
// cache=true, persist=true, writable=false
XOrm.Register(NewConfig(), true, true, false)

// 计算结果模型：频繁读取、不持久化、只读
// cache=true, persist=false, writable=false
XOrm.Register(NewResult(), true, false, false)

// 日志模型：不缓存、持久化存储、可写
// cache=false, persist=true, writable=true
XOrm.Register(NewLog(), false, true, true)

// 版本信息模型：不缓存、持久化存储、只读
// cache=false, persist=true, writable=false
XOrm.Register(NewVersion(), false, true, false)
```

#### 2.4 条件查询

支持多种查询方式和复杂的条件组合。

##### 2.4.1 创建条件

```go
// 1. 创建空条件
cond := XOrm.Condition()

// 2. 从现有条件创建
baseCond := orm.NewCondition()
cond := XOrm.Condition(baseCond)

// 3. 从表达式创建（推荐）
cond := XOrm.Condition("age > {0} && name == {1}", 18, "test")
```

##### 2.4.2 比较运算符

```go
// 大于/大于等于
cond := XOrm.Condition("age > {0}", 18)  // age__gt
cond := XOrm.Condition("age >= {0}", 18) // age__gte

// 小于/小于等于
cond := XOrm.Condition("age < {0}", 18)  // age__lt
cond := XOrm.Condition("age <= {0}", 18) // age__lte

// 等于/不等于
cond := XOrm.Condition("age == {0}", 18) // age__exact
cond := XOrm.Condition("age != {0}", 18) // age__ne

// 空值判断
cond := XOrm.Condition("age isnull {0}", true) // age__isnull
```

##### 2.4.3 字符串匹配

```go
// 包含
cond := XOrm.Condition("name contains {0}", "test") // name__contains

// 前缀匹配
cond := XOrm.Condition("name startswith {0}", "test") // name__startswith

// 后缀匹配
cond := XOrm.Condition("name endswith {0}", "test") // name__endswith
```

##### 2.4.4 逻辑组合

```go
// AND 组合
cond := XOrm.Condition("age > {0} && name == {1}", 18, "test")

// OR 组合
cond := XOrm.Condition("age < {0} || age > {1}", 18, 60)

// NOT 条件
cond := XOrm.Condition("!(age >= {0})", 30)

// 复杂组合（使用括号控制优先级）
cond := XOrm.Condition("(age >= {0} && age <= {1}) || name == {2}", 18, 30, "test")
cond := XOrm.Condition("((age > {0} && name contains {1}) || status == {2}) && active == {3}",
    18, "test", "active", true)
```

##### 2.4.5 分页查询

```go
// 限制返回数量
cond := XOrm.Condition("age > {0} limit {1}", 18, 10)

// 设置偏移量
cond := XOrm.Condition("age > {0} offset {1}", 18, 20)

// 组合使用
cond := XOrm.Condition("age > {0} limit {1} offset {2}", 18, 10, 20)
```

##### 2.4.6 使用示例

```go
// 1. 简单查询
user := NewUser()
cond := XOrm.Condition("age > {0}", 18)
if XOrm.Read(user, cond) {
    fmt.Printf("Found user: %v\n", user.Name)
}

// 2. 复杂条件查询
var users []*User
cond := XOrm.Condition("(age >= {0} && age <= {1}) || name contains {2}", 18, 30, "test")
count := XOrm.List(&users, cond)
fmt.Printf("Found %d users\n", count)

// 3. 分页查询
var users []*User
cond := XOrm.Condition("age > {0} limit {1} offset {2}", 18, 10, 20)
XOrm.List(&users, cond)

// 4. 统计查询
cond := XOrm.Condition("status == {0} && age > {1}", "active", 18)
count := XOrm.Count(NewUser(), cond)
```

注意事项：
1. 条件表达式中的参数使用 `{n}` 形式引用，n 从 0 开始
2. 参数数量必须与表达式中的占位符数量一致
3. 复杂条件建议使用括号明确优先级
4. 条件会被缓存以提高性能，相同的表达式只会解析一次
5. 支持所有 Beego ORM 的条件操作符

### 3. 上下文操作

基于上下文的缓存机制，支持事务和并发控制。

#### 3.1 基本操作

所有数据操作都需要在会话监听的上下文中进行，以确保缓存策略和事务控制的正确性：

```go
// 开启会话监听，获取会话ID
sid := XOrm.Watch()
defer XOrm.Defer() // 结束会话时，将提交缓存队列并清理会话内存

// 写入操作：写入数据到会话缓存和全局缓存
user := NewUser()
user.Name = "test"
user.Age = 18
XOrm.Write(user) // 设置 delete=false，create=true

// 读取操作：按优先级依次从会话缓存、全局缓存、远端数据库读取
user := NewUser()
user.ID = 1
if XOrm.Read(user) { // 精确查找，检查缓存标记
    fmt.Printf("User: %v\n", user.Name)
}

// 条件读取：支持模糊查找和条件匹配
cond := XOrm.Condition("age > {0}", 18)
if XOrm.Read(user, cond) { // 模糊查找，可能触发远端读取
    fmt.Printf("User: %v\n", user.Name)
}

// 删除操作：标记删除状态到缓存
XOrm.Delete(user) // 设置 delete=true

// 清理操作：批量标记删除状态
cond = XOrm.Condition("age < {0}", 18)
XOrm.Clear(user, cond) // 设置 delete=true, clear=true

// 列举操作：从缓存和远端组合数据
var users []*User
cond = XOrm.Condition("age > {0} && name like {1}", 18, "%test%")
XOrm.List(&users, cond) // 依次检查会话缓存、全局缓存、远端数据

// 统计操作：直接访问数据源
count := XOrm.Count(NewUser(), cond)
```

注意：
1. 所有操作必须在 `Watch()` 和 `Defer()` 之间进行
2. 写入操作会同时更新会话缓存和全局缓存
3. 读取操作遵循缓存优先级：会话缓存 > 全局缓存 > 远端数据
4. 删除和清理操作仅做标记，实际删除在会话提交时执行
5. 列举和统计等批量操作可能会同时访问缓存和远端数据

#### 3.2 运行机理
```mermaid
stateDiagram-v2
    direction LR
    [*] --> InitOrm: 初始化Orm
    InitOrm --> InitModel: 注册数据模型
    InitModel --> Ready: Run
    Ready --> Watch: 监听
    Watch --> DBTransaction: 开始会话监听
    DBTransaction --> Defer: 提交会话操作
    Ready --> Close: 退出信号

    state InitOrm {
        direction TB
        [*] --> RegisterDataBase: 解析配置项
    }
    state InitModel {
        direction TB
        [*] --> XOrm.Register()
    }
    state Watch {
        direction TB
        [*] --> XOrm.Watch()
        XOrm.Watch() --> contextMap: 按goroutine id记录上下文
    }
    state Defer {
        direction TB
        [*] --> XOrm.Defer()
        XOrm.Defer() --> 缓冲至队列: 对比会话内存
        缓冲至队列 --> 清除会话内存
        清除会话内存 --> [*]
    }
    state Close{
        direction TB
        [*] --> Flush(): 刷新所有处理队列
        Flush() --> [*]
    }
    state Flush(){
        direction TB
        [*] --> XOrm.Flush(): 逐个刷新当前队列
        XOrm.Flush() --> 关闭队列
    }
    缓冲至队列 --> 创建缓冲队列: 队列不存在
    创建缓冲队列 --> 处理队列数据
    处理队列数据 --> 关闭队列: 退出信号
```

#### 3.3 缓存策略
```mermaid
stateDiagram-v2
    direction TB
    state Context {
        [*] --> XOrm.Write(): 数据写入
        XOrm.Write() --> 设置全局缓存
        设置全局缓存 --> 设置会话缓存: set delete=false 设置模型有效
        设置会话缓存 --> [*]: set create=true delete|clear=false

        [*] --> XOrm.Read(): 数据读取
        XOrm.Read() --> 精确查找
        精确查找 --> 会话缓存读取
        会话缓存读取 --> 全局缓存读取: if delete|clear 设置模型失效 fi
        全局缓存读取 --> 远端数据读取: if delete 设置模型失效 else 设置全局缓存 fi
        远端数据读取 --> [*]: 设置全局缓存 设置会话缓存
        XOrm.Read() --> 模糊查找
        模糊查找 --> 仅缓存查找
        仅缓存查找 --> [*]: if 会话缓存读取 else 全局缓存读取 fi
        模糊查找 --> 判断列举读取会话缓存
        判断列举读取会话缓存 --> 判断列举读取全局缓存
        判断列举读取全局缓存 --> 远端条件读取
        远端条件读取 --> [*]

        [*] --> XOrm.List(): 数据列举
        XOrm.List() --> 列举读取会话缓存
        列举读取会话缓存 --> 列举读取全局缓存
        列举读取全局缓存 --> 远端列表读取
        远端列表读取 --> [*]

        [*] --> XOrm.Delete(): 数据删除
        XOrm.Delete() --> 标记全局缓存删除
        标记全局缓存删除 --> 标记会话缓存删除: set delete=true
        标记会话缓存删除 --> [*]: set delete=true

        [*] --> XOrm.Clear(): 数据清理
        XOrm.Clear() --> 标记会话缓存清除
        标记会话缓存清除 --> 标记全局缓存清除: set delete|clear=true
        标记全局缓存清除 --> [*]: set delete|clear=true

        [*] --> XOrm.Count(): 数据计数
        XOrm.Count() --> [*]

        [*] --> XOrm.Incre(): 索引自增
        XOrm.Incre() --> [*]

        [*] --> XOrm.Max(): 最大索引
        XOrm.Max() --> [*]
        
        [*] --> XOrm.Min(): 最小索引
        XOrm.Min() --> [*]
    }
```

## 常见问题

更多问题，请查阅[问题反馈](../CONTRIBUTING.md#问题反馈)。

## 项目信息

- [更新记录](../CHANGELOG.md)
- [贡献指南](../CONTRIBUTING.md)
- [许可证](../LICENSE)
