// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"

	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XTime"
)

var (
	// globalLockMap 存储了所有模型的全局锁，键为模型唯一标识，值为对应的 WaitGroup。
	globalLockMap sync.Map // map[string]*sync.WaitGroup
)

// globalLock 对指定的数据模型加锁。model 参数为要加锁的数据模型，必须实现 IModel 接口。
// 函数会获取或创建模型的 WaitGroup，并增加一个等待计数。每次调用都会增加一个等待计数，
// 必须通过 globalUnlock 解锁。锁的粒度是模型级别的，同一模型的所有实例共享同一把锁。
func globalLock(model IModel) {
	wg, _ := globalLockMap.LoadOrStore(model.ModelUnique(), new(sync.WaitGroup))
	wg.(*sync.WaitGroup).Add(1)
}

// globalWait 等待指定数据模型的锁释放。source 参数为调用来源的标识，用于日志；
// model 参数为要等待的数据模型，必须实现 IModel 接口；meta 参数为模型的元数据信息。
//
// 函数首先检查模型配置，如果是仅缓存模式，则直接返回。然后获取模型的 WaitGroup，
// 如果存在，则开始等待并记录等待开始时间。等待完成后，函数会删除锁记录并记录等待耗时。
// 仅缓存模式的模型不需要等待，函数会记录详细的等待日志，等待完成后会自动清理锁。
func globalWait(source string, model IModel, meta *modelInfo) {
	if meta.Cache && !meta.Persist {
		return
	}
	if tmp, _ := globalLockMap.Load(model.ModelUnique()); tmp != nil {
		wg := tmp.(*sync.WaitGroup)
		t := XTime.GetMicrosecond()
		XLog.Notice("XOrm.globalWait: [%v] %v wait for unlock.", source, model.ModelUnique())
		wg.Wait()
		globalLockMap.Delete(model.ModelUnique())
		XLog.Notice("XOrm.globalWait: [%v] %v unlock cost %.2fms", source, model.ModelUnique(), float64(XTime.GetMicrosecond()-t)/1e3)
	}
}

// globalUnlock 解除指定数据模型的锁定。model 参数为要解锁的数据模型，必须实现 IModel 接口。
// 函数会获取模型的 WaitGroup，如果存在，则减少一个等待计数。必须与 globalLock 配对使用，
// 如果没有对应的锁，调用会被忽略。解锁后等待的操作会被唤醒。
func globalUnlock(model IModel) {
	if tmp, _ := globalLockMap.Load(model.ModelUnique()); tmp != nil {
		wg := tmp.(*sync.WaitGroup)
		wg.Done()
	}
}
