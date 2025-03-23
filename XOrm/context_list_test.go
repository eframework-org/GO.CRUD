// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"

	"github.com/beego/beego/v2/client/orm"
	"github.com/petermattis/goid"
)

// TestContextList 测试列举操作
func TestContextList(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		checkFunc func(t *testing.T)
	}{
		{
			name:      "ListFromGlobalMemory",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearSessionCache(t)
				if !isGlobalListed(model, getModelInfo(model), false, false) {
					t.Error("global expected listed")
				}

				// 测试无条件列举
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() length = %v, want %v", len(results), 5)
				}
				if !isSessionListed(model, false, false) {
					t.Error("session expected listed")
				}
				clearSessionCache(t) //需再次清除
				// 测试条件列举
				results = List(model, Condition("id > {0}", 2))
				if len(results) != 3 {
					t.Errorf("List() with condition length = %v, want %v", len(results), 3)
				}

				results = List(model, true)
				for _, r := range results {
					if sobj, exists := getSessionCache(r).Load(r.DataUnique()); exists {
						if sobj.(*sessionObject).writable != 2 {
							t.Error("List() with writable flag should mark objects as writable")
						}
					}
				}
			},
		},
		{
			name:      "ListFromSessionMemory",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表
				clearGlobalCache(t)
				if !isSessionListed(model, false, false) {
					t.Error("session expected listed")
				}

				// 测试无条件列举
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() length = %v, want %v", len(results), 5)
				}
				clearGlobalCache(t)
				// 测试条件列举
				results = List(model, Condition("id > {0}", 2))
				if len(results) != 3 {
					t.Errorf("List() with condition length = %v, want %v", len(results), 3)
				}
				clearGlobalCache(t)
				// 测试带删除标记的列举
				scache := getSessionCache(model)
				if scache != nil {
					scache.Range(func(key, value any) bool {
						if sobj := value.(*sessionObject); sobj != nil {
							if sobj.ptr.(*TestBaseModel).ID == 1 {
								sobj.delete = true
							}
						}
						return true
					})
				}
				results = List(model)
				if len(results) != 4 {
					t.Errorf("List() after delete length = %v, want %v", len(results), 4)
				}
			},
		},
		{
			name:      "ListFromDatabase",
			modelArgs: []bool{false, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()

				// 测试无条件列举
				results := List(model)
				if len(results) != 5 {
					t.Errorf("List() length = %v, want %v", len(results), 5)
				}

				// 测试条件列举
				results = List(model, Condition("id <= {0}", 3))
				if len(results) != 3 {
					t.Errorf("List() with condition length = %v, want %v", len(results), 3)
				}

				// 验证返回对象的有效性
				for _, r := range results {
					if !r.IsValid() {
						t.Error("List() should return valid objects")
					}
				}

				// 使用原生查询原语
				results = List(model, Condition(orm.NewCondition().And("int_val__in", []int{1, 2})))
				if len(results) != 2 {
					t.Errorf("List() with condition length = %v, want %v", len(results), 2)
				}
			},
		},
		{
			name:      "ListWithMixedOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 初始加载

				// 在会话内存中修改一个对象
				results := List(model)
				if len(results) > 0 {
					modifiedModel := results[0]
					modifiedModel.StringVal = "modified"
					setSessionCache(modifiedModel, getModelInfo(model))
				}

				// 在全局内存中标记删除另一个对象
				if len(results) > 1 {
					deletedModel := results[1]
					if gobj, exists := getGlobalCache(model).Load(deletedModel.DataUnique()); exists {
						gobj.(*globalObject).delete = true
					}
				}

				// 重新列举并验证结果
				newResults := List(model)
				foundModified := false
				for _, r := range newResults {
					if r.StringVal == "modified" {
						foundModified = true
						break
					}
				}
				if !foundModified {
					t.Error("List() should return modified object from session memory")
				}
				clearSessionCache(t)
				newResults = List(model)
				if len(newResults) != 4 {
					t.Errorf("List() after mixed operations length = %v, want %v from global memory", len(newResults), 4)
				}
			},
		},
		{
			name:      "ListWithInvalidModel",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				Cleanup() // 清除注册信息使模型无效

				results := List(model)
				if len(results) != 0 {
					t.Errorf("List() with invalid model length = %v, want %v", len(results), 0)
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
		})
	}
}

// 清理全局缓存
func clearGlobalCache(t *testing.T) {
	var model = NewTestBaseModel()
	globalCacheMap.Delete(model.ModelUnique()) // release global memory
	globalListMap.Delete(model.ModelUnique())  // release global memory

	if isGlobalListed(model, getModelInfo(model), false, false) {
		t.Error("global expected not listed")
	}
}

// 清理绘画缓存
func clearSessionCache(t *testing.T) {
	gid := goid.Get()
	sessionCacheMap.Delete(gid) // release memory
	sessionListMap.Delete(gid)  // release memory
	var model = NewTestBaseModel()
	if isSessionListed(model, false, false) {
		t.Error("session expected not listed")
	}
}
