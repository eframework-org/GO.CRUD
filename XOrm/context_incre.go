// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
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
	cacheDumpWait.Wait()

	gid := goid.Get()
	ctx := getContext(gid)
	if ctx == nil {
		XLog.Critical("XOrm.Incre: context was not found: %v", XLog.Caller(1, false))
		return -1
	}
	meta := getModelMeta(model)
	if meta == nil {
		XLog.Critical("XOrm.Incre: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return -1
	}
	if !ctx.writable {
		XLog.Error("XOrm.Incre: context was not writable: %v", XLog.Caller(1, false))
		return -1
	}
	if !meta.writable {
		XLog.Error("XOrm.Incre: model of %v was not writable: %v", model.ModelUnique(), XLog.Caller(1, false))
		return -1
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
		if meta.fields.pk != nil {
			cname = meta.fields.pk.column
		}
	}
	if cname == "" {
		XLog.Error("XOrm.Incre: column was empty: %v", model.ModelUnique())
		return 0
	}

	increKey := fmt.Sprintf("%v_%v", model.ModelUnique(), cname)
	if val, ok := globalIncreMap.Load(increKey); !ok {
		defer globalIncreMutex.Unlock()
		globalIncreMutex.Lock()
		if nval, ok := globalIncreMap.Load(increKey); ok {
			return int(atomic.AddInt64(nval.(*int64), int64(delta)))
		} else {
			index := model.Max([]string{cname}...)
			index += delta
			lindex := int64(index)
			globalIncreMap.Store(increKey, &lindex)
			return index
		}
	} else {
		return int(atomic.AddInt64(val.(*int64), int64(delta)))
	}
}
