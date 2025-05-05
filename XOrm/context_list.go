// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"runtime"
	"sync"

	"github.com/eframework-org/GO.UTIL/XCollect"
	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XLoom"
	"github.com/petermattis/goid"
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
	cacheDumpWait.Wait()

	gid := goid.Get()
	frets := make([]T, 0)
	meta := getModelMeta(model)
	if meta == nil {
		XLog.Critical("XOrm.List: model of %v was not registered: %v", model.ModelUnique(), XLog.Caller(1, false))
	} else {
		writable := meta.writable
		var cond *Condition
		for _, v := range writableAndCond {
			switch nv := v.(type) {
			case bool:
				writable = nv
			case *Condition:
				cond = nv
			default:
				XLog.Critical("XOrm.List: writableAndCond of %v type is error: %v", v, XLog.Caller(1, false))
			}
		}
		var slisted = isSessionListed(gid, model)
		var glisted = isGlobalListed(model)
		if slisted { // 会话内存读取
			scache := getSessionCache(gid, model)
			if scache != nil {
				var chunks [][]T
				concurrentRange(scache, func(index int, key, value any) bool {
					sobj := value.(*sessionObject)
					if !sobj.ptr.IsValid() { // 忽略无效数据
					} else if sobj.ptr.Matchs(cond) {
						sobj.writable(writable)
						chunks[index] = append(chunks[index], sobj.ptr.(T))
					}
					return true
				}, func(worker int) { chunks = make([][]T, worker) })
				for _, chunk := range chunks {
					frets = append(frets, chunk...)
				}
			}
		} else if glisted { // 全局内存读取
			gcache := getGlobalCache(model)
			scache := getSessionCache(gid, model)
			if gcache != nil {
				var chunks [][]T
				concurrentRange(gcache, func(index int, key, value any) bool {
					gobj := value.(IModel)
					if !gobj.IsValid() {
						// 已经被标记删除，则不读取
					} else if gobj.Matchs(cond) {
						ele := gobj.Clone() // 内存拷贝
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
							sobj = setSessionCache(gid, ele) // 监控内存
						}
						sobj.writable(writable)
						chunks[index] = append(chunks[index], ele.(T))
					}
					return true
				}, func(worker int) { chunks = make([][]T, worker) })
				for _, chunk := range chunks {
					frets = append(frets, chunk...)
				}
			}
		} else { // 远端读取
			globalWait("XOrm.List", model)
			model.List(&frets, cond)
			if len(frets) > 0 {
				gcache := getGlobalCache(model)
				scache := getSessionCache(gid, model)

				// 多线程处理数据
				dataCount := len(frets)
				var workerCount = runtime.NumCPU()
				requiredCount := (dataCount + concurrentRangeChunk - 1) / concurrentRangeChunk
				if requiredCount < workerCount {
					workerCount = requiredCount
				}
				chunkSize := dataCount / workerCount
				valids := make([]string, 0)
				var validsMu sync.Mutex
				invalids := make(map[int]struct{})
				var invalidsMu sync.Mutex
				var wg sync.WaitGroup
				for i := range workerCount {
					wg.Add(1)
					XLoom.RunAsyncT1(func(workerID int) {
						defer wg.Done()

						// 每个 goroutine 处理一部分数据
						startIndex := workerID * chunkSize
						endIndex := (workerID + 1) * chunkSize
						if workerID == workerCount-1 {
							// 最后一个 goroutine 处理剩余的数据
							endIndex = dataCount
						}

						for j := startIndex; j < endIndex; j++ {
							removed := false
							obj := frets[j]
							name := obj.DataUnique()
							var gobj IModel
							var sobj *sessionObject
							if meta.cache {
								if gcache != nil { // 全局内存读取
									t, _ := gcache.Load(name)
									if t != nil {
										gobj = t.(IModel)
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
								if !sobj.ptr.IsValid() { // 忽略无效数据
									// 已经被标记删除，则不读取
									removed = true
									invalidsMu.Lock()
									invalids[j] = struct{}{}
									invalidsMu.Unlock()
									XLog.Notice("XOrm.List: session object is marked as invalid or deleted: %v", name)
								} else {
									frets[j] = sobj.ptr.(T)
									isSCache = true
									XLog.Notice("XOrm.List: using global object: %v", name)
								}
							} else if gobj != nil && !isSCache { // 已经在全局内存中，但不在会话内存中
								if !gobj.IsValid() {
									// 已经被标记删除，则不读取
									removed = true
									invalidsMu.Lock()
									invalids[j] = struct{}{}
									invalidsMu.Unlock()
									XLog.Notice("XOrm.List: global object is marked as invalid: %v", name)
								} else {
									nobj := gobj.Clone() // 内存拷贝
									frets[j] = nobj.(T)
									sobj := setSessionCache(gid, nobj) // 监控内存
									sobj.writable(writable)
									XLog.Notice("XOrm.List: using global object: %v", name)
								}
							} else { // 既不在会话内存中，也不在全局内存中
								if meta.cache {
									setGlobalCache(obj.Clone()) // 内存拷贝
								}
								sobj := setSessionCache(gid, obj) // 监控内存
								sobj.writable(writable)
							}
							if !removed {
								validsMu.Lock()
								valids = append(valids, name)
								validsMu.Unlock()
							}
						}
					}, i)
				}
				wg.Wait()

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
					var chunks [][]T
					var chunkValids [][]string
					concurrentRange(scache, func(index int, key, value any) bool {
						skey := key.(string)
						sobj := value.(*sessionObject)
						if !sobj.ptr.IsValid() { // 忽略无效数据
						} else if !XCollect.Contains(valids, skey) && !XCollect.Contains(chunkValids[index], skey) {
							if sobj.ptr.Matchs(cond) {
								chunkValids[index] = append(chunkValids[index], skey) // 在会话内存中，但是不在远端的，且满足筛选条件的，亦加入frets中
								chunks[index] = append(chunks[index], sobj.ptr.(T))
								XLog.Notice("XOrm.List: add session object: %v", skey)
							}
						}
						return true
					}, func(worker int) {
						chunks = make([][]T, worker)
						chunkValids = make([][]string, worker)
					})
					for _, chunk := range chunks {
						frets = append(frets, chunk...)
					}
					for _, chunkValid := range chunkValids {
						valids = append(valids, chunkValid...)
					}
				}
				if meta.cache {
					if gcache != nil { // 遍历全局内存
						added := new(sync.Map)

						concurrentRange(gcache, func(index int, key, value any) bool {
							gkey := key.(string)
							gobj := value.(IModel)
							if !gobj.IsValid() {
							} else if !XCollect.Contains(valids, gkey) {
								if gobj.Matchs(cond) {
									added.Store(gkey, gobj)
								}
							}
							return true
						})

						added.Range(func(key, value any) bool {
							gkey := key.(string)
							gobj := value.(IModel)
							valids = append(valids, gkey)      // 在全局内存中，但是不在远端的，且满足筛选条件的，亦加入frets中
							nobj := gobj.Clone()               // 内存拷贝
							sobj := setSessionCache(gid, nobj) // 监控内存
							sobj.writable(writable)
							frets = append(frets, nobj.(T))
							XLog.Notice("XOrm.List: add global object: %v", gkey)
							return true
						})
					}
				}
			}
		}

		if cond == nil {
			if !slisted {
				isSessionListed(gid, model, true)
			}
			if !glisted && meta.cache {
				isGlobalListed(model, true)
			}
		}
	}

	return frets
}
