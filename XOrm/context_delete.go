// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
)

// Delete 标记数据模型为删除状态。
// model 为要删除的数据模型，必须实现 IModel 接口。
//
// 删除操作首先验证模型是否已注册。如果启用了缓存，会在全局内存中查找对应的对象，
// 如果存在，则设置其删除标记为 true。然后在会话内存中创建或获取会话对象，
// 并设置删除标记为 true。
//
// 删除操作是软删除，不会立即从内存中移除数据。被标记删除的数据在读取时会被忽略。
func Delete[T IModel](model T) {
	gid := goid.Get()
	meta := getModelInfo(model)
	if meta == nil {
		XLog.Critical("XOrm.Delete: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return
	}
	if meta.Cache { // 标记全局内存删除
		gcache := getGlobalCache(model)
		if gcache != nil {
			gobj, exist := gcache.Load(model.DataUnique())
			if exist {
				gobj.(*globalObject).delete = true
			}
		}
	}

	setSessionCache(gid, model, meta).delete = true
}
