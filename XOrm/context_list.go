// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"github.com/eframework-org/GO.UTIL/XCollect"
	"github.com/eframework-org/GO.UTIL/XLog"
)

// List 获取数据模型的列表。model 参数为要查询的数据模型，必须实现 IModel 接口。
// writableAndCond 为可变参数，可包含布尔值（表示是否可写）和查询条件对象（*Condition 类型）。
//
// 函数首先验证模型是否已注册，然后解析参数获取可写标记和查询条件。查询数据时按照优先级依次从
// 会话内存、全局内存和远端数据源获取。会话内存查询会过滤掉已标记删除的数据并应用查询条件；
// 全局内存查询会克隆数据到会话内存并处理覆盖数据；远端数据源查询会将数据同步到全局内存（如果启用缓存）
// 和会话内存，并处理删除标记以确保数据一致性。
//
// 对于远端查询结果，函数会检查并使用会话内存和全局内存中的最新数据，移除被标记删除的数据，
// 并添加仅在会话内存或全局内存中的匹配数据作为补充同步。函数返回满足条件的数据模型切片，
// 已被标记删除的数据将被过滤。返回的数据是原始数据的克隆，非条件列举可能导致数据不同步，
// 建议避免在异步操作期间进行条件列举。
func List[T IModel](model T, writableAndCond ...any) []T {
	frets := make([]T, 0)
	meta := getModelInfo(model)
	if meta == nil {
		XLog.Critical("XOrm.List: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
	} else {
		writable := meta.Writable
		var cond *condition
		for _, v := range writableAndCond {
			switch nv := v.(type) {
			case bool:
				writable = nv
			case *condition:
				cond = nv
			default:
				XLog.Critical("XOrm.List: writableAndCond of %v type is error: %v", v, XLog.Caller(1, false))
			}
		}
		if isSessionListed(model, true, false, cond) { // 会话内存读取
			scache := getSessionCache(model)
			if scache != nil {
				scache.Range(func(key, value any) bool {
					sobj := value.(*sessionObject)
					if sobj.clear || sobj.delete {
						// 已经被标记删除，则不读取
					} else if sobj.ptr.Matchs(cond) {
						sobj.setWritable(writable)
						frets = append(frets, sobj.ptr.(T))
					}
					return true
				})
			}
		} else if isGlobalListed(model, meta, true, false, cond) || (meta.Cache && !meta.Persist) { // 全局内存读取（若仅支持缓存，则只在内存中查找）
			gcache := getGlobalCache(model)
			scache := getSessionCache(model)
			if gcache != nil {
				gcache.Range(func(key, value any) bool {
					gobj := value.(*globalObject)
					if gobj.delete {
						// 已经被标记删除，则不读取
					} else if gobj.ptr.Matchs(cond) {
						ele := gobj.ptr.Clone() // 内存拷贝
						var sobj *sessionObject
						if scache != nil { // 会话内存读取
							t, _ := scache.Load(ele.DataUnique())
							if t != nil {
								sobj = t.(*sessionObject)
							}
						}
						if sobj != nil { // 使用会话内存数据替换之
							// 这里无需判断SClear和SDelete，因为数据和全局内存是同步的
							ele = sobj.ptr
						} else {
							sobj = setSessionCache(ele, meta) // 监控内存
						}
						sobj.setWritable(writable)
						frets = append(frets, ele.(T))
					}
					return true
				})
			}
		} else { // 远端读取
			globalWait("XOrm.List", model, meta)
			model.List(&frets, cond)
			gcache := getGlobalCache(model)
			scache := getSessionCache(model)
			valids := make([]string, 0)
			invalids := make(map[int]struct{})
			for i := range frets {
				removed := false
				obj := frets[i]
				name := obj.DataUnique()
				var gobj *globalObject
				var sobj *sessionObject
				if meta.Cache {
					if gcache != nil { // 全局内存读取
						t, _ := gcache.Load(name)
						if t != nil {
							gobj = t.(*globalObject)
						}
					}
				}
				if scache != nil { // 会话内存读取
					t, _ := scache.Load(name)
					if t != nil {
						sobj = t.(*sessionObject)
					}
				}
				isSCache := false
				if sobj != nil { // 使用会话内存数据替换之
					if sobj.clear || sobj.delete {
						// 已经被标记删除，则不读取
						removed = true
						invalids[i] = struct{}{}
						XLog.Notice("XOrm.List: del sobj: %v", name)
					} else {
						frets[i] = sobj.ptr.(T)
						isSCache = true
						XLog.Notice("XOrm.List: use sobj: %v", name)
					}
				} else if gobj != nil && !isSCache { // 已经在全局内存中，但不在会话内存中
					if gobj.delete {
						// 已经被标记删除，则不读取
						removed = true
						invalids[i] = struct{}{}
						XLog.Notice("XOrm.List: del gobj: %v", name)
					} else {
						nobj := gobj.ptr.Clone() // 内存拷贝
						frets[i] = nobj.(T)
						sobj := setSessionCache(nobj, meta) // 监控内存
						sobj.setWritable(writable)
						XLog.Notice("XOrm.List: use gobj: %v", name)
					}
				} else { // 既不在会话内存中，也不在全局内存中
					if meta.Cache {
						setGlobalCache(obj.Clone()) // 内存拷贝
					}
					sobj := setSessionCache(obj, meta) // 监控内存
					sobj.setWritable(writable)
				}
				if !removed {
					valids = append(valids, name)
				}
			}

			// 移除被标记为删除的
			if len(invalids) > 0 {
				nrets := make([]T, 0)
				for i, o := range frets {
					if _, ok := invalids[i]; !ok {
						nrets = append(nrets, o)
					}
				}
				frets = nrets
			}

			// 修复数据不同步20210508
			// 若数据未被全量列举（非条件列举），则对该类型数据进行条件列举可能会导致数据不同步（远端读取，但数据写入/删除是异步的）
			// 还是要尽量避免这样使用
			if scache != nil { // 遍历会话内存
				scache.Range(func(key, value any) bool {
					sobj := value.(*sessionObject)
					if sobj.clear || sobj.delete {
					} else if !XCollect.Contains(valids, key.(string)) {
						if sobj.ptr.Matchs(cond) {
							valids = append(valids, key.(string)) // 在会话内存中，但是不在远端的，且满足筛选条件的，亦加入frets中
							frets = append(frets, sobj.ptr.(T))
							XLog.Notice("XOrm.List: add sobj: %v", key.(string))
						}
					}
					return true
				})
			}
			if meta.Cache {
				if gcache != nil { // 遍历全局内存
					added := make(map[string]*globalObject)
					gcache.Range(func(key, value any) bool {
						gobj := value.(*globalObject)
						if gobj.delete {
						} else if !XCollect.Contains(valids, key.(string)) {
							if gobj.ptr.Matchs(cond) {
								added[key.(string)] = gobj
							}
						}
						return true
					})
					for k, v := range added {
						valids = append(valids, k)          // 在全局内存中，但是不在远端的，且满足筛选条件的，亦加入frets中
						nobj := v.ptr.Clone()               // 内存拷贝
						sobj := setSessionCache(nobj, meta) // 监控内存
						sobj.setWritable(writable)
						frets = append(frets, nobj.(T))
						XLog.Notice("XOrm.List: add gobj: %v", k)
					}
				}
			}
		}
	}

	return frets
}
