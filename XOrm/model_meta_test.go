// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XObject"
	"github.com/stretchr/testify/assert"
)

type TestModelMetaInfo struct {
	Model[TestModelMetaInfo]
	Id   int    `orm:"column(id);pk"`
	Name string `orm:"column(name);size(100)"`
}

func (m *TestModelMetaInfo) AliasName() string {
	return "myalias"
}

func (m *TestModelMetaInfo) TableName() string {
	return "mytable"
}

func TestModelMeta(t *testing.T) {
	defer orm.ResetModelCache()
	orm.ResetModelCache()

	model := XObject.New[TestModelMetaInfo]()

	// 检查模型是否已注册
	Meta(model, true, true)
	meta := getModelMeta(model)
	assert.NotNil(t, meta, "已注册的模型描述信息不应为空。")
	assert.Equal(t, "mytable", meta.table, "注册模型的名称应当和输入的一致。")
	assert.Equal(t, true, meta.cache, "注册模型的缓存标识应当和输入的一致。")
	assert.Equal(t, true, meta.writable, "注册模型的可写标识应当和输入的一致。")

	// 测试重复注册同一个模型
	defer func() {
		if r := recover(); r == nil {
			t.Error("重复注册相同模型时应当 panic。")
		}
	}()
	Meta(model, true, true)

	// 测试空模型
	defer func() {
		if r := recover(); r == nil {
			t.Error("注册空模型时应当 panic。")
		}
	}()
	Meta(nil, true, true)
}
