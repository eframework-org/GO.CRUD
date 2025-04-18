// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"github.com/eframework-org/GO.UTIL/XLog"
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
	meta := getModelInfo(model)
	if meta == nil {
		XLog.Critical("XOrm.Clear: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return
	}
	marked := make(map[string]int)
	scache := getSessionCache(model)
	if scache != nil { // 标记会话内存清除
		scache.Range(func(key, value any) bool {
			if value.(*sessionObject).ptr.Matchs(cond...) {
				value.(*sessionObject).delete = true
				value.(*sessionObject).clear = true
				marked[key.(string)] = 1
			}
			return true
		})
	}

	if meta.Cache { // 标记全局内存清除
		gcache := getGlobalCache(model)
		if gcache != nil {
			gcache.Range(func(key, value any) bool {
				if value.(*globalObject).ptr.Matchs(cond...) {
					value.(*globalObject).delete = true
					if _, exist := marked[key.(string)]; !exist {
						nobj := value.(*globalObject).ptr.Clone()
						ret := setSessionCache(nobj, meta) // 监控内存
						ret.delete = true
						ret.clear = true
					}
				}
				return true
			})
		}
	}
}
