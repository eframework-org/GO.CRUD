// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

// metricsInfo 定义了全局的统计信息。
type metricsInfo struct{}

var sharedMetrics = &metricsInfo{}

// 提供了统计信息的全局访问点。
func Metrics() *metricsInfo {
	return sharedMetrics
}
