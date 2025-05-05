// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"sync"
	"testing"

	"github.com/petermattis/goid"
	"github.com/stretchr/testify/assert"
)

// TestContextCount 测试计数操作。
func TestContextCount(t *testing.T) {
	defer ResetContext(t)
	defer ResetBaseTest(t)

	model := NewTestBaseModel()
	ResetBaseTest(t)
	SetupBaseTest(t)

	tests := []struct {
		name       string
		concurrent int
		arrange    func(chunk int)
	}{
		{
			name:       "Session",
			concurrent: 10,
			arrange: func(chunk int) {
				gid := goid.Get()
				isSessionListed(gid, model, true)
				for i := range 1000 {
					data := NewTestBaseModel()
					data.ID = chunk*1000 + i + 1
					data.IsValid(true)
					setSessionCache(gid, data)
				}
			},
		},
		{
			name:       "Global",
			concurrent: 10,
			arrange: func(chunk int) {
				isGlobalListed(model, true)
				for i := range 1000 {
					data := NewTestBaseModel()
					data.ID = chunk*1000 + i + 1
					data.IsValid(true)
					setGlobalCache(data)
				}
			},
		},
		{
			name:       "Database",
			concurrent: 1,
			arrange: func(chunk int) {
				isSessionListed(goid.Get(), model, false)
				isGlobalListed(model, false)
				for i := range 1000 {
					data := NewTestBaseModel()
					data.ID = chunk*1000 + i + 1
					data.IsValid(true)
					data.Write()
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Dump()
			var wg sync.WaitGroup
			for i := range test.concurrent {
				wg.Add(1)

				go func(chunk int) {
					defer wg.Done()

					gid := goid.Get()
					ctx := &context{}
					contextMap.Store(gid, ctx)
					defer contextMap.Delete(gid)

					test.arrange(chunk)
					assert.Equal(t, 1000, Count(model, Cond("id >= {0} && id <= {1}", chunk*1000+1, chunk*1000+1000)))
				}(i)
			}
			wg.Wait()
		})
	}
}
