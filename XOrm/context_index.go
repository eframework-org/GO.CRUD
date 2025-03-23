// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"

	"github.com/eframework-org/GO.UTIL/XLog"
)

var (
	// globalMaxMap 存储了所有模型的最大值缓存。第一层键为模型唯一标识，值为该模型的列最大值映射；
	// 第二层键为列名，值为该列的当前最大值。
	globalMaxMap sync.Map // map[string]map[string]int
)

// Incre 获取并自增指定列的最大值。model 参数为要操作的数据模型，必须实现 IModel 接口。
// columnAndDelta 为可变参数，支持多种组合：无参数时自增主键且增量为 1；一个参数时，若为字符串则
// 指定列名且增量为 1，若为整数则使用主键并指定增量；两个参数时，第一个为列名（字符串），第二个为增量（整数）。
//
// 函数首先获取或创建模型的最大值缓存，然后解析参数确定目标列名和增量值。如果未指定列名，会尝试使用主键列，
// 若无主键则报错。获取当前最大值时优先使用缓存的值，如果缓存不存在，则从数据源获取，最后计算并缓存新值。
//
// 函数返回自增后的新值，如果列名为空，则返回 0。目标列必须是整数类型，建议在事务中使用以确保一致性。
// 需要注意的是，缓存的最大值在程序重启后会重置。
func Incre(model IModel, columnAndDelta ...any) int {
	var gcache *sync.Map
	val, _ := globalMaxMap.Load(model.ModelUnique())
	if val == nil {
		gcache = &sync.Map{}
		globalMaxMap.Store(model.ModelUnique(), gcache)
	} else {
		gcache = val.(*sync.Map)
	}

	delta := 1
	cname := ""
	if len(columnAndDelta) == 1 {
		switch columnAndDelta[0].(type) {
		case string:
			cname = columnAndDelta[0].(string)
		case int:
			delta = columnAndDelta[0].(int)
		}
	} else if len(columnAndDelta) == 2 {
		switch columnAndDelta[0].(type) {
		case string:
			cname = columnAndDelta[0].(string)
		}
		switch columnAndDelta[1].(type) {
		case int:
			delta = columnAndDelta[1].(int)
		}
	}
	if cname == "" {
		meta := getModelInfo(model)
		if meta != nil && meta.Fields.Pk != nil {
			cname = meta.Fields.Pk.Column
		}
	}
	if cname == "" {
		XLog.Error("XOrm.Model.Incre(%v): column was empty.", model.ModelUnique())
		return 0
	}

	index, exist := gcache.Load(cname)
	if !exist {
		index = model.Max([]string{cname}...)
	}
	nindex := index.(int) + delta
	gcache.Store(cname, nindex)
	return nindex
}

// Max 获取指定列的最大值。model 参数为要查询的数据模型，必须实现 IModel 接口。
// column 参数为可选的列名列表，用于指定要查询的列。函数返回指定列的最大值，
// 如果未找到或发生错误，返回值取决于具体实现。
func Max(model IModel, column ...string) int {
	index := model.Max(column...)
	return index
}

// Min 获取指定列的最小值。model 参数为要查询的数据模型，必须实现 IModel 接口。
// column 参数为可选的列名列表，用于指定要查询的列。函数返回指定列的最小值，
// 如果未找到或发生错误，返回值取决于具体实现。
func Min(model IModel, column ...string) int {
	index := model.Min(column...)
	return index
}
