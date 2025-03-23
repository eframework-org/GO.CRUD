// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XLoom"
	"github.com/eframework-org/GO.UTIL/XTime"
	"github.com/illumitacit/gostd/quit"
	"github.com/petermattis/goid"
)

// commitBatchCapacity 定义了单个提交队列的最大容量，当超过此容量时，新的批次将被丢弃。
const commitBatchCapacity = 100000

// commitMetrics 定义了提交队列的统计信息。
type commitMetrics struct {
	wait  int64 // 等待处理的批次数
	count int64 // 已处理的对象总数
}

// Wait 返回当前等待处理的批次数。
func (cm *commitMetrics) Wait() int64 {
	return atomic.LoadInt64(&cm.wait)
}

// Count 返回已处理的对象总数，可选择是否重置计数。reset 参数为 true 时会将计数器重置为 0。
func (cm *commitMetrics) Count(reset ...bool) int64 {
	if len(reset) == 1 && reset[0] {
		return atomic.SwapInt64(&cm.count, 0)
	} else {
		return atomic.LoadInt64(&cm.count)
	}
}

// Commit 提供了提交队列统计信息的全局访问点。
var sharedCommitMetrics = &commitMetrics{}

func (mi *metricsInfo) Commit() *commitMetrics {
	return sharedCommitMetrics
}

var (
	// commitMap 存储了所有活动的提交队列，键为 goroutine ID，值为对应的批次队列。
	commitMap        sync.Map  // map[int64]*commitBatch
	commitObjectPool sync.Pool = sync.Pool{
		New: func() any {
			obj := new(commitObject)
			obj.reset()
			return obj
		},
	}
	commitBatchPool sync.Pool = sync.Pool{
		New: func() any {
			batch := new(commitBatch)
			batch.reset()
			return batch
		},
	}

	initSigMap sync.Map //map[int64]chan os.Signal
	// 用于管理提交处理器的刷新和退出
	commitWaitGroup    sync.WaitGroup // 等待所有提交处理器完成
	flushSigMap        sync.Map       //map[int64]chan *sync.WaitGroup
	sharedCommitFlush  int32          // 提交批次是否已刷新的标志
	sharedCommitClosed int32          // 提交批次是否已关闭的标志
)

// commitObject 定义了提交队列中传输的数据对象，包含了对象的状态和元数据信息。
type commitObject struct {
	raw    IModel     // 原始数据对象
	create bool       // 创建标记，表示对象是新创建的
	delete bool       // 删除标记，表示对象需要被删除
	clear  bool       // 清理标记，表示对象需要被清理
	modify bool       // 修改标记，表示对象被修改过
	cond   *condition // 查询条件，用于清理操作
	meta   *modelInfo // 模型元数据，包含持久化和缓存配置
}

// reset 重置提交对象的状态，在对象被放回对象池前调用。
func (obj *commitObject) reset() {
	obj.raw = nil
	obj.create = false
	obj.delete = false
	obj.clear = false
	obj.modify = false
	obj.cond = nil
	obj.meta = nil
}

// commitBatch 定义了一批需要处理的数据对象，用于批量异步处理数据变更。
type commitBatch struct {
	tag  *XLog.LogTag    // 日志标签，用于追踪批次处理
	time int             // 批次创建时间（微秒）
	objs []*commitObject // 待处理的对象列表
}

// reset 重置批次对象的状态，在批次被放回对象池前调用。
func (cb *commitBatch) reset() {
	cb.tag = nil
	cb.time = 0
	cb.objs = nil
}

// commit 提交待处理的批次对象
func (cb *commitBatch) commit() {
	if cb.tag != nil {
		XLog.Watch(cb.tag)
	}
	waitTime := XTime.GetMicrosecond() - cb.time
	nt := XTime.GetMicrosecond()
	// 先处理删除操作，尽早释放锁，提高效率
	for _, cobj := range cb.objs {
		if cobj.delete {
			handleCommitObject(cobj)
		}
	}
	for _, cobj := range cb.objs {
		if !cobj.delete {
			handleCommitObject(cobj)
		}
	}
	costTime := XTime.GetMicrosecond() - nt
	XLog.Notice("XOrm.Commit: [Finish] [Cost:%.2fms] [Wait:%.2fms] pushed %v object(s)",
		float64(costTime)/1e3,
		float64(waitTime)/1e3,
		len(cb.objs))
	atomic.AddInt64(&sharedCommitMetrics.wait, -1)
	atomic.AddInt64(&sharedCommitMetrics.count, int64(len(cb.objs)))

	if cb.tag != nil {
		XLog.Defer()
	}

	cb.reset()
	commitBatchPool.Put(cb)

	for _, cobj := range cb.objs {
		cobj.reset()
		commitObjectPool.Put(cobj)
	}
}

// getCommitBatch 为指定的 goroutine 分配一个提交队列。gid 参数为 goroutine ID。函数首先检查是否已存在队列，
// 如果不存在，则创建新的队列，启动异步处理线程，并注册队列到全局映射。异步处理线程会监听队列中的批次，
// 设置日志标签，优先处理删除操作，然后处理其他操作，记录性能指标，最后清理和回收资源。函数返回分配的队列。
func getCommitBatch(gid int64) chan *commitBatch {
	var queue chan *commitBatch
	val, _ := commitMap.Load(gid)
	if val != nil {
		queue = val.(chan *commitBatch)
	}
	if atomic.LoadInt32(&sharedCommitClosed) > 0 {
		//关闭直接返回
		return nil
	}
	if queue == nil {
		queue = make(chan *commitBatch, commitBatchCapacity)
		commitMap.Store(gid, queue)
		wg := sync.WaitGroup{}
		wg.Add(1)
		doneOnce := sync.Once{}
		XLoom.RunAsyncT1(func(gid int64) {
			initVal, _ := initSigMap.LoadOrStore(gid, make(chan os.Signal, 1))
			ch := initVal.(chan os.Signal)
			signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)

			quit.GetWaiter().Add(1)
			commitWaitGroup.Add(1)
			doneOnce.Do(func() { // 确保只调用一次，否则recover后会重复调用
				wg.Done() // 确保线程启动完成
			})
			flushVal, _ := flushSigMap.LoadOrStore(gid, make(chan *sync.WaitGroup, 1))
			flushSig := flushVal.(chan *sync.WaitGroup)
			defer func() {
				// 处理剩余的批次
				for {
					if len(queue) > 0 {
						batch := <-queue
						batch.commit()
						continue
					} else {
						break
					}
				}
				flushSigMap.Delete(gid)
				commitMap.Delete(gid)
				quit.GetWaiter().Done()
				commitWaitGroup.Done()
			}()
			for {
				select {
				case batch := <-queue:
					if batch == nil {
						return
					}
					batch.commit()
				case fwg := <-flushSig:
					for {
						if len(queue) > 0 {
							batch := <-queue
							batch.commit()
							continue
						} else {
							break
						}
					}
					fwg.Done()
				case sig, ok := <-ch:
					if ok {
						XLog.Notice("XOrm.Loop(%v): receive signal of %v.", gid, sig.String())
					} else {
						XLog.Notice("XOrm.Loop(%v): channel of signal is closed.", gid)
					}
					return
				case <-quit.GetQuitChannel():
					XLog.Notice("XOrm.Loop: receive signal of QUIT.")
					return
				}
			}
		}, gid, true)
		wg.Wait()
	}
	return queue
}

// handleCommitObject 执行数据对象的持久化操作。cobj 参数为要处理的提交对象。函数根据对象的标记执行不同的操作：
// 对于删除操作，执行实际删除，同步全局内存，并解除锁定；对于创建操作，执行写入；对于清理操作，执行清理
// 并更新全局列举标记；对于修改操作，执行写入。函数优先处理删除操作以提高效率，确保全局内存与远端数据同步，
// 并记录操作耗时和详细日志。
func handleCommitObject(cobj *commitObject) {
	t1 := XTime.GetMicrosecond()
	obj := cobj.raw
	k := obj.DataUnique()

	action := ""
	if cobj.delete {
		if !cobj.clear && cobj.meta.Persist {
			// 仅删除
			obj.Delete()
			action = "Delete"
		} else {
			// 删除 && 清理 则表示该对象会在清理逻辑中被删除，无需重复delete
		}

		if cobj.meta.Cache {
			gcache := getGlobalCache(obj)
			if gcache != nil {
				gobj, exist := gcache.Load(k)
				if exist {
					ggobj := gobj.(*globalObject)
					if ggobj.delete {
						if action == "" {
							action = "Delete"
						}
						gcache.Delete(k) // 同步被删除的数据至全局内存
					} else {
						// 因延迟写入，有可能该数据又被标记为写入（被新数据覆盖）
					}
				}
			}
		}

		// 解锁
		if !(cobj.meta.Cache && !cobj.meta.Persist) {
			globalUnlock(obj)
		}
	} else if cobj.create {
		obj.Write()
		action = "Create"
	} else if cobj.clear {
		obj.Clear(cobj.cond)
		isGlobalListed(obj, cobj.meta, false, true) // 清除全局内存列举标记
		action = "Clear"
	} else if cobj.modify {
		obj.Write()
		action = "Update"
	}

	if action != "" {
		t2 := XTime.GetMicrosecond()
		XLog.Notice("XOrm.Commit: [%v] [Cost:%.2fms] %v: %v", action, float64(t2-t1)/1e3, k, obj.Json())
	}
}

// FlushNow 刷新当前协程提交处理器并等待未完成的批次处理完成
// 如果传入参数则刷新指定的协程id
func FlushNow(gid ...int64) {
	var pid int64
	if len(gid) > 0 {
		pid = gid[0]
	} else {
		pid = goid.Get()
	}
	flushVal, _ := flushSigMap.Load(pid)
	if flushVal != nil {
		fwg := flushVal.(chan *sync.WaitGroup)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		select {
		case fwg <- wg:
			wg.Wait()
		}
		XLog.Notice("XOrm.FlushNow: cur goroutine commit batch have been flushed.")
	}
}

// Flush 刷新所有提交处理器并等待所有未完成的批次处理完成。
func Flush() {
	if atomic.LoadInt32(&sharedCommitClosed) == 0 {
		if atomic.CompareAndSwapInt32(&sharedCommitFlush, 0, 1) {
			commitMap.Range(func(key any, value any) bool {
				if gid, ok := key.(int64); ok {
					FlushNow(gid)
				}
				return true
			})
			atomic.CompareAndSwapInt32(&sharedCommitFlush, 1, 0)
			XLog.Notice("XOrm.Flush: all commit batch have been flushed.")
		} else {
			XLog.Notice("XOrm.Flush: all commit batch have been flushed by another goroutine.")
		}
	} else {
		XLog.Notice("XOrm.Flush: all commit batch have been closed by another goroutine.")
	}
}

// Close 关闭所有提交处理器并等待所有未完成的批次处理完成。
// 此函数会发送退出信号并等待所有处理器完成当前工作。
func Close() {
	if atomic.CompareAndSwapInt32(&sharedCommitClosed, 0, 1) {
		initSigMap.Range(func(key any, value any) bool {
			if ch, ok := value.(chan os.Signal); ok {
				signal.Stop(ch)
				close(ch)
			}
			return true
		})
		// 等待所有处理器完成
		commitWaitGroup.Wait()
	} else {
		XLog.Notice("XOrm.Close: all commit batch have been closed by another goroutine.")
	}
}
