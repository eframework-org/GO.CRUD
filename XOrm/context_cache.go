// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XLoom"
)

var (
	// globalCacheMap 存储全局内存缓存，键为模型标识，值为对象映射 map[string]*globalObject。
	globalCacheMap sync.Map

	// globalListMap 存储全局列举标记，键为模型标识，值为是否已列举(bool)。
	globalListMap sync.Map

	// sessionCacheMap 存储会话内存缓存，键为 goroutine ID，值为模型对象映射。
	sessionCacheMap sync.Map

	// sessionListMap 存储会话列举标记，键为 goroutine ID，值为模型列举状态。
	sessionListMap sync.Map
)

// globalObject 定义了全局缓存中的对象结构。
type globalObject struct {
	ptr    IModel // 模型实例指针
	delete bool   // 是否已标记为删除
}

// sessionObject 定义了会话缓存中的对象结构。
type sessionObject struct {
	raw      IModel     // 原始模型实例（未修改的副本）
	ptr      IModel     // 工作模型实例（当前使用的实例）
	create   bool       // 是否为新建记录
	delete   bool       // 是否已标记为删除
	clear    bool       // 是否已标记为清除
	cond     *condition // 查询条件
	writable int        // 读写状态（0：未标记，1：只读，2：读写）
	model    *modelInfo // 模型元信息
}

// setWritable 设置或获取对象的读写状态。
// status 为可选的读写标志，true 表示读写，false 表示只读。
// 返回当前的读写状态（0：未标记，1：只读，2：读写）。
// 若对象已被标记为读写(2)，则无法降级为只读(1)。
// 未标记状态(0)可以设置为任意状态。
// 只读状态(1)可以升级为读写状态(2)。
func (so *sessionObject) setWritable(status ...bool) int {
	if len(status) >= 1 {
		// 若该数据已经被标记为读写，则无法修改为只读
		if so.writable == 0 { // 未标记
			if status[0] {
				so.writable = 2
			} else {
				so.writable = 1
			}
		} else if so.writable == 1 { // 只读
			if status[0] {
				so.writable = 2
			}
		}
	}
	return so.writable
}

// getGlobalCache 获取指定模型的全局内存映射。
// model 为模型实例。
// 返回对象映射，如果不存在则返回 nil。
func getGlobalCache(model IModel) *sync.Map {
	value, _ := globalCacheMap.Load(model.ModelUnique())
	if value != nil {
		return value.(*sync.Map)
	}
	return nil
}

// getSessionCache 获取当前会话中指定模型的内存映射。
// gid 为 goroutine ID。
// model 为模型实例。
// 返回对象映射，如果不存在则返回 nil。
func getSessionCache(gid int64, model IModel) *sync.Map {
	value, _ := sessionCacheMap.Load(gid)
	if value != nil {
		value2, _ := value.(*sync.Map).Load(model.ModelUnique())
		if value2 != nil {
			return value2.(*sync.Map)
		}
	}
	return nil
}

// setGlobalCache 将模型实例保存到全局缓存中。
// model 为要缓存的模型实例。
// 返回全局对象实例。
// 此操作会覆盖已存在的对象，除非该对象已被标记为删除。
// 覆盖操作会记录错误日志。
func setGlobalCache(model IModel) *globalObject {
	name := model.DataUnique()
	omap, _ := globalCacheMap.LoadOrStore(model.ModelUnique(), &sync.Map{})
	value, loaded := omap.(*sync.Map).LoadOrStore(name, &globalObject{})
	gobj := value.(*globalObject)
	if !loaded {
		gobj.ptr = model
	} else if gobj.ptr != model {
		gobj.ptr = model
		if gobj.delete {
			// 若是被标记为删除的数据，则不算覆盖
		} else {
			XLog.Error("XOrm.setGlobalCache: %v has been overwritten: %v", name, XLog.Caller(1, false))
		}
	}
	return gobj
}

// setSessionCache 将模型实例保存到会话缓存中。
// gid 为 goroutine ID。
// model 为要缓存的模型实例。
// meta 为模型的元信息。
// 返回会话对象实例。
// 此操作会覆盖已存在的对象，除非该对象已被标记为删除。
// 覆盖操作会记录错误日志。
// 会保存原始模型的克隆副本用于比较。
func setSessionCache(gid int64, model IModel, meta *modelInfo) *sessionObject {
	name := model.DataUnique()
	tmap, _ := sessionCacheMap.LoadOrStore(gid, &sync.Map{}) // 对应线程
	omap, _ := tmap.(*sync.Map).LoadOrStore(model.ModelUnique(), &sync.Map{})
	value, loaded := omap.(*sync.Map).LoadOrStore(name, &sessionObject{})
	sobj := value.(*sessionObject)
	if !loaded {
		sobj.ptr = model
		sobj.raw = model.Clone()
	} else if sobj.ptr != model {
		sobj.ptr = model
		sobj.raw = model.Clone()
		if sobj.delete {
			// 若是被标记为删除的数据，则不算覆盖
		} else {
			XLog.Error("XOrm.setSessionCache: %v has been overwritten: %v", name, XLog.Caller(1, false))
		}
	}
	sobj.model = meta
	return sobj
}

// isGlobalListed 检查或设置模型的全局列举状态。
// model 为模型实例。
// meta 为模型元信息。
// mark 指定是否标记为已列举。
// reset 指定是否重置列举状态。
// cond 为可选的查询条件。
// 返回模型是否已被列举。
// 只有启用缓存的模型才会被列举。
// 当 mark 为 true 且没有查询条件时，会标记模型为已列举。
func isGlobalListed(model IModel, meta *modelInfo, mark bool, reset bool, cond ...*condition) bool {
	if !meta.Cache {
		return false
	}
	// ISSUE：这里对 sync.Map 的操作不是原子性的，高并发环境是否会导致标记信息异常？
	local := false
	status, loaded := globalListMap.Load(model.ModelUnique())
	if loaded {
		local = status.(bool)
		if reset {
			globalListMap.Delete(model.ModelUnique())
		}
	} else {
		if mark && (len(cond) == 0 || cond[0] == nil) { // 标记该模型的数据已被列举过
			globalListMap.Store(model.ModelUnique(), true)
		}
	}
	return local
}

// isSessionListed 检查或设置模型在当前会话中的列举状态。
// gid 为 goroutine ID。
// model 为模型实例。
// mark 指定是否标记为已列举。
// reset 指定是否重置列举状态。
// cond 为可选的查询条件。
// 返回模型是否已被列举。
// 当 mark 为 true 且没有查询条件时，会标记模型为已列举。
// 会话列举状态与当前 goroutine 关联。
func isSessionListed(gid int64, model IModel, mark bool, reset bool, cond ...*condition) bool {
	// ISSUE：这里对 sync.Map 的操作不是原子性的，高并发环境是否会导致标记信息异常？
	var slist *sync.Map
	tmp, loaded := sessionListMap.Load(gid)
	if !loaded {
		tmp = &sync.Map{}
		sessionListMap.Store(gid, tmp)
	}
	slist = tmp.(*sync.Map)
	local := false
	status, loaded := slist.Load(model.ModelUnique())
	if loaded {
		local = status.(bool)
		if reset {
			slist.Delete(model.ModelUnique())
		}
	} else {
		if mark && (len(cond) == 0 || cond[0] == nil) { // 标记该模型的数据已被列举过
			slist.Store(model.ModelUnique(), true)
		}
	}
	return local
}

// concurrentRangeChunk 定义了并发遍历 sync.Map 时的最小键值对数量。
const concurrentRangeChunk = 100

// concurrentRange 并行遍历 sync.Map 中的所有键值对。
// 它将所有的键收集到一个切片中，然后按指定的 worker 数量将键分块，每个工作 goroutine 处理其中一部分。
// 如果回调函数返回 false，遍历会提前停止。
// 为了保证遍历过程的线程安全，该函数使用原子操作控制是否中止遍历。
//
//	data 为待遍历的 sync.Map。
//	process 为遍历时调用的回调函数，接受线程索引、键和值作为参数，如果回调函数返回 false，遍历会立即停止。
//	worker 会在遍历开始前返回并发 goroutine 的数量。
//	该函数没有返回值。它会通过并发执行来加速遍历操作，遍历结束后自动退出。
func concurrentRange(data *sync.Map, process func(index int, key, value any) bool, worker ...func(int)) {
	if data == nil || process == nil {
		return
	}

	var keys []any
	data.Range(func(key, value any) bool {
		keys = append(keys, key)
		return true
	})
	dataCount := len(keys)

	var workerCount = runtime.NumCPU()
	requiredCount := (dataCount + concurrentRangeChunk - 1) / concurrentRangeChunk
	if requiredCount < workerCount {
		workerCount = requiredCount
	}
	if len(worker) > 0 && worker[0] != nil {
		worker[0](workerCount)
	}

	chunkSize := dataCount / workerCount
	var wg sync.WaitGroup
	var done int32

	for i := range workerCount {
		wg.Add(1)
		XLoom.RunAsyncT1(func(workerID int) {
			defer wg.Done()

			// 每个 goroutine 处理一部分数据
			startIndex := workerID * chunkSize
			endIndex := (workerID + 1) * chunkSize
			if workerID == workerCount-1 {
				// 最后一个 goroutine 处理剩余的数据
				endIndex = len(keys)
			}

			for j := startIndex; j < endIndex; j++ {
				if atomic.LoadInt32(&done) == 1 {
					return
				}

				key := keys[j]
				value, _ := data.Load(key)

				if !process(workerID, key, value) {
					atomic.StoreInt32(&done, 1)
					return
				}
			}
		}, i)
	}
	wg.Wait()
}

// Dump 生成当前缓存状态的文本表示。
// 返回包含以下信息的字符串：
// - [Mem]: 全局内存中的对象
// - [List]: 全局列举状态
// - [Max]: 全局最大值记录
// 此函数会记录执行耗时。
func Dump() string {
	defer XLog.Elapse(0)()

	var ctt strings.Builder
	ctt.WriteString("[Mem]\n")
	globalCacheMap.Range(func(k1, v1 any) bool {
		v1.(*sync.Map).Range(func(k2, v2 any) bool {
			gobj := v2.(*globalObject)
			ctt.WriteString("\t")
			ctt.WriteString(k2.(string))
			ctt.WriteString(" = ")
			ctt.WriteString(gobj.ptr.Json())
			ctt.WriteString("\n")
			return true
		})
		return true
	})

	ctt.WriteString("[List]\n")
	globalListMap.Range(func(k, v any) bool {
		ctt.WriteString(fmt.Sprintf("\t%v = %v\n", k, v))
		return true
	})

	ctt.WriteString("[Max]\n")
	globalMaxMap.Range(func(k, v any) bool {
		cols := ""
		v.(*sync.Map).Range(func(k2, v2 any) bool {
			cols += fmt.Sprintf("%v-%v ", k2, v2)
			return true
		})
		ctt.WriteString(fmt.Sprintf("\t%v = %v\n", k, cols))
		return true
	})

	return ctt.String()
}
