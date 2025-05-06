// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XString"
	"github.com/eframework-org/GO.UTIL/XTime"
	"github.com/petermattis/goid"
)

var (
	// contextID 是上下文 ID 的原子计数器，用于生成唯一的会话标识。
	contextID int64

	// contextMap 存储了上下文映射，键为 goroutine ID，值为 context 实例。
	contextMap sync.Map

	// contextPool 是上下文对象池，用于复用 context 实例。
	contextPool = sync.Pool{New: func() any { return new(context) }}
)

// context 定义了 CRUD 操作的上下文信息，用于管理操作的生命周期。
type context struct {
	time          int   // 操作开始时间
	writable      bool  // 是否读写操作
	readCount     int64 // 读取操作次数
	readElapsed   int64 // 读取操作耗时
	listCount     int64 // 列举操作次数
	listElapsed   int64 // 列举操作耗时
	writeCount    int64 // 写入操作次数
	writeElapsed  int64 // 写入操作耗时
	deleteCount   int64 // 删除操作次数
	deleteElapsed int64 // 删除操作耗时
	clearCount    int64 // 清除操作次数
	clearElapsed  int64 // 清除操作耗时
	increCount    int64 // 自增操作次数
	increElapsed  int64 // 自增操作耗时
}

// reset 重置上下文状态。
func (ctx *context) reset() {
	ctx.time = 0
	ctx.writable = false
	ctx.readCount = 0
	ctx.readElapsed = 0
	ctx.listCount = 0
	ctx.listElapsed = 0
	ctx.writeCount = 0
	ctx.writeElapsed = 0
	ctx.deleteCount = 0
	ctx.deleteElapsed = 0
	ctx.clearCount = 0
	ctx.clearElapsed = 0
	ctx.increCount = 0
	ctx.increElapsed = 0
}

// getContext 根据 goroutine ID 获取上下文实例。
func getContext(gid ...int64) *context {
	var ggid int64 = 0
	if len(gid) > 0 {
		ggid = gid[0]
	} else {
		ggid = goid.Get()
	}
	var ctx *context
	value, _ := contextMap.Load(ggid)
	if value != nil {
		ctx = value.(*context)
	}
	return ctx
}

// Watch 开始 CRUD 操作监控。
// writable 是可选的，默认为 true（读写模式），设置为 false 则为只读模式。
// 函数获取当前 goroutine ID，生成新的会话 ID，从对象池获取上下文实例并初始化（记录开始时间和设置读写模式）。
// 并返回新分配的会话 ID。
//
// 使用示例：
//
//	sid := Watch()        // 开始 CRUD 监控。
//	defer Defer()         // 结束 CRUD 监控。
func Watch(writable ...bool) int {
	cacheDumpWait.Wait()

	gid := goid.Get()
	sid := int(atomic.AddInt64(&contextID, 1))
	ctx := contextPool.Get().(*context)
	ctx.time = XTime.GetMicrosecond()
	ctx.writable = true
	if len(writable) == 1 {
		ctx.writable = writable[0]
	}
	contextMap.Store(gid, ctx)

	tag := XLog.Tag()
	if tag != nil { // 设置日志标签
		tag.Set("Go", XString.ToString(int(gid)))
		tag.Set("Context", XString.ToString(sid))
	}

	XLog.Info("XOrm.Watch: context has been started.")
	return sid
}

// Defer 结束 CRUD 操作监控。
// 函数获取当前 goroutine ID 并检索对应的上下文实例。
// 如果是读写操作，会自动对比 CRUD 前后的数据变更（新建、删除和修改等）。
// 然后对变更进行合批并路由到（基于 goroutine ID）指定的队列中进行异步提交。
// 对于只读操作，仅清理会话映射，不进行数据同步。
//
// 此函数应通过 defer 调用，确保每个 Watch 都有对应的 Defer。
func Defer() {
	cacheDumpWait.Wait()

	gid := goid.Get()
	if val, _ := contextMap.LoadAndDelete(gid); val == nil {
		XLog.Error("XOrm.Defer: context was not found.")
		return
	} else {
		ctx := val.(*context)
		startTime := XTime.GetMicrosecond()
		var selfCost int = 0

		defer func() {
			if XLog.Able(XLog.LevelInfo) {
				otherCost := int64(XTime.GetMicrosecond() - ctx.time - selfCost)
				var crudLog string
				if ctx.readCount > 0 {
					crudLog += fmt.Sprintf("[Read(%v):%.2fms] ", ctx.readCount, float64(ctx.readElapsed)/1e3)
					otherCost -= ctx.readElapsed
				}
				if ctx.listCount > 0 {
					crudLog += fmt.Sprintf("[List(%v):%.2fms] ", ctx.listCount, float64(ctx.listElapsed)/1e3)
					otherCost -= ctx.listElapsed
				}
				if ctx.writeCount > 0 {
					crudLog += fmt.Sprintf("[Write(%v):%.2fms] ", ctx.writeCount, float64(ctx.writeElapsed)/1e3)
					otherCost -= ctx.writeElapsed
				}
				if ctx.deleteCount > 0 {
					crudLog += fmt.Sprintf("[Delete(%v):%.2fms] ", ctx.deleteCount, float64(ctx.deleteElapsed)/1e3)
					otherCost -= ctx.deleteElapsed
				}
				if ctx.clearCount > 0 {
					crudLog += fmt.Sprintf("[Clear(%v):%.2fms] ", ctx.clearCount, float64(ctx.clearElapsed)/1e3)
					otherCost -= ctx.clearElapsed
				}
				if ctx.increCount > 0 {
					crudLog += fmt.Sprintf("[Incre(%v):%.2fms] ", ctx.increCount, float64(ctx.increElapsed)/1e3)
					otherCost -= ctx.increElapsed
				}
				XLog.Info("XOrm.Defer: context has been deferred, elapsed %.2fms for %v[Self:%.2fms] [Other:%.2fms].",
					float64((XTime.GetMicrosecond()-ctx.time))/1e3,
					crudLog,
					float64(selfCost)/1e3,
					float64(otherCost)/1e3)
			}
			ctx.reset()
			contextPool.Put(ctx)
		}()

		var batch *commitBatch
		var scache *sync.Map
		if tmp, _ := sessionCacheMap.LoadAndDelete(gid); tmp != nil {
			scache = tmp.(*sync.Map)
		}
		if scache != nil {
			if ctx.writable {
				batch = commitBatchPool.Get().(*commitBatch)
				tag := XLog.Tag() // 保持和上下文一致的日志标签
				if tag != nil {
					batch.tag = tag.Clone()
				} else {
					batch.tag = tag
				}
				batch.posthandler = func(batch *commitBatch, sobj *sessionObject) {
					obj := sobj.raw
					if sobj.delete || sobj.clear != nil {
						meta := getModelMeta(obj)
						if meta.cache {
							gcache := getGlobalCache(obj)
							if gcache != nil {
								if sobj.delete {
									key := obj.DataUnique()
									gobj, exist := gcache.Load(key)
									if exist {
										ggobj := gobj.(IModel)
										if !ggobj.IsValid() {
											gcache.Delete(key) // 同步被删除的数据至全局内存
										} else {
											// 因延迟写入，有可能该数据又被标记为写入（被新数据覆盖）
										}
									}
								} else {
									var deleteKeys []string
									gcache.Range(func(key, value any) bool {
										gobj := value.(IModel)
										if !gobj.IsValid() {
											deleteKeys = append(deleteKeys, key.(string))
										}
										return true
									})
									if len(deleteKeys) > 0 {
										for _, key := range deleteKeys {
											gcache.Delete(key) // 同步被删除的数据至全局内存
										}
									}
								}
							}
						}
						globalUnlock(obj) // 解锁数据表
					}
					sobj.reset()
					sessionObjectPool.Put(sobj) // 回收会话内存
				}

				var batchChunks [][][]*sessionObject
				concurrentRange(scache, func(index1 int, key1, value1 any) bool {
					watch := value1.(*sync.Map)
					if watch != nil {
						concurrentRange(watch, func(index2 int, key2, value2 any) bool {
							sobj := value2.(*sessionObject)
							if sobj == nil {
								return true
							}
							meta := getModelMeta(sobj.ptr)
							if meta.writable { // 不处理全局只读数据
								update := false
								if sobj.create { // 新的数据
									sobj.ptr.OnEncode() // encode for writing object
								} else if !sobj.ptr.IsValid() { // 标记为删除或无效的数据
								} else if sobj.isWritable() == 1 { // 只读数据，不对比，不写入
									return true
								} else { // 需要对比的数据
									sobj.ptr.OnEncode() // encode for comparing and writing object
									update = !sobj.ptr.Equals(sobj.raw)
								}
								if update || sobj.create || sobj.delete || sobj.clear != nil {
									if (update || sobj.create) && meta.cache {
										// 同步被修改的数据至全局内存，Clear 和 Delete 的内存将在 pipe 中被移除（避免脏数据）
										gcache := getGlobalCache(sobj.ptr)
										if gcache != nil {
											gkey := sobj.ptr.DataUnique()
											if _, loaded := gcache.Load(gkey); loaded {
												gcache.Store(gkey, sobj.ptr.Clone()) // 拷贝内存
											}
										}
									}
									if sobj.delete || sobj.clear != nil {
										// 因提交 database 是异步的，故加锁，避免 XOrm.List 或 XOrm.Read 脏数据（已被标记删除，但又被读取），需要在 XOrm.List 和 XOrm.Read 中判断 globalWait。
										globalLock(sobj.ptr)
									}

									batchChunks[index1][index2] = append(batchChunks[index1][index2], sobj)
								}
							}
							return true
						}, func(chunk2 int) { batchChunks[index1] = make([][]*sessionObject, chunk2) })
					}
					return true
				}, func(chunk1 int) { batchChunks = make([][][]*sessionObject, chunk1) })

				for _, chunk := range batchChunks {
					for _, objs := range chunk {
						if len(objs) > 0 {
							batch.objects = append(batch.objects, objs...)
						}
					}
				}
			} else {
				scache.Range(func(key, value any) bool {
					watch := value.(*sync.Map)
					if watch != nil {
						watch.Range(func(key, value any) bool {
							sobj := value.(*sessionObject)
							if sobj != nil {
								sobj.reset()
								sessionObjectPool.Put(sobj) // 回收会话内存
							}
							return true
						})
					}
					return true
				})
			}
		}
		sessionListMap.Delete(gid) // 清除会话列举标识

		if batch != nil {
			if len(batch.objects) > 0 {
				batch.submit()
			}
			selfCost = XTime.GetMicrosecond() - startTime
		}
	}
}
