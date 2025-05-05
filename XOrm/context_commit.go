// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XLoom"
	"github.com/eframework-org/GO.UTIL/XPrefs"
	"github.com/eframework-org/GO.UTIL/XTime"
	"github.com/illumitacit/gostd/quit"
	"github.com/petermattis/goid"
)

const (
	// commitQueueCountPrefs 定义了提交队列的数量的偏好设置键。
	commitQueueCountPrefs = "Orm/Commit/Queue"

	// commitBatchCountPrefs 定义了单个队列的最大容量的偏好设置键。
	commitBatchCountPrefs = "Orm/Commit/Batch"
)

var (
	// commitQueueCount 定义了提交队列的数量，默认为 CPU 核心数。
	commitQueueCount int = runtime.NumCPU()

	// commitBatchCount 定义了单个队列的最大容量，当超过此容量时，新的批次将被丢弃。
	commitBatchCount int = 100000

	// commitQueues 定义了提交队列的切片，用于缓冲待处理的批次数据。
	commitQueues []chan *commitBatch

	// commitSetupSig 定义了提交队列的信号通道，用于接收退出信号。
	commitSetupSig []chan os.Signal

	// commitFlushWait 定义了提交队列的等待通道，用于等待批次处理完成。
	commitFlushWait []chan *sync.WaitGroup

	// commitCloseWait 定义了提交队列的关闭通道，用于等待所有队列关闭完成。
	commitCloseWait sync.WaitGroup

	// commitFlushSig 定义了提交批次是否已刷新的标志，用于控制批次的状态。
	commitFlushSig int32

	// commitCloseSig 定义了提交队列是否已关闭的标志，用于控制队列的状态。
	commitCloseSig int32

	// commitBatchPool 定义了批次对象的对象池，用于重用已创建的批次对象。
	commitBatchPool sync.Pool = sync.Pool{
		New: func() any {
			obj := new(commitBatch)
			obj.reset()
			return obj
		},
	}
)

func init() { setupCommit(XPrefs.Asset()) }

// setupCommit 初始化提交队列。
// 该函数会从 prefs 中获取提交队列的数量和批次大小，并启动提交队列循环。
func setupCommit(prefs XPrefs.IBase) {
	Close()

	commitQueueCount = prefs.GetInt(commitQueueCountPrefs, commitQueueCount)
	commitBatchCount = prefs.GetInt(commitBatchCountPrefs, commitBatchCount)

	if commitQueueCount <= 0 {
		commitQueueCount = runtime.NumCPU()
	}

	if commitBatchCount <= 0 {
		commitBatchCount = 100000
	}

	commitQueues = make([]chan *commitBatch, commitQueueCount)
	commitSetupSig = make([]chan os.Signal, commitQueueCount)
	commitFlushWait = make([]chan *sync.WaitGroup, commitQueueCount)
	for i := range commitQueueCount {
		commitQueues[i] = make(chan *commitBatch, commitBatchCount)
		commitSetupSig[i] = make(chan os.Signal, 1)
		commitFlushWait[i] = make(chan *sync.WaitGroup, 1)
	}

	commitCloseWait = sync.WaitGroup{}
	atomic.StoreInt32(&commitFlushSig, 0)
	atomic.StoreInt32(&commitCloseSig, 0)

	// 启动提交队列线程
	wg := sync.WaitGroup{}
	for i := range commitQueueCount {
		wg.Add(1)
		XLoom.RunAsyncT2(func(queueID int, doneOnce *sync.Once) {
			setupSig := commitSetupSig[queueID]
			signal.Notify(setupSig, syscall.SIGTERM, syscall.SIGINT)

			quit.GetWaiter().Add(1)
			commitCloseWait.Add(1)
			doneOnce.Do(func() { // 确保只调用一次，否则recover后会重复调用
				wg.Done() // 确保线程启动完成
			})

			flushSig := commitFlushWait[queueID]
			queue := commitQueues[queueID] // 提交队列，用于接收批次数据

			defer func() {
				// 处理剩余的批次
				for {
					if len(queue) > 0 {
						batch := <-queue
						batch.push()
						continue
					} else {
						break
					}
				}
				quit.GetWaiter().Done()
				commitCloseWait.Done()
			}()

			for {
				select {
				case batch := <-queue:
					if batch == nil {
						return
					}
					batch.push()
				case fwg := <-flushSig:
					for {
						if len(queue) > 0 {
							batch := <-queue
							batch.push()
							continue
						} else {
							break
						}
					}
					fwg.Done()
				case sig, ok := <-setupSig:
					if ok {
						XLog.Notice("XOrm.Commit.Setup(%v): receive signal of %v.", queueID, sig.String())
					} else {
						XLog.Notice("XOrm.Commit.Setup(%v): channel of signal is closed.", queueID)
					}
					return
				case <-quit.GetQuitChannel():
					XLog.Notice("XOrm.Commit.Setup(%v): receive signal of QUIT.", queueID)
					return
				}
			}
		}, i, &sync.Once{}, true)
	}
	wg.Wait()

	XLog.Notice("XOrm.Commit.Setup: queue of commit count is %v, batch of queue count is %v.", commitQueueCount, commitBatchCount)
}

// commitBatch 定义了一批需要处理的数据对象，用于批量异步处理数据变更。
type commitBatch struct {
	tag         *XLog.LogTag                                  // 日志标签，用于追踪批次处理
	time        int                                           // 批次创建时间（微秒）
	objects     []*sessionObject                              // 待处理的对象列表
	prehandler  func(batch *commitBatch, sobj *sessionObject) // 预处理函数，在处理对象前调用
	posthandler func(batch *commitBatch, sobj *sessionObject) // 后处理函数，在处理对象后调用
}

// reset 重置批次对象的状态，在批次被放回对象池前调用。
func (cb *commitBatch) reset() {
	cb.tag = nil
	cb.time = 0
	cb.objects = nil
}

// submit 提交批次对象至队列中，等待被处理。
func (cb *commitBatch) submit(gid ...int64) {
	if atomic.LoadInt32(&commitCloseSig) > 0 {
		return
	}

	var ggid int64
	if len(gid) > 0 {
		ggid = gid[0]
	} else {
		ggid = goid.Get()
	}

	// 确保 queue ID 在 0 到 commitQueueCount 之间，相同的 goroutine ID 会被分配到同一个队列。
	queueID := max(int(ggid)%commitQueueCount, 0)
	queue := commitQueues[queueID]

	select {
	case queue <- cb:
	default:
		XLog.Error("XOrm.Commit.Submit: too many data to commit.")
	}
}

// push 推送批次对象至远端数据库或缓存中。
func (cb *commitBatch) push() {
	if cb.tag != nil {
		XLog.Watch(cb.tag)
	}
	waitTime := XTime.GetMicrosecond() - cb.time
	nowTime := XTime.GetMicrosecond()

	// 优先处理清除操作，尽早释放全局锁，提高效率
	for _, cobj := range cb.objects {
		if cobj.clear != nil {
			cb.handle(cobj)
		}
	}

	// 优先处理删除操作，尽早释放全局锁，提高效率
	for _, cobj := range cb.objects {
		if cobj.delete {
			cb.handle(cobj)
		}
	}

	for _, cobj := range cb.objects {
		if cobj.ptr != nil && !cobj.delete && cobj.clear == nil {
			cb.handle(cobj)
		}
	}

	costTime := XTime.GetMicrosecond() - nowTime
	XLog.Notice("XOrm.Commit.Push: [Finish] [Cost:%.2fms] [Wait:%.2fms] pushed %v object(s).",
		float64(costTime)/1e3,
		float64(waitTime)/1e3,
		len(cb.objects))

	if cb.tag != nil {
		XLog.Defer()
	}

	cb.reset()
	commitBatchPool.Put(cb)
}

func (cb *commitBatch) handle(sobj *sessionObject) {
	startTime := XTime.GetMicrosecond()
	obj := sobj.ptr
	key := obj.DataUnique()

	// 回调预处理函数。
	if cb.prehandler != nil {
		cb.prehandler(cb, sobj)
	}

	action := ""
	if sobj.create {
		obj.Write()
		action = "Create"
	} else if sobj.delete {
		obj.Delete()
		action = "Delete"
	} else if sobj.clear != nil {
		obj.Clear(sobj.clear)
		action = "Clear"
	} else {
		obj.Write()
		action = "Update"
	}

	// 回调后处理函数。
	if cb.posthandler != nil {
		cb.posthandler(cb, sobj)
	}

	if action != "" {
		t2 := XTime.GetMicrosecond()
		XLog.Notice("XOrm.Commit.Push: [%v] [Cost:%.2fms] %v: %v.", action, float64(t2-startTime)/1e3, key, obj.Json())
	}
}

// Flush 将等待指定的队列提交完成。
// gid 参数为 goroutine ID，若未指定，则使用当前 goroutine ID，
// 若 gid 为 -1，则表示等待所有的队列提交完成。
func Flush(gid ...int64) {
	if atomic.LoadInt32(&commitCloseSig) == 0 {
		var ggid int64
		if len(gid) > 0 {
			ggid = gid[0]
		} else {
			ggid = goid.Get()
		}
		if ggid == -1 {
			if atomic.CompareAndSwapInt32(&commitFlushSig, 0, 1) {
				for index := range commitQueues {
					sig := commitFlushWait[index]
					if sig != nil {
						wg := &sync.WaitGroup{}
						wg.Add(1)
						select {
						case sig <- wg:
							wg.Wait()
						}
						XLog.Notice("XOrm.Flush: batches of commit queue-%v has been flushed.", index)
					}
				}
				atomic.CompareAndSwapInt32(&commitFlushSig, 1, 0)
				XLog.Notice("XOrm.Flush: batches of all commit queue has been flushed.")
			}
		} else {
			queueID := max(int(ggid)%commitQueueCount, 0)
			sig := commitFlushWait[queueID]
			if sig != nil {
				wg := &sync.WaitGroup{}
				wg.Add(1)
				select {
				case sig <- wg:
					wg.Wait()
				}
				XLog.Notice("XOrm.Flush: batches of commit queue-%v has been flushed.", queueID)
			}
		}
	}
}

// Close 关闭所有的提交队列并等待所有未完成的批次处理完成。
// 此函数会发送退出信号并等待所有队列完成当前工作。
func Close() {
	if atomic.CompareAndSwapInt32(&commitCloseSig, 0, 1) {
		for _, sig := range commitSetupSig {
			signal.Stop(sig)
			close(sig)
		}
		// 等待所有队列完成
		commitCloseWait.Wait()
	}
}
