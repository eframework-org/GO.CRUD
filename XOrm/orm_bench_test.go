// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"testing"
)

// BenchmarkOrm 比较 XOrm 缓存查询和回源查询的性能。
// 多核测试可以使用命令 go test -run=^$ -bench=^BenchmarkOrm$ -benchmem -cpu=1,2,4,8,16,32
func BenchmarkOrm(b *testing.B) {
	defer ResetContext()
	defer ResetBaseTest()

	for _, count := range []int{10000, 100000} {
		ResetContext()
		ResetBaseTest()
		SetupBaseTest()
		WriteBaseTest(count)

		model := NewTestBaseModel()
		cond := Cond("int_val > {0} && string_val == {1}", 0, "hello world")
		Watch()
		List(model) // 读取至缓存
		Defer()

		b.Run(fmt.Sprintf("XOrm.List/%d", count), func(b *testing.B) {
			Watch()
			List(model, cond)
			Defer()
		})

		b.Run(fmt.Sprintf("XOrm.Model.List/%d", count), func(b *testing.B) {
			var rets []*TestBaseModel
			model.List(&rets, cond)
		})
	}
}
