// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/petermattis/goid"
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
// model 为模型实例。
// 返回对象映射，如果不存在则返回 nil。
func getSessionCache(model IModel) *sync.Map {
	value, _ := sessionCacheMap.Load(goid.Get())
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
	omap, _ := globalCacheMap.Load(model.ModelUnique())
	if omap == nil {
		omap = &sync.Map{}
		globalCacheMap.Store(model.ModelUnique(), omap)
	}
	oomap := omap.(*sync.Map)
	value, _ := oomap.Load(name)
	var gobj *globalObject
	if value != nil {
		gobj = value.(*globalObject)
	}
	if gobj == nil {
		gobj = new(globalObject)
		oomap.Store(name, gobj)
		gobj.ptr = model
	} else if gobj.ptr != model {
		gobj.ptr = model
		if gobj.delete {
			// 若是被标记为删除的数据，则不算覆盖
		} else {
			XLog.Error("XOrm.globalCache: %v has been overwrited: %v", name, XLog.Caller(1, false))
		}
	}
	return gobj
}

// setSessionCache 将模型实例保存到会话缓存中。
// model 为要缓存的模型实例。
// meta 为模型的元信息。
// 返回会话对象实例。
// 此操作会覆盖已存在的对象，除非该对象已被标记为删除。
// 覆盖操作会记录错误日志。
// 会保存原始模型的克隆副本用于比较。
func setSessionCache(model IModel, meta *modelInfo) *sessionObject {
	name := model.DataUnique()
	tmap, _ := sessionCacheMap.Load(goid.Get()) // 对应线程
	if tmap == nil {
		tmap = &sync.Map{}
		sessionCacheMap.Store(goid.Get(), tmap)
	}
	omap, _ := tmap.(*sync.Map).Load(model.ModelUnique())
	if omap == nil {
		omap = &sync.Map{}
		tmap.(*sync.Map).Store(model.ModelUnique(), omap)
	}
	oomap := omap.(*sync.Map)
	value, _ := oomap.Load(name)
	var sobj *sessionObject
	if value != nil {
		sobj = value.(*sessionObject)
	}
	if sobj == nil {
		sobj = new(sessionObject)
		oomap.Store(name, sobj)
		sobj.ptr = model
		sobj.raw = model.Clone()
	} else if sobj.ptr != model {
		sobj.ptr = model
		sobj.raw = model.Clone()
		if sobj.delete {
			// 若是被标记为删除的数据，则不算覆盖
		} else {
			XLog.Error("XOrm.sessionCache: %v has been overwrited: %v", name, XLog.Caller(1, false))
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
	local := false
	status, exist := globalListMap.Load(model.ModelUnique())
	if exist {
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
// model 为模型实例。
// mark 指定是否标记为已列举。
// reset 指定是否重置列举状态。
// cond 为可选的查询条件。
// 返回模型是否已被列举。
// 当 mark 为 true 且没有查询条件时，会标记模型为已列举。
// 会话列举状态与当前 goroutine 关联。
func isSessionListed(model IModel, mark bool, reset bool, cond ...*condition) bool {
	var slist *sync.Map
	tmp, exist := sessionListMap.Load(goid.Get())
	if !exist {
		tmp = &sync.Map{}
		sessionListMap.Store(goid.Get(), tmp)
	}
	slist = tmp.(*sync.Map)
	local := false
	status, exist := slist.Load(model.ModelUnique())
	if exist {
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
