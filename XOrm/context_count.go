// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
)

// Count 获取满足条件的数据模型数量。
// model 为要计数的数据模型，必须实现 IModel 接口。
// cond 为可选的查询条件列表，用于筛选要计数的数据。
//
// 计数过程首先验证模型是否已注册，然后按照优先级获取计数。
// 当数据已被列举过时，会从会话内存中计数，过滤掉已标记删除的数据，并应用查询条件。
// 如果数据已在全局内存中列举过或模型仅支持缓存模式，则从全局内存中计数，同样过滤掉已标记删除的数据并应用查询条件。
// 如果以上条件都不满足，则直接从远端数据源获取计数。
//
// 函数返回满足条件的数据模型数量，如果模型未注册，则返回 0。
// 计数会自动排除已标记删除的数据。对于仅缓存模式的模型，只在内存中计数。
// 计数结果会受到数据同步状态的影响。
func Count[T IModel](model T, cond ...*Condition) int {
	cacheDumpWait.Wait()

	gid := goid.Get()
	meta := getModelMeta(model)
	if meta == nil {
		XLog.Critical("XOrm.Count: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return 0
	}
	ret := 0
	if isSessionListed(gid, model) { // 会话内存读取
		var lret int64 = 0
		scache := getSessionCache(gid, model)
		if scache != nil {
			concurrentRange(scache, func(index int, key, value any) bool {
				sobj := value.(*sessionObject)
				if !sobj.ptr.IsValid() { // 忽略无效数据
				} else if sobj.ptr.Matchs(cond...) {
					atomic.AddInt64(&lret, 1)
				}
				return true
			})
		}
		ret = int(lret)
	} else if isGlobalListed(model) { // 全局内存读取
		var lret int64 = 0
		gcache := getGlobalCache(model)
		if gcache != nil {
			concurrentRange(gcache, func(index int, key, value any) bool {
				gobj := value.(IModel)
				if !gobj.IsValid() {
					// 已经被标记删除，则不读取
				} else if gobj.Matchs(cond...) {
					atomic.AddInt64(&lret, 1)
				}
				return true
			})
		}
		ret = int(lret)
	} else { // 远端读取
		ret = model.Count(cond...)
	}
	return ret
}
