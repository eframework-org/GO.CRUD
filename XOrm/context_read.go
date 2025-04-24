// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
)

// Read 从数据源读取数据模型。model 参数为要读取的数据模型，必须实现 IModel 接口。
// writableAndCond 为可变参数，可包含布尔值（表示是否可写）和查询条件对象（*Condition 类型）。
//
// 函数根据是否有查询条件采用不同的读取策略。对于精确查找（无查询条件），首先尝试从会话内存中读取，
// 如果启用缓存则尝试从全局内存中读取，最后从远端数据源读取。对于模糊查找（有查询条件），
// 仅缓存模式下先查询会话内存再查询全局内存；其他模式则按照会话列表、全局列表、远端数据源的顺序查找。
//
// 函数返回读取到的数据模型，如果数据被标记为删除，模型的 IsValid 将被设置为 false。
func Read[T IModel](model T, writableAndCond ...any) T {
	gid := goid.Get()
	meta := getModelInfo(model)
	if meta == nil {
		XLog.Critical("XOrm.Read: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
		return model
	}
	writable := meta.Writable
	var cond *condition
	for _, v := range writableAndCond {
		switch nv := v.(type) {
		case bool:
			writable = nv
		case *condition:
			cond = nv
		default:
			XLog.Critical("XOrm.Read: writableAndCond of %v type is error: %v", v, XLog.Caller(1, false))
		}
	}
	if cond == nil { // 精确查找
		isGet := false
		scache := getSessionCache(gid, model)
		if scache != nil { // 会话内存读取
			obj, _ := scache.Load(model.DataUnique())
			if obj != nil {
				sobj := obj.(*sessionObject)
				if sobj.clear || sobj.delete {
					// 已经被标记删除，则不读取
					model.IsValid(false)
				} else {
					model = sobj.ptr.(any).(T)
					sobj.setWritable(writable)
				}
				isGet = true
			}
		}
		if !isGet && meta.Cache { // 全局内存读取
			gcache := getGlobalCache(model)
			if gcache != nil {
				obj, _ := gcache.Load(model.DataUnique())
				if obj != nil {
					gobj := obj.(*globalObject)
					if gobj.delete {
						// 已经被标记删除，则不读取
						model.IsValid(false)
					} else {
						model = gobj.ptr.Clone().(any).(T)        // 内存拷贝
						sobj := setSessionCache(gid, model, meta) // 监控内存
						sobj.setWritable(writable)
					}
					isGet = true
				}
			}
		}
		if !isGet { // 远端读取
			globalWait("XOrm.Read", model, meta)
			if model.Read(cond) {
				isGet = true
				if meta.Cache {
					setGlobalCache(model.Clone()) // 保存至全局内存中
				}
				setSessionCache(gid, model, meta) // 监控内存
			}
		}
	} else { // 模糊查找
		if meta.Cache && !meta.Persist { // 仅缓存，则先查询会话内存，而后查询全局内存
			var isGet int32
			scache := getSessionCache(gid, model)
			if scache != nil { // 会话内存读取
				concurrentRange(scache, func(index int, key, value any) bool {
					sobj := value.(*sessionObject)
					if sobj.clear || sobj.delete {
						// 已经被标记删除，则不读取
					} else if sobj.ptr.Matchs(cond) {
						model = sobj.ptr.(any).(T)
						sobj.setWritable(writable)
						atomic.SwapInt32(&isGet, 1)
						return false
					}
					return true
				})
			}
			if isGet == 0 {
				gcache := getGlobalCache(model)
				if gcache != nil { // 全局内存读取
					concurrentRange(gcache, func(index int, key, value any) bool {
						gobj := value.(*globalObject)
						if gobj.delete {
							// 已经被标记删除，则不读取
						} else if gobj.ptr.Matchs(cond) {
							model = gobj.ptr.Clone().(any).(T)        // 内存拷贝
							sobj := setSessionCache(gid, model, meta) // 监控内存
							sobj.setWritable(writable)
							return false
						}
						return true
					})
				}
			}
		} else {
			if isSessionListed(gid, model, false, false, cond) { // 会话内存被列举过
				scache := getSessionCache(gid, model)
				if scache != nil { // 会话内存读取
					concurrentRange(scache, func(index int, key, value any) bool {
						sobj := value.(*sessionObject)
						if sobj.clear || sobj.delete {
							// 已经被标记删除，则不读取
						} else if sobj.ptr.Matchs(cond) {
							model = sobj.ptr.(any).(T)
							sobj.setWritable(writable)
							return false
						}
						return true
					})
				}
			} else if isGlobalListed(model, meta, false, false, cond) { // 全局内存被列举过
				gcache := getGlobalCache(model)
				if gcache != nil { // 全局内存读取
					concurrentRange(gcache, func(index int, key, value any) bool {
						gobj := value.(*globalObject)
						if gobj.delete {
							// 已经被标记删除，则不读取
						} else if gobj.ptr.Matchs(cond) {
							model = gobj.ptr.Clone().(any).(T)        // 内存拷贝
							sobj := setSessionCache(gid, model, meta) // 监控内存
							sobj.setWritable(writable)
							return false
						}
						return true
					})
				}
			} else { // 远端筛选
				globalWait("XOrm.Read", model, meta)
				if model.Read(cond) {
					// 判断内存中是否有
					isSCache := false
					scache := getSessionCache(gid, model)
					if scache != nil { // 会话内存读取
						obj, _ := scache.Load(model.DataUnique())
						if obj != nil {
							sobj := obj.(*sessionObject)
							if sobj.clear || sobj.delete {
								// 已经被标记删除，则不读取
								model.IsValid(false) // 设置为不合法，直接返回
								XLog.Notice("XOrm.Read: del sobj: %v", model.DataUnique())
								return model
							} else {
								if model.Matchs(cond) { // 执行一遍条件
									sobj := obj.(*sessionObject)
									model = sobj.ptr.(any).(T) // 使用会话内存替换
									sobj.setWritable(writable)
									isSCache = true
									XLog.Notice("XOrm.Read: use sobj: %v", model.DataUnique())
								} else {
									// 注意此处可能导致数据覆盖：从远端模糊查找返回的对象已在内存中，但内存对象并不满足筛选条件，若该内存对象有修改，则会导致这些修改被覆盖（无效）
									XLog.Error("XOrm.Read: remote-obj overwrited smem-obj because of mismatched condition, the change of smem-obj will be discarded, name = %v", model.DataUnique())
								}
							}
						}
					}
					if meta.Cache { // 全局内存读取
						gcache := getGlobalCache(model)
						if gcache != nil {
							obj, _ := gcache.Load(model.DataUnique())
							if obj != nil {
								// 已经在全局内存中
								gobj := obj.(*globalObject)
								if gobj.delete {
									// 已经被标记删除，则不读取
									model.IsValid(false) // 设置为不合法，直接返回
									XLog.Notice("XOrm.Read: del gobj: %v", model.DataUnique())
									return model
								} else if !isSCache { // 未在会话内存中，但在全局内存中，替换之
									model = gobj.ptr.Clone().(any).(T)        // 内存拷贝
									sobj := setSessionCache(gid, model, meta) // 监控内存
									sobj.setWritable(writable)
									XLog.Notice("XOrm.Read: use gobj: %v", model.DataUnique())
								}
							} else {
								setGlobalCache(model.Clone()) // 保存至全局内存
							}
						}
					}
					setSessionCache(gid, model, meta) // 监控内存
				}
			}
		}
	}
	return model
}
