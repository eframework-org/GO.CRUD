// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"
)

// TestContextDelete 测试删除操作
func TestContextDelete(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		needReset bool
		checkFunc func(t *testing.T)
	}{
		{
			name:      "DeleteInSessionMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearGlobalCache(t)

				if !isSessionListed(model, false, false) {
					t.Error("session expected listed")
				}

				// 删除ID为3的数据
				model.ID = 3
				Delete(model)

				// 验证会话内存中的删除标记
				scache := getSessionCache(model)
				if scache != nil {
					sobj, exist := scache.Load(model.DataUnique())
					if !exist {
						t.Error("Deleted object should exist in session memory")
					} else if !sobj.(*sessionObject).delete {
						t.Error("Object should be marked as deleted in session memory")
					}
				}

				// 验证列举结果
				results := List(model)
				for _, r := range results {
					if r.ID == 3 {
						t.Error("Deleted object should not appear in list results")
					}
				}
				if len(results) != 4 {
					t.Errorf("List() after delete = %v, want 4", len(results))
				}
			},
		},
		{
			name:      "DeleteInGlobalMemory",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearSessionCache(t)

				if !isGlobalListed(model, getModelInfo(model), false, false) {
					t.Error("global expected listed")
				}

				// 删除ID为2的数据
				model.ID = 2
				Delete(model)

				// 验证全局内存中的删除标记
				gcache := getGlobalCache(model)
				if gcache != nil {
					gobj, exist := gcache.Load(model.DataUnique())
					if !exist {
						t.Error("Deleted object should exist in global memory")
					} else if !gobj.(*globalObject).delete {
						t.Error("Object should be marked as deleted in global memory")
					}
				}

				// 验证列举结果
				results := List(model)
				for _, r := range results {
					if r.ID == 2 {
						t.Error("Deleted object should not appear in list results")
					}
				}
				if len(results) != 4 {
					t.Errorf("List() after delete = %v, want 4", len(results))
				}
			},
		},
		{
			name:      "DeleteWithMemorySync",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 删除ID为1的数据
				model.ID = 1
				Delete(model)

				// 验证全局内存和会话内存的同步
				gcache := getGlobalCache(model)
				scache := getSessionCache(model)
				if gcache != nil && scache != nil {
					gobj, gexist := gcache.Load(model.DataUnique())
					sobj, sexist := scache.Load(model.DataUnique())

					if !gexist || !sexist {
						t.Error("Deleted object should exist in both memories")
					}
					if !gobj.(*globalObject).delete {
						t.Error("Object should be marked as deleted in global memory")
					}
					if !sobj.(*sessionObject).delete {
						t.Error("Object should be marked as deleted in session memory")
					}
				}
			},
		},
		{
			name:      "DeleteNonExistentObject",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 尝试删除不存在的数据
				model.ID = 999
				Delete(model)

				// 验证操作不会影响现有数据
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() after deleting non-existent object = %v, want 5", len(results))
				}
			},
		},
		{
			name:      "DeleteWithUnregisteredModel",
			modelArgs: []bool{true, true, true},
			needReset: false,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				Cleanup()   // 清除注册信息

				model.ID = 1
				// 删除操作应该不会导致panic
				Delete(model)

				// 重新注册并验证数据未被删除
				Register(NewTestBaseModel(), true, true, true)
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() after delete with unregistered model = %v, want 5", len(results))
				}
			},
		},
		{
			name:      "DeleteWithNonCachedModel",
			modelArgs: []bool{false, true, true},
			needReset: true,
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 删除ID为4的数据
				model.ID = 4
				Delete(model)

				// 验证会话内存中的删除标记
				scache := getSessionCache(model)
				if scache != nil {
					sobj, exist := scache.Load(model.DataUnique())
					if !exist {
						t.Error("Deleted object should exist in session memory")
					} else if !sobj.(*sessionObject).delete {
						t.Error("Object should be marked as deleted in session memory")
					}
				}

				// 验证全局内存中没有删除标记
				gcache := getGlobalCache(model)
				if gcache != nil {
					gobj, exist := gcache.Load(model.DataUnique())
					if !exist {
						t.Error("Deleted object should exist in global memory")
					} else if !gobj.(*globalObject).delete {
						t.Error("Object should be marked as deleted in global memory")
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
