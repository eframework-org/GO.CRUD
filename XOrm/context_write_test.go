// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"testing"
)

// TestContextWrite 测试写入操作
func TestContextWrite(t *testing.T) {
	tests := []struct {
		name   string
		args   []bool
		expect bool
	}{
		{
			name:   "WriteCacheModel",
			args:   []bool{true, true, true},
			expect: true,
		},
		{
			name:   "WriteReadWriteModel",
			args:   []bool{false, true, true},
			expect: true,
		},
		{
			name:   "WriteOnlyReadModel",
			args:   []bool{false, true, false},
			expect: false,
		},
		{
			name:   "WriteNoPersistModel",
			args:   []bool{false, false, false},
			expect: false,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)

			Cleanup()
			Register(NewTestBaseModel(), test.args[0], test.args[1], test.args[2])

			model := NewTestBaseModel()
			model.ID = Incre(model)
			model.IntVal = i
			model.FloatVal = float64(i) + 0.5
			model.StringVal = fmt.Sprintf("test_string_%d", i)
			model.BoolVal = true

			defer func() {
				Defer()
				FlushNow()
				// 验证写入结果
				nmodel := NewTestBaseModel()
				res := nmodel.Read(Condition("int_val == {0}", i))
				if res != test.expect {
					t.Errorf("result expect %v", test.expect)
				}
			}()

			Watch()
			// 执行写入
			Write(model)
		})
	}
}

func TestContextSimpleWrite(t *testing.T) {
	t.Run("WriteByPrimaryKey", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)
		PrepareTestData(t, 5)

		model := NewTestBaseModel()
		model.ID = 100
		model.IntVal = 100
		model.FloatVal = float64(100) + 0.5
		model.StringVal = fmt.Sprintf("test_string_%d", 100)
		model.BoolVal = true

		defer func() {
			Defer()
			FlushNow()
			// 验证写入结果
			nmodel := NewTestBaseModel()
			res := nmodel.Read(Condition("int_val == {0}", 100))
			if !res || nmodel.ID != 100 {
				t.Error("result expect ")
			}
		}()

		Watch()
		// 执行写入
		Write(model)
	})
}
