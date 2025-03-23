// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"strings"
	"testing"
)

// TestContextCache 测试缓存操作
func TestContextCache(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		checkFunc func(t *testing.T)
	}{
		{
			name:      "GlobalMemoryCacheOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				model.ID = 1
				model.StringVal = "test_global"

				// 测试全局内存缓存
				gobj := setGlobalCache(model)
				if gobj == nil {
					t.Error("Global object should not be nil after cache")
				}

				// 验证缓存的数据
				gcache := getGlobalCache(model)
				if gcache == nil {
					t.Error("Global memory should not be nil")
				}

				value, exist := gcache.Load(model.DataUnique())
				if !exist {
					t.Error("Cached object should exist in global memory")
				}
				if value.(*globalObject).ptr != model {
					t.Error("Cached object should reference the original model")
				}

				// 测试重复缓存相同对象
				gobj2 := setGlobalCache(model)
				if gobj2 != gobj {
					t.Error("Caching same object should return same global object")
				}

				// 测试缓存不同对象但相同标识
				model2 := NewTestBaseModel()
				model2.ID = 1
				model2.StringVal = "test_global_2"
				gobj3 := setGlobalCache(model2)
				if gobj3 != gobj {
					t.Error("Caching object with same unique ID should update existing object")
				}
				if gobj3.ptr != model2 {
					t.Error("Cached object should be updated to new model")
				}
			},
		},
		{
			name:      "SessionMemoryCacheOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				model.ID = 2
				model.StringVal = "test_session"
				meta := getModelInfo(model)

				// 测试会话内存缓存
				sobj := setSessionCache(model, meta)
				if sobj == nil {
					t.Error("Session object should not be nil after cache")
				}

				// 验证缓存的数据
				scache := getSessionCache(model)
				if scache == nil {
					t.Error("Session memory should not be nil")
				}

				value, exist := scache.Load(model.DataUnique())
				if !exist {
					t.Error("Cached object should exist in session memory")
				}
				if value.(*sessionObject).ptr != model {
					t.Error("Cached object should reference the original model")
				}
				if value.(*sessionObject).model != meta {
					t.Error("Cached object should have correct model info")
				}

				// 验证原始数据的克隆
				if value.(*sessionObject).raw.Json() != model.Json() {
					t.Error("Raw object should be a clone of original model")
				}

				// 测试重复缓存相同对象
				sobj2 := setSessionCache(model, meta)
				if sobj2 != sobj {
					t.Error("Caching same object should return same session object")
				}

				// 测试缓存不同对象但相同标识
				model2 := NewTestBaseModel()
				model2.ID = 2
				model2.StringVal = "test_session_2"
				sobj3 := setSessionCache(model2, meta)
				if sobj3 != sobj {
					t.Error("Caching object with same unique ID should update existing object")
				}
				if sobj3.ptr != model2 {
					t.Error("Cached object should be updated to new model")
				}
			},
		},
		{
			name:      "SessionObjectReadWriteFlags",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				model.ID = 3
				meta := getModelInfo(model)

				sobj := setSessionCache(model, meta)

				// 测试默认状态
				if sobj.setWritable() != 0 {
					t.Error("Initial read/write flag should be 0")
				}

				// 测试设置只读标记
				sobj.setWritable(false, true)
				if sobj.setWritable() != 1 {
					t.Error("Read-only flag should be 1")
				}

				// 测试设置读写标记
				sobj.setWritable(true, true)
				if sobj.setWritable() != 2 {
					t.Error("Read-write flag should be 2")
				}

				// 测试无法从读写降级到只读
				sobj.setWritable(false, true)
				if sobj.setWritable() != 2 {
					t.Error("Cannot downgrade from read-write to read-only")
				}
			},
		},
		{
			name:      "GlobalListedStatus",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				meta := getModelInfo(model)

				// 测试初始状态
				if isGlobalListed(model, meta, false, false) {
					t.Error("Initial listed status should be false")
				}

				// 测试标记为已列举
				isGlobalListed(model, meta, true, false)
				if !isGlobalListed(model, meta, false, false) {
					t.Error("Listed status should be true after marking")
				}

				// 测试重置状态
				isGlobalListed(model, meta, false, true)
				if isGlobalListed(model, meta, false, false) {
					t.Error("Listed status should be false after reset")
				}

				// 测试非缓存模型
				meta.Cache = false
				if isGlobalListed(model, meta, true, false) {
					t.Error("Non-cached model should always return false")
				}
			},
		},
		{
			name:      "SessionListedStatus",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()

				// 测试初始状态
				if isSessionListed(model, false, false) {
					t.Error("Initial listed status should be false")
				}

				// 测试标记为已列举
				isSessionListed(model, true, false)
				if !isSessionListed(model, false, false) {
					t.Error("Listed status should be true after marking")
				}

				// 测试重置状态
				isSessionListed(model, false, true)
				if isSessionListed(model, false, false) {
					t.Error("Listed status should be false after reset")
				}
			},
		},
		{
			name:      "MemoryDumpFunctionality",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				// 准备测试数据
				model := NewTestBaseModel()
				model.ID = 1
				model.StringVal = "test_dump"
				setGlobalCache(model)

				// 执行内存转储
				dump := Dump()

				// 验证转储内容
				if !strings.Contains(dump, "test_dump") {
					t.Error("Dump should contain cached object data")
				}
				if !strings.Contains(dump, "[Mem]") {
					t.Error("Dump should contain memory section")
				}
				if !strings.Contains(dump, "[List]") {
					t.Error("Dump should contain list section")
				}
				if !strings.Contains(dump, "[Max]") {
					t.Error("Dump should contain max section")
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
