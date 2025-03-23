// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XTime"
	"github.com/petermattis/goid"
)

var (
	// contextID 是上下文 ID 的原子计数器
	// 用于生成唯一的会话标识
	contextID   int64        // 上下文 ID 计数器
	contextMap  sync.Map     // 存储上下文映射，键为 goroutine ID，值为 context 实例
	contextPool = sync.Pool{ // 上下文对象池，用于复用 context 实例
		New: func() any {
			ctx := new(context)
			ctx.reset()
			return ctx
		},
	}
)

// context 定义了数据库操作的上下文信息，用于跟踪和管理数据库操作的生命周期。
type context struct {
	time     int  // 操作开始时间（微秒）
	writable bool // 是否为读写操作
}

// reset 重置上下文状态，将时间设为 0，读写标志设为 false。
func (ctx *context) reset() {
	ctx.time = 0
	ctx.writable = false
}

// Watch 开始监控数据库操作。接受可选的读写模式标志 writable，默认为 true（读写模式），
// 设为 false 则为只读模式。函数获取当前 goroutine ID，生成新的会话 ID，从对象池获取
// 上下文实例，并初始化上下文（记录开始时间和设置读写模式）。最后将上下文存储到全局映射，
// 并返回新分配的会话 ID。
//
// 使用示例：
//
//	sid := Watch()        // 开始读写监控
//	sid := Watch(false)   // 开始只读监控
//	defer Defer()         // 结束监控
func Watch(writable ...bool) int {
	gid := goid.Get()
	sid := int(atomic.AddInt64(&contextID, 1))
	ctx := contextPool.Get().(*context)
	ctx.time = XTime.GetMicrosecond()
	ctx.writable = true
	if len(writable) == 1 {
		ctx.writable = writable[0]
	}
	contextMap.Store(gid, ctx)

	XLog.Info("XOrm.Watch: start.")
	return sid
}

// Defer 结束数据库操作监控。函数获取当前 goroutine ID 并检索对应的上下文实例。
// 如果是读写操作，会创建新的管道批次，遍历会话内存中的对象（处理新建、删除和修改的数据），
// 同步数据到全局内存，并将变更放入管道进行异步处理。最后清理资源，记录性能指标（总耗时、
// 管道耗时、逻辑耗时），删除上下文映射，重置并回收上下文实例。
//
// 此函数应通过 defer 调用，确保每个 Watch 都有对应的 Defer。读写操作会触发数据同步。
func Defer() {
	gid := goid.Get()
	if val, _ := contextMap.Load(gid); val == nil {
		XLog.Error("XOrm.Defer: context was not found.")
		return
	} else {
		ctx := val.(*context)
		var commitCost int = 0
		var commitCount int = 0
		defer func() {
			logicCost := XTime.GetMicrosecond() - ctx.time - commitCost
			XLog.Info("XOrm.Defer: [Total:%.2fms] [Commit(%v):%.2fms] [Logic:%.2fms]",
				float64((XTime.GetMicrosecond()-ctx.time)/1e3),
				commitCount,
				float64(commitCost)/1e3,
				float64(logicCost)/1e3)

			contextMap.Delete(gid) // release memory
			ctx.reset()
			contextPool.Put(ctx)
		}()

		// 对比内容并且写入
		watchs, _ := sessionCacheMap.Load(gid)
		if watchs != nil {
			if ctx.writable {
				batch := commitBatchPool.Get().(*commitBatch)
				tag := XLog.Tag() // 和会话线程保持一致的日志标签
				if tag != nil {
					batch.tag = tag.Clone()
				} else {
					batch.tag = tag
				}
				batch.time = XTime.GetMicrosecond()
				batch.objs = make([]*commitObject, 0)
				watchs.(*sync.Map).Range(func(k1, v1 any) bool {
					v1.(*sync.Map).Range(func(k2, v2 any) bool {
						sobj := v2.(*sessionObject)
						if !sobj.model.Writable { // 全局只读数据
							return true
						}
						modify := false
						if sobj.create { // 新的数据
							sobj.ptr.OnEncode() // encode for writing object
						} else if sobj.delete || sobj.clear { // 标记为删除或清除的数据
						} else if sobj.setWritable() == 1 { // 只读数据，不对比，不写入
							return true
						} else { // 需要对比的数据
							sobj.ptr.OnEncode() // encode for comparing and writing object
							modify = sobj.ptr.Equals(sobj.raw)
						}
						if sobj.create || sobj.delete || sobj.clear || !modify {
							if (sobj.create || !modify) && sobj.model.Cache {
								// 同步被修改的数据至全局内存，Clear和Delete的内存将在pipe中被移除（避免脏数据）
								gcache := getGlobalCache(sobj.ptr)
								if gcache != nil {
									gkey := sobj.ptr.DataUnique()
									gobj, _ := gcache.Load(gkey)
									if gobj != nil {
										gobj.(*globalObject).ptr = sobj.ptr.Clone() // 拷贝内存
									}
								}
							}

							commitCount++
							pobj := commitObjectPool.Get().(*commitObject)
							pobj.raw = sobj.ptr.Clone() // 使用备份的数据写入
							pobj.create = sobj.create
							pobj.delete = sobj.delete
							pobj.clear = sobj.clear
							pobj.modify = !modify
							pobj.cond = sobj.cond
							pobj.meta = sobj.model
							batch.objs = append(batch.objs, pobj)
							if pobj.delete && !(sobj.model.Cache && !sobj.model.Persist) {
								// 因写入db是异步的，故加锁，避免XMem.List或XMem.Read脏数据（已被标记删除，但又被读取），需要在XMem.List和XMem.Read中判断_UGWait
								globalLock(pobj.raw)
							}
						}
						return true
					})
					return true
				})
				if len(batch.objs) > 0 {
					commit := getCommitBatch(gid)
					if commit == nil {
						XLog.Error("XOrm.Defer: commit batch have been closed.")
					} else {
						select {
						case commit <- batch:
							atomic.AddInt64(&sharedCommitMetrics.wait, 1)
						default:
							XLog.Error("XOrm.Defer: too many data to commit.")
						}
					}
				}
				commitCost = XTime.GetMicrosecond() - batch.time
			}
			sessionCacheMap.Delete(gid) // release memory
		}
		sessionListMap.Delete(gid) // release memory
	}
}
