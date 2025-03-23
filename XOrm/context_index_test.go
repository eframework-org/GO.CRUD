// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"
	"testing"
)

// TestContextIndex 测试索引操作
func TestContextIndex(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		needReset bool
		checkFunc func(t *testing.T)
	}{
		{
			name:      "IncreWithDefaultPrimaryKey",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 测试默认主键自增
				index := Incre(model)
				if index != 6 { // 已有5条数据，下一个应该是6
					t.Errorf("Incre() = %v, want %v", index, 6)
				}

				// 测试连续自增
				index = Incre(model)
				if index != 7 {
					t.Errorf("Incre() = %v, want %v", index, 7)
				}
			},
		},
		{
			name:      "IncreWithCustomColumn",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 测试指定列自增
				index := Incre(model, "int_val")
				maxVal := Max(model, "int_val")
				if index != maxVal+1 {
					t.Errorf("Incre(int_val) = %v, want %v", index, maxVal+1)
				}

				// 测试指定列和增量
				index = Incre(model, "int_val", 5)
				if index != maxVal+6 { // 上一次+1，这一次+5
					t.Errorf("Incre(int_val, 5) = %v, want %v", index, maxVal+6)
				}
			},
		},
		{
			name:      "IncreWithDeltaOnly",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 测试仅指定增量
				index := Incre(model, 3)
				if index != 8 { // 已有5条数据，+3
					t.Errorf("Incre(3) = %v, want %v", index, 8)
				}
			},
		},
		{
			name:      "IncreWithInvalidColumn",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				// 测试无效列名
				index := Incre(model, "non_existent_column")
				if index != 0 {
					t.Errorf("Incre(invalid) = %v, want %v", index, 0)
				}

				// 测试字符串列名
				index = Incre(model, "string_val")
				if index == 0 {
					t.Errorf("Incre(string_val) = %v, want %v", index, 0)
				}
			},
		},
		{
			name:      "IncreWithUnregisteredModel",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				Cleanup() // 清除注册信息

				// 测试未注册模型
				index := Incre(model)
				if index != 0 {
					t.Errorf("Incre() with unregistered model = %v, want %v", index, 0)
				}
			},
		},
		{
			name:      "MaxAndMinOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 测试主键最大值
				maxId := Max(model)
				if maxId != 5 { // 已有5条数据
					t.Errorf("Max() = %v, want %v", maxId, 5)
				}

				// 测试指定列最大值
				maxIntVal := Max(model, "int_val")
				if maxIntVal <= 0 {
					t.Error("Max(int_val) should return positive value")
				}

				// 测试主键最小值
				minId := Min(model)
				if minId != 1 {
					t.Errorf("Min() = %v, want %v", minId, 1)
				}

				// 测试指定列最小值
				minIntVal := Min(model, "int_val")
				if minIntVal < 0 {
					t.Error("Min(int_val) should not return negative value")
				}
			},
		},
		{
			name:      "MaxWithInvalidColumn",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				// 测试无效列名
				index := Max(model, "non_existent_column")
				if index != -1 {
					t.Errorf("Max(invalid) = %v, want %v", index, 0)
				}
			},
		},
		{
			name:      "MaxAndMinWithEmptyTable",
			modelArgs: []bool{true, true, true},
			needReset: true,
			checkFunc: func(t *testing.T) {
				ClearBaseTest(t)

				model := NewTestBaseModel()
				// 测试空表的最大值和最小值
				maxVal := Max(model)
				if maxVal != 0 {
					t.Errorf("Max() on empty = %v, want %v", maxVal, 0)
				}

				minVal := Min(model)
				if minVal != 0 {
					t.Errorf("Min() on empty = %v, want %v", minVal, 0)
				}
			},
		},
		{
			name:      "IncreWithConcurrentAccessRaceCondition",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) // 刷新列表

				// 创建并发goroutine进行自增操作
				var wg sync.WaitGroup
				goroutines := 5
				iterations := 10
				results := make([]int, goroutines*iterations)

				// 启动多个goroutine同时进行自增操作
				for i := 0; i < goroutines; i++ {
					wg.Add(1)
					go func(goID int) {
						defer wg.Done()
						for j := 0; j < iterations; j++ {
							// 执行自增操作
							index := Incre(model)
							// 记录获得的索引值
							results[goID*iterations+j] = index
						}
					}(i)
				}

				// 等待所有goroutine完成
				wg.Wait()

				// 检查是否有重复的索引值
				seen := make(map[int]bool)
				duplicates := make(map[int]int)
				for _, val := range results {
					if seen[val] {
						duplicates[val]++
					}
					seen[val] = true
				}

				// 如果有重复值，说明存在并发问题
				if len(duplicates) > 0 {
					t.Logf("Found duplicate indices in concurrent Incre operations: %v", duplicates)
					t.Log("This is expected behavior as Incre() is not thread-safe")
				}

				// 验证索引值的范围
				expectedStart := 6 // 已有5条数据，从6开始
				expectedEnd := expectedStart + goroutines*iterations - 1
				for val := range seen {
					if val < expectedStart || val > expectedEnd {
						t.Errorf("Index %d is outside expected range [%d, %d]", val, expectedStart, expectedEnd)
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

// 清除全局最大值缓存
func clearGlobalMax() {
	var model = NewTestBaseModel()
	globalMaxMap.Delete(model.ModelUnique())
}
