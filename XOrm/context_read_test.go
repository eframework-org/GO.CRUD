// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"testing"
)

// TestRead 测试读取操作
func TestRead(t *testing.T) {
	tests := []struct {
		name     string
		cache    bool
		persist  bool
		writable bool
		prepare  func(t *testing.T) *TestBaseModel // 准备数据
		cond     func() *condition                 // 查询条件
	}{
		{
			name:     "ReadFromSessionMemory",
			cache:    true,
			persist:  true,
			writable: true,
			prepare: func(t *testing.T) *TestBaseModel {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.IntVal = 1
				model.FloatVal = 1.5
				model.StringVal = "session_memory"
				model.BoolVal = true
				Write(model)
				return model
			},
			cond: func() *condition {
				return Condition("int_val == {0}", 1)
			},
		},
		{
			name:     "ReadFromGlobalMemory",
			cache:    true,
			persist:  true,
			writable: false,
			prepare: func(t *testing.T) *TestBaseModel {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.IntVal = 2
				model.FloatVal = 2.5
				model.StringVal = "global_memory"
				model.BoolVal = true
				Write(model)
				return model
			},
			cond: func() *condition {
				return Condition("int_val == {0}", 2)
			},
		},
		{
			name:     "ReadFromDatabase",
			cache:    false,
			persist:  true,
			writable: true,
			prepare: func(t *testing.T) *TestBaseModel {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.IntVal = 3
				model.FloatVal = 3.5
				model.StringVal = "database"
				model.BoolVal = true
				Write(model)
				return model
			},
			cond: func() *condition {
				return Condition("int_val == {0}", 3)
			},
		},
		{
			name:     "ReadWithFuzzyCondition",
			cache:    true,
			persist:  true,
			writable: true,
			prepare: func(t *testing.T) *TestBaseModel {
				// 准备多条数据用于模糊查询
				for i := 4; i < 6; i++ {
					model := NewTestBaseModel()
					model.ID = Incre(model)
					model.IntVal = i
					model.FloatVal = float64(i) + 0.5
					model.StringVal = fmt.Sprintf("fuzzy_%d", i)
					model.BoolVal = true
					Write(model)
				}
				return nil
			},
			cond: func() *condition {
				return Condition("int_val > {0}", 4)
			},
		},
		{
			name:     "ReadDeletedData",
			cache:    true,
			persist:  true,
			writable: true,
			prepare: func(t *testing.T) *TestBaseModel {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.IntVal = 7
				model.FloatVal = 7.5
				model.StringVal = "deleted"
				model.BoolVal = true
				Write(model)
				Delete(model)
				return model
			},
			cond: func() *condition {
				return Condition("int_val == {0}", 7)
			},
		},
	}
	for i, tt := range tests {
		if i != 3 {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)

			Cleanup()
			Register(NewTestBaseModel(), tt.cache, tt.persist, tt.writable)

			Watch(true)
			// 准备数据
			model := tt.prepare(t)
			Defer() // 提交到数据库
			// 等待异步操作完成
			FlushNow()

			// 执行读取操作
			nmodel := NewTestBaseModel()
			result := Read(nmodel, tt.writable, tt.cond())

			switch tt.name {
			case "ReadFromSessionMemory", "ReadFromGlobalMemory", "ReadFromDatabase":
				if !result.IsValid() {
					t.Error("Should read valid data")
				}
				if result.IntVal != model.IntVal || result.StringVal != model.StringVal {
					t.Error("Read data mismatch")
				}
			case "ReadWithFuzzyCondition":
				if !result.IsValid() {
					t.Error("Should read valid data")
				}
				if result.IntVal <= 4 {
					t.Error("Fuzzy condition not working")
				}
			case "ReadDeletedData":
				if result.IsValid() {
					t.Error("Should not read deleted data")
				}
			}
		})
	}
}

// TestReadPriority 测试读取优先级
func TestReadPriority(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 准备测试数据
	Watch(true)
	Cleanup()
	Register(NewTestBaseModel(), true, true, true)

	// 写入数据到数据库
	model := NewTestBaseModel()
	model.ID = Incre(model)
	model.IntVal = 1
	model.FloatVal = 1.5
	model.StringVal = "original"
	model.BoolVal = true
	Write(model)
	Defer()

	// 等待异步操作完成
	FlushNow()

	// 测试会话内存优先级
	t.Run("ReadMemoryPriority", func(t *testing.T) {
		List(model)

		scache := getSessionCache(model)
		if scache != nil {
			scache.Range(func(key, value any) bool {
				sobj := value.(*sessionObject)
				if sobj.ptr.(*TestBaseModel).ID == 1 {
					sobj.ptr.(*TestBaseModel).StringVal = "session_modified"
				}
				return true
			})
		}
		// 读取数据
		nmodel := NewTestBaseModel()
		result := Read(nmodel, true, Condition("int_val == {0}", 1))

		if result.StringVal != "session_modified" {
			t.Error("Should read from session memory first")
		}

		gcache := getGlobalCache(model)
		if gcache != nil {
			gcache.Range(func(key, value any) bool {
				sobj := value.(*globalObject)
				if sobj.ptr.(*TestBaseModel).ID == 1 {
					sobj.ptr.(*TestBaseModel).StringVal = "global_modified"
				}
				return true
			})
		}
		// 读取数据
		nmodel = NewTestBaseModel()
		result = Read(nmodel, true, Condition("int_val == {0}", 1))
		if result.StringVal == "global_modified" {
			t.Error("Should read from session memory")
		}

		clearSessionCache(t)
		// 读取数据
		nmodel = NewTestBaseModel()
		result = Read(nmodel, true, Condition("int_val == {0}", 1))
		if result.StringVal != "global_modified" {
			t.Error("Should read from global memory when session memory not available", result.StringVal)
		}
	})
}
