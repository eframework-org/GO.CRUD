// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"reflect"
	"sync"
	"unsafe"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XLog"
)

//go:linkname defaultModelCache github.com/beego/beego/v2/client/orm.defaultModelCache
var defaultModelCache *any

// beegoModelCache 是 beego/orm 的模型缓存实例。
var beegoModelCache *beegoModelMap = (*beegoModelMap)(unsafe.Pointer(reflect.ValueOf(defaultModelCache).UnsafePointer()))

// beegoModelMap 定义了 beego/orm 的内部模型映射结构。
type beegoModelMap struct {
	Orders          []string                   // 模型注册顺序
	Cache           map[string]*beegoModelInfo // 按表名索引的模型信息
	CacheByFullName map[string]*beegoModelInfo // 按完整名称索引的模型信息
	BootstrapOnce   *sync.Once                 // 初始化同步锁
}

// beegoModelInfo 定义了 beego/orm 的内部模型信息结构。
type beegoModelInfo struct {
	Manual    bool           // 是否手动注册
	IsThrough bool           // 是否为中间表
	Pkg       string         // 包名
	Name      string         // 模型名称
	FullName  string         // 完整名称
	Table     string         // 表名
	Model     any            // 模型实例
	Fields    *beegoFieldMap // 字段映射
	AddrField reflect.Value  // 模型地址
	Uniques   []string       // 唯一索引
}

// beegoFieldMap 定义了 beego/orm 的内部字段映射结构。
type beegoFieldMap struct {
	Pk            *beegoFieldInfo            // 主键字段
	Columns       map[string]*beegoFieldInfo // 按列名索引的字段
	Fields        map[string]*beegoFieldInfo // 按字段名索引的字段
	FieldsLow     map[string]*beegoFieldInfo // 按小写字段名索引的字段
	FieldsByType  map[int][]*beegoFieldInfo  // 按类型索引的字段
	FieldsRel     []*beegoFieldInfo          // 关联字段
	FieldsReverse []*beegoFieldInfo          // 反向关联字段
	FieldsDB      []*beegoFieldInfo          // 数据库字段
	Rels          []*beegoFieldInfo          // 所有关联字段
	Orders        []string                   // 字段顺序
	DbCols        []string                   // 数据库列名
}

// beegoFieldInfo 定义了 beego/orm 的内部字段信息结构。
type beegoFieldInfo struct {
	DbCol               bool                // 是否为数据库列（外键和一对一关系）
	InModel             bool                // 是否在模型中
	Auto                bool                // 是否自增
	Pk                  bool                // 是否为主键
	Null                bool                // 是否可为空
	Index               bool                // 是否有索引
	Unique              bool                // 是否唯一
	ColDefault          bool                // 是否有默认值标签
	ToText              bool                // 是否转换为文本
	AutoNow             bool                // 是否自动更新时间
	AutoNowAdd          bool                // 是否自动添加时间
	Rel                 bool                // 是否为关联字段（外键、一对一、多对多）
	Reverse             bool                // 是否为反向关联
	IsFielder           bool                // 是否实现 Fielder 接口
	Mi                  *beegoModelInfo     // 所属模型信息
	FieldIndex          []int               // 字段索引
	FieldType           int                 // 字段类型
	Name                string              // 字段名
	FullName            string              // 完整名称
	Column              string              // 列名
	AddrValue           reflect.Value       // 字段地址
	Sf                  reflect.StructField // 结构体字段
	Initial             string              // 默认值
	Size                int                 // 字段大小
	ReverseField        string              // 反向关联字段名
	ReverseFieldInfo    *beegoFieldInfo     // 反向关联字段信息
	ReverseFieldInfoTwo *beegoFieldInfo     // 双向关联字段信息
	ReverseFieldInfoM2M *beegoFieldInfo     // 多对多关联字段信息
	RelTable            string              // 关联表名
	RelThrough          string              // 中间表名
	RelThroughModelInfo *beegoModelInfo     // 中间表模型信息
	RelModelInfo        *beegoModelInfo     // 关联模型信息
	Digits              int                 // 数字位数
	Decimals            int                 // 小数位数
	OnDelete            string              // 删除时操作
	Description         string              // 字段描述
	TimePrecision       *int                // 时间精度
	DbType              string              // 数据库类型
}

// modelInfo 定义了模型的扩展信息。
type modelInfo struct {
	*beegoModelInfo      // 继承 beego/orm 的模型信息
	Cache           bool // 是否启用缓存
	Persist         bool // 是否持久化
	Writable        bool // 是否可写
}

// modelCache 存储所有已注册模型的信息。
var modelCache map[string]*modelInfo = make(map[string]*modelInfo)

// modelCacheMu 用于保护模型缓存的互斥锁。
var modelCacheMu sync.Mutex

// getModelInfo 获取指定模型的信息。
// model 为模型实例。
// 返回模型信息，如果模型未注册则返回 nil。
func getModelInfo(model IModel) *modelInfo {
	if model != nil {
		return modelCache[model.ModelUnique()]
	}
	return nil
}

// Register 注册一个模型。
// model 为模型实例。
// cache 指定是否启用缓存。
// persist 指定是否持久化。
// writable 指定是否可写。
// 如果模型为 nil 或已注册，将触发 panic。
func Register(model IModel, cache bool, persist bool, writable bool) {
	if model == nil {
		XLog.Panic("XOrm.Register: nil model instance.")
		return
	}

	modelCacheMu.Lock()
	defer modelCacheMu.Unlock()

	id := model.ModelUnique()
	if _, ok := modelCache[id]; ok {
		XLog.Panic("XOrm.Register: dumplicated model of %v.", id)
		return
	}
	orm.RegisterModel(model)
	meta := &modelInfo{beegoModelInfo: beegoModelCache.Cache[model.TableName()], Cache: cache, Persist: persist, Writable: writable}
	modelCache[id] = meta
}

// Cleanup 重置模型缓存。
// 此操作会清空所有已注册的模型信息。
func Cleanup() {
	modelCacheMu.Lock()
	defer modelCacheMu.Unlock()

	modelCache = make(map[string]*modelInfo)
	orm.ResetModelCache()
}
