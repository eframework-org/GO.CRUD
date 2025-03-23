// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"
)

// TestContextCount 测试计数操作
func TestContextCount(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		checkFunc func(t *testing.T)
	}{
		{
			name:      "CountSimpleInGlobal",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) //刷新列表
				clearSessionCache(t)

				if !isGlobalListed(model, getModelInfo(NewTestBaseModel()), false, false) {
					t.Error("global expected listed")
				}
				count := Count(model)
				if count != 5 {
					t.Errorf("Count() = %v, want %v", count, 5)
				}
				count1 := Count(model, Condition("id > {0}", 1))
				if count1 != 4 {
					t.Errorf("Count() = %v, want %v", count1, 4)
				}
				count2 := Count(model, Condition("id == {0}", 1))
				if count2 != 1 {
					t.Errorf("Count() = %v, want %v", count2, 1)
				}
			},
		},
		{
			name:      "CountSimpleInSession",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				List(model) //刷新列表
				clearGlobalCache(t)
				if !isSessionListed(model, false, false) {
					t.Errorf("session expected listed")
				}
				count := Count(model)
				if count != 5 {
					t.Errorf("Count() = %v, want %v", count, 5)
				}
				count1 := Count(model, Condition("id > {0}", 1))
				if count1 != 4 {
					t.Errorf("Count() = %v, want %v", count1, 4)
				}
				count2 := Count(model, Condition("id == {0}", 1))
				if count2 != 1 {
					t.Errorf("Count() = %v, want %v", count2, 1)
				}
			},
		},
		{
			name:      "CountSimpleInDb",
			modelArgs: []bool{false, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				count := Count(model)
				if count != 5 {
					t.Errorf("Count() = %v, want %v", count, 1)
				}
				count1 := Count(model, Condition("id > {0}", 1))
				if count1 != 4 {
					t.Errorf("Count() = %v, want %v", count1, 4)
				}
				count2 := Count(model, Condition("id == {0}", 1))
				if count2 != 1 {
					t.Errorf("Count() = %v, want %v", count2, 1)
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
