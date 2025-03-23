// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"testing"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XObject"
)

type TestModelInfo struct {
	Model[TestModelInfo]
	Id   int    `orm:"pk;auto"`
	Name string `orm:"size(100)"`
}

func (m *TestModelInfo) AliasName() string {
	return "myalias"
}

func (m *TestModelInfo) TableName() string {
	return "mytable"
}

func NewTestModelInfo() *TestModelInfo {
	return XObject.New[TestModelInfo]()
}

func SetupModelInfoTest() {
	Register(NewTestModelInfo(), true, true, true)
}

func ResetModelInfoTest() {
	orm.ResetModelCache()
	modelCacheMu.Lock()
	modelCache = make(map[string]*modelInfo)
	modelCacheMu.Unlock()
}

func TestModelRegister(t *testing.T) {
	SetupModelInfoTest()
	defer ResetModelInfoTest()

	// 测试重复注册
	model := NewTestModelInfo()

	// 检查模型是否已注册
	meta := getModelInfo(NewTestModelInfo())
	if meta == nil {
		t.Error("Model should be registered")
	}

	// 验证模型属性
	if !meta.Cache || !meta.Persist || !meta.Writable {
		t.Error("Model info properties not set correctly")
	}

	if meta.Table != "mytable" {
		t.Errorf("Expected table name 'mytable', got '%s'", meta.Table)
	}

	// 测试重复注册同一个模型
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for duplicate registration")
		}
	}()
	Register(model, true, true, true)

	// 测试空模型
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil model")
		}
	}()
	Register(nil, true, true, true)
}

func TestGetModelInfo(t *testing.T) {
	SetupModelInfoTest()
	defer ResetModelInfoTest()

	// 测试获取已注册模型
	meta := getModelInfo(NewTestModelInfo())
	if meta == nil {
		t.Error("Should get model info for registered model")
	}

	// 测试获取空模型
	meta = getModelInfo(nil)
	if meta != nil {
		t.Error("Should return nil for nil model")
	}
}

func TestModelFieldInfo(t *testing.T) {
	SetupModelInfoTest()
	defer ResetModelInfoTest()

	meta := getModelInfo(NewTestModelInfo())
	if meta == nil {
		t.Fatal("Model info should not be nil")
	}

	// 检查主键字段
	if meta.Fields.Pk == nil {
		t.Error("Primary key field should be defined")
	}

	if meta.Fields.Pk.Name != "Id" {
		t.Errorf("Expected primary key name 'Id', got '%s'", meta.Fields.Pk.Name)
	}

	// 检查字段数量
	if len(meta.Fields.Fields) != 2 { // Id and Name
		t.Errorf("Expected 2 fields, got %d", len(meta.Fields.Fields))
	}

	// 检查具体字段属性
	nameField := meta.Fields.Fields["Name"]
	if nameField == nil {
		t.Error("Name field should exist")
	}

	if nameField.Size != 100 {
		t.Errorf("Expected Name field size 100, got %d", nameField.Size)
	}
}
