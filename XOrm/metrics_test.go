// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import "testing"

// TestSharedMetrics 测试统计信息的全局访问点。
func TestSharedMetrics(t *testing.T) {
	if Metrics() == nil {
		t.Fatalf("Shared metrics instance should not be nil.")
	}
}
