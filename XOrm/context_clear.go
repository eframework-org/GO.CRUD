// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
)

// Clear 根据条件批量标记数据模型为清理状态。
// model 为要清理的数据模型，必须实现 IModel 接口。
// cond 为可选的查询条件列表，用于匹配要清理的数据。
//
// 清理操作首先验证模型是否已注册，然后创建标记映射用于跟踪已处理的对象。
// 在会话内存清理阶段，遍历会话内存中的对象，对匹配条件的对象设置删除标记和清理标记，并记录到标记映射中。
// 如果启用了缓存，在全局内存清理阶段，遍历全局内存中的对象，对匹配条件的对象设置删除标记。
// 对于未在会话内存中处理过的对象，会克隆到会话内存并设置相应的删除和清理标记。
//
// 清理操作是软删除，不会立即从内存中移除数据。被标记清理的数据在读取时会被忽略。
func Clear[T IModel](model T, cond ...*condition) {
	gid := goid.Get()
	meta := getModelInfo(model)
	if meta == nil {
		XLog.Critical("XOrm.Clear: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return
	}

	marked := syncMapPool.Get().(*sync.Map)
	scache := getSessionCache(gid, model)
	if scache != nil { // 标记会话内存清除
		concurrentRange(scache, func(index int, key, value any) bool {
			if value.(*sessionObject).ptr.Matchs(cond...) {
				value.(*sessionObject).delete = true
				value.(*sessionObject).clear = true
				marked.Store(key, 1)
			}
			return true
		})
		isSessionListed(gid, model, false, true, cond...) // 清除会话列举状态
	}

	if meta.Cache { // 标记全局内存清除
		gcache := getGlobalCache(model)
		if gcache != nil {
			concurrentRange(gcache, func(index int, key, value any) bool {
				if value.(*globalObject).ptr.Matchs(cond...) {
					value.(*globalObject).delete = true
					if _, loaded := marked.Load(key); !loaded {
						nobj := value.(*globalObject).ptr.Clone()
						ret := setSessionCache(gid, nobj, meta) // 监控会话内存
						ret.delete = true
						ret.clear = true
					}
				}
				return true
			})
			isGlobalListed(model, meta, false, true, cond...) // 清除全局列举状态
		}
	}

	marked.Clear()
	syncMapPool.Put(marked)
}
