// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"testing"

	"github.com/petermattis/goid"
)

// TestContext 测试上下文操作
func TestContext(t *testing.T) {
	tests := []struct {
		name         string
		writable     bool
		wantWritable bool
		wantSess     bool
	}{
		{
			name:         "DefaultReadWriteContext",
			writable:     true,
			wantWritable: true,
			wantSess:     true,
		},
		{
			name:         "ReadOnlyContext",
			writable:     false,
			wantWritable: false,
			wantSess:     true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)

			// 测试Watch
			sid := Watch(tt.writable)
			if sid <= 0 {
				t.Error("Watch should return positive session ID")
			}

			// 获取当前上下文
			gid := goid.Get()
			val, ok := contextMap.Load(gid)
			if !ok {
				t.Error("Context should be stored in contextMap")
			}

			ctx := val.(*context)
			if ctx.writable != tt.wantWritable {
				t.Errorf("Context writable = %v, want %v", ctx.writable, tt.wantWritable)
			}
			if ctx.time <= 0 {
				t.Error("Context time should be positive")
			}

			// 测试会话存储
			model := NewTestBaseModel()
			model.ID = Incre(model)
			model.IntVal = i
			model.FloatVal = float64(i) + 0.5
			model.StringVal = fmt.Sprintf("test_string_%d", i)
			model.BoolVal = true
			Cleanup()
			Register(model, true, true, true)
			Write(model)

			// 验证会话存储
			if watchs, exists := sessionCacheMap.Load(gid); exists != tt.wantSess {
				t.Errorf("Session storage exists = %v, want %v", exists, tt.wantSess)
			} else if exists {
				if watchs == nil {
					t.Error("Session storage should not be nil")
				}
			}

			// 测试Defer
			Defer()

			// 验证上下文已清理
			if _, exists := contextMap.Load(gid); exists {
				t.Error("Context should be removed after Defer")
			}

			// 验证会话存储已清理
			if _, exists := sessionCacheMap.Load(gid); exists {
				t.Error("Session storage should be removed after Defer")
			}

			// 等待异步操作完成
			FlushNow()

			// 验证数据写入结果（仅对读写上下文）
			if tt.wantWritable {
				nmodel := NewTestBaseModel()
				if !nmodel.Read(Condition("int_val == {0}", i)) {
					t.Error("Data should be written for read-write context")
				}
			}
		})
	}
}

// TestContextConsecutive 测试连续的Watch和Defer操作
func TestContextConsecutive(t *testing.T) {
	t.Run("ConsecutiveContextInDifferentBatch", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)

		sid1 := Watch(true)
		if sid1 <= 0 {
			t.Error("First Watch should return positive session ID")
		}

		model1 := NewTestBaseModel()
		model1.ID = Incre(model1)
		model1.IntVal = 1
		model1.FloatVal = 1.5
		model1.StringVal = "first_context"
		model1.BoolVal = true

		Write(model1)
		Defer()

		sid2 := Watch(true)
		if sid2 <= 0 {
			t.Error("Second Watch should return positive session ID")
		}

		model2 := NewTestBaseModel()
		model2.ID = Incre(model2)
		model2.IntVal = 2
		model2.FloatVal = 1.5
		model2.StringVal = "second_context"
		model2.BoolVal = true

		Write(model2)
		Defer()

		// 等待第一个上下文的异步操作完成
		FlushNow()

		// 验证第一个上下文的数据写入
		nmodel := NewTestBaseModel()
		if !nmodel.Read(Condition("int_val == {0}", 1)) {
			t.Error("Data from first context should be written")
		}
		// 验证第一个上下文的数据写入
		nmodel2 := NewTestBaseModel()
		if !nmodel2.Read(Condition("int_val == {0}", 2)) {
			t.Error("Data from second context should be written")
		}
	})

	t.Run("ConsecutiveContextInOneBatch", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)

		defer func() {
			Defer()
			FlushNow()
			nmodel := NewTestBaseModel()
			if !nmodel.Read(Condition("int_val == {0}", 1)) {
				t.Error("Data from first context should be written")
			}
			nmodel2 := NewTestBaseModel()
			if !nmodel2.Read(Condition("int_val == {0}", 2)) {
				t.Error("Data from second context should be written")
			}
		}()

		sid1 := Watch(true)
		if sid1 <= 0 {
			t.Error("First Watch should return positive session ID")
		}
		model1 := NewTestBaseModel()
		model1.ID = Incre(model1)
		model1.IntVal = 1
		model1.FloatVal = 1.5
		model1.StringVal = "first_context"
		model1.BoolVal = true
		Write(model1)

		sid2 := Watch(true)
		if sid2 <= 0 {
			t.Error("Second Watch should return positive session ID")
		}
		model2 := NewTestBaseModel()
		model2.ID = Incre(model2)
		model2.IntVal = 2
		model2.FloatVal = 1.5
		model2.StringVal = "second_context"
		model2.BoolVal = true
		Write(model2)
	})
}

// 重置所有资源
func ResetAllResource(t *testing.T) {
	clearGlobalCache(t)
	clearSessionCache(t)
	clearGlobalMax()
	clearCommitMap()
	clearLock()
}
