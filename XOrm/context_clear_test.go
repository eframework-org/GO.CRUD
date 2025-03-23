// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"
)

// TestContextClear 测试清除操作
func TestContextClear(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		needReset bool
		checkFunc func(t *testing.T)
	}{
		{
			name:      "ClearAllInSessionMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearGlobalCache(t)

				if !isSessionListed(model, false, false) {
					t.Error("session expected listed")
				}

				// 清除所有数据
				Clear(model)

				// 验证会话内存中的数据都被标记为删除
				scache := getSessionCache(model)
				if scache != nil {
					scache.Range(func(key, value any) bool {
						sobj := value.(*sessionObject)
						if !sobj.delete && !sobj.clear {
							t.Errorf("Object %v should be marked as deleted and cleared", key)
						}
						return true
					})
				}

				// 验证列举结果为空
				results := List(model)
				if len(results) != 0 {
					t.Errorf("List() after clear = %v, want empty list", len(results))
				}
			},
		},
		{
			name:      "ClearWithConditionInSessionMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearGlobalCache(t)

				// 清除 ID > 3 的数据
				Clear(model, Condition("id > {0}", 3))

				// 验证会话内存中的数据部分被标记为删除
				deletedCount := 0
				scache := getSessionCache(model)
				if scache != nil {
					scache.Range(func(key, value any) bool {
						sobj := value.(*sessionObject)
						if sobj.ptr.(*TestBaseModel).ID > 3 {
							if !sobj.delete && !sobj.clear {
								t.Errorf("Object %v should be marked as deleted and cleared", key)
							}
							deletedCount++
						} else {
							if sobj.delete || sobj.clear {
								t.Errorf("Object %v should not be marked as deleted or cleared", key)
							}
						}
						return true
					})
				}
				if deletedCount != 2 { // ID 4,5 应该被删除
					t.Errorf("Expected 2 objects to be deleted, got %v", deletedCount)
				}

				// 验证列举结果
				results := List(model)
				if len(results) != 3 { // 应该剩下 ID 1,2,3
					t.Errorf("List() after conditional clear = %v, want 3", len(results))
				}
			},
		},
		{
			name:      "ClearInGlobalMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearSessionCache(t)

				if !isGlobalListed(model, getModelInfo(model), false, false) {
					t.Error("global expected listed")
				}

				// 清除所有数据
				Clear(model)

				// 验证全局内存中的数据都被标记为删除
				gcache := getGlobalCache(model)
				if gcache != nil {
					gcache.Range(func(key, value any) bool {
						gobj := value.(*globalObject)
						if !gobj.delete {
							t.Errorf("Object %v should be marked as deleted", key)
						}
						return true
					})
				}

				// 验证列举结果为空
				results := List(model)
				if len(results) != 0 {
					t.Errorf("List() after clear = %v, want empty list", len(results))
				}
			},
		},
		{
			name:      "ClearWithConditionInGlobalMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearSessionCache(t)

				// 清除 ID <= 2 的数据
				Clear(model, Condition("id <= {0}", 2))

				// 验证全局内存中的数据部分被标记为删除
				gcache := getGlobalCache(model)
				deletedCount := 0
				if gcache != nil {
					gcache.Range(func(key, value any) bool {
						gobj := value.(*globalObject)
						if gobj.ptr.(*TestBaseModel).ID <= 2 {
							if !gobj.delete {
								t.Errorf("Object %v should be marked as deleted", key)
							}
							deletedCount++
						} else {
							if gobj.delete {
								t.Errorf("Object %v should not be marked as deleted", key)
							}
						}
						return true
					})
				}
				if deletedCount != 2 { // ID 1,2 应该被删除
					t.Errorf("Expected 2 objects to be deleted, got %v", deletedCount)
				}

				// 验证列举结果
				results := List(model)
				if len(results) != 3 { // 应该剩下 ID 3,4,5
					t.Errorf("List() after conditional clear = %v, want 3", len(results))
				}
			},
		},
		{
			name:      "ClearWithUnregisteredModel",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				Cleanup()   // 清除注册信息

				// 清除操作应该不会导致panic
				Clear(model)

				// 重新注册并验证数据未被清除
				Register(NewTestBaseModel(), true, true, true)
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() after clear with unregistered model = %v, want 5", len(results))
				}
			},
		},
		{
			name:      "ClearWithNonCachedModel",
			modelArgs: []bool{false, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 清除所有数据
				Clear(model)

				// 验证会话内存中的清除标记
				scache := getSessionCache(model)
				if scache != nil {
					found := false
					scache.Range(func(key, value any) bool {
						sobj := value.(*sessionObject)
						if sobj.clear {
							found = true
						}
						return !found
					})
					if !found {
						t.Error("Clear flag should be set in session memory")
					}
				}
			},
		},
	}

	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)
	PrepareTestData(t, 5)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Cleanup()
			Register(NewTestBaseModel(), tt.modelArgs[0], tt.modelArgs[1], tt.modelArgs[2])
			ResetAllResource(t)

			tt.checkFunc(t)
			if tt.needReset {
				ClearBaseTest(t)
				PrepareTestData(t, 5)
			}
		})
	}
}
