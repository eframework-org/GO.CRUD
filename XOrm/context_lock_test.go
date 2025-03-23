// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"
	"testing"
	"time"
)

// TestGlobalLock 测试全局锁操作
func TestGlobalLock(t *testing.T) {
	t.Run("LockGlobal", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)

		Cleanup()
		Register(NewTestBaseModel(), true, true, true)

		model := NewTestBaseModel()
		model.IntVal = 1
		model.FloatVal = 1.5
		model.StringVal = "test_lock"
		model.BoolVal = true

		// 测试加锁
		globalLock(model)

		// 验证锁是否存在
		if tmp, exists := globalLockMap.Load(model.ModelUnique()); !exists {
			t.Error("Lock should be created")
		} else if tmp == nil {
			t.Error("Lock should not be nil")
		}

		// 测试解锁
		globalUnlock(model)

		// 验证锁是否被正确释放
		if tmp, exists := globalLockMap.Load(model.ModelUnique()); exists {
			// 尝试立即加锁，如果能加锁说明之前的锁已经被释放
			done := make(chan bool)
			go func() {
				wg := tmp.(*sync.WaitGroup)
				wg.Add(1)
				defer wg.Done()
				done <- true
			}()
			select {
			case <-done:
				// 成功加锁，符合预期
			case <-time.After(100 * time.Millisecond):
				t.Error("Lock should be released")
			}
		}
	})
}

// TestGlobalWait 测试全局等待操作
func TestGlobalWait(t *testing.T) {
	tests := []struct {
		name       string
		cache      bool
		persist    bool
		writable   bool
		shouldWait bool
	}{
		{
			name:       "WaitWithCacheAndPersist",
			cache:      true,
			persist:    true,
			writable:   true,
			shouldWait: true,
		},
		{
			name:       "NoWaitWithOnlyCache",
			cache:      true,
			persist:    false,
			writable:   true,
			shouldWait: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)
			Cleanup()
			Register(NewTestBaseModel(), tt.cache, tt.persist, tt.writable)

			meta := &modelInfo{Cache: tt.cache, Persist: tt.persist, Writable: tt.writable}

			model := NewTestBaseModel()
			model.IntVal = 1
			model.FloatVal = 1.5
			model.StringVal = "test_wait"
			model.BoolVal = true

			if tt.shouldWait {
				// 创建一个锁定状态
				globalLock(model)

				// 在另一个goroutine中等待一段时间后解锁
				go func() {
					time.Sleep(100 * time.Millisecond)
					globalUnlock(model)
				}()

				// 记录等待开始时间
				start := time.Now()

				// 执行等待
				globalWait("TestGlobalWait", model, meta)

				// 验证是否真的等待了
				if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
					t.Error("Should wait for lock to be released")
				}

				// 验证锁是否被清理
				if _, exists := globalLockMap.Load(model.ModelUnique()); exists {
					t.Error("Lock should be cleaned up after wait")
				}
			} else {
				// 对于不需要等待的情况，验证是否立即返回
				start := time.Now()
				globalWait("TestGlobalWait", model, meta)
				if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
					t.Error("Should return immediately without waiting")
				}
			}
		})
	}
}

func clearLock() {
	globalLockMap = sync.Map{}
}
