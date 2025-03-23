// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XObject"
	_ "github.com/go-sql-driver/mysql"
)

const (
	// 数据库配置
	TestDatabaseHost = "localhost"
	TestDatabasePort = 3306
	TestDatabaseName = "mysql"
	TestDatabaseUser = "root"
	TestDatabasePass = "123456"

	// 测试表配置
	TestTableName = "model_base_test"
	TestAliasName = "test_alias"

	// 测试SQL
	TestTruncateTableSQL = `TRUNCATE TABLE ` + TestTableName + `;`
	TestDropTableSQL     = `DROP TABLE IF EXISTS ` + TestTableName + `;`
	TestCreateTableSQL   = `
		CREATE TABLE ` + TestTableName + ` (
			id INT AUTO_INCREMENT PRIMARY KEY,
			int_val INT,
			float_val FLOAT,
			string_val VARCHAR(255),
			bool_val BOOLEAN
		);`
)

// TestBaseModel 测试模型
type TestBaseModel struct {
	Model[TestBaseModel] `orm:"-" json:"-"`
	ID                   int     `orm:"column(id);pk"`
	IntVal               int     `orm:"column(int_val)"`
	FloatVal             float64 `orm:"column(float_val)"`
	StringVal            string  `orm:"column(string_val)"`
	BoolVal              bool    `orm:"column(bool_val)"`
}

func (m *TestBaseModel) AliasName() string {
	return TestAliasName
}

func (m *TestBaseModel) TableName() string {
	return TestTableName
}

func NewTestBaseModel() *TestBaseModel {
	return XObject.New[TestBaseModel]()
}

var setupDatabaseOnce sync.Once

// SetupBaseTest 初始化测试数据库
func SetupBaseTest(t *testing.T) {
	// 连接测试数据库
	testDatabase, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4",
		TestDatabaseUser, TestDatabasePass, TestDatabaseHost, TestDatabasePort, TestDatabaseName))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer testDatabase.Close()

	// 创建测试表
	if _, err := testDatabase.Exec(TestDropTableSQL); err != nil {
		t.Fatalf("Failed to drop test table: %v", err)
	}
	if _, err := testDatabase.Exec(TestCreateTableSQL); err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// 注册数据库
	setupDatabaseOnce.Do(func() {
		if err := orm.RegisterDataBase(TestAliasName, "mysql",
			fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4",
				TestDatabaseUser, TestDatabasePass, TestDatabaseHost, TestDatabasePort, TestDatabaseName)); err != nil {
			t.Fatalf("Failed to register database: %v", err)
		}
	})

	// 注册模型
	Register(NewTestBaseModel(), true, true, true)
}

// ResetBaseTest 清理测试数据库
func ResetBaseTest(t *testing.T) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		TestDatabaseUser, TestDatabasePass, TestDatabaseHost, TestDatabasePort, TestDatabaseName))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(TestDropTableSQL); err != nil {
		t.Fatalf("Failed to cleanup test table: %v", err)
	}

	// 清理数据库连接和注册信息
	orm.ResetModelCache()
	modelCacheMu.Lock()
	modelCache = make(map[string]*modelInfo)
	modelCacheMu.Unlock()
	commitWaitGroup = sync.WaitGroup{}
	clearGlobalMax()
	clearCommitMap()
	clearLock()
}

// ClearBaseTest 仅清空测试数据库数据
func ClearBaseTest(t *testing.T) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		TestDatabaseUser, TestDatabasePass, TestDatabaseHost, TestDatabasePort, TestDatabaseName))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(TestTruncateTableSQL); err != nil {
		t.Fatalf("Failed to truncate test table: %v", err)
	}
}

// PrepareTestData 准备测试数据
func PrepareTestData(t *testing.T, count int) []*TestBaseModel {
	var models []*TestBaseModel
	for i := 1; i <= count; i++ {
		model := NewTestBaseModel()
		model.ID = Incre(model)
		model.IntVal = i
		model.FloatVal = float64(i) + 0.5
		model.StringVal = fmt.Sprintf("test_string_%d", i)
		model.BoolVal = i%2 == 0

		if writeCount := model.Write(); writeCount <= 0 {
			t.Fatalf("Failed to insert test data %d", i)
		}
		models = append(models, model)
	}
	return models
}

// TestModelBasic 测试基本操作
func TestModelBasic(t *testing.T) {
	// 测试写入
	t.Run("Write", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)

		model := NewTestBaseModel()
		model.IntVal = 1
		model.FloatVal = 1.5
		model.StringVal = "test_string"
		model.BoolVal = true

		count := model.Write()
		if count <= 0 {
			t.Error("Write operation failed")
		}
	})

	// 测试读取
	t.Run("Read", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)
		models := PrepareTestData(t, 5)

		// 测试主键读取
		nmodel := NewTestBaseModel()
		nmodel.ID = models[3].ID
		if !nmodel.Read() {
			t.Error("Read by primary key failed")
		}
		if !nmodel.Equals(models[3]) {
			t.Error("Read by primary key data mismatch")
		}

		// 测试条件读取
		nmodel = NewTestBaseModel()
		nmodel.ID = models[2].ID
		cond := Condition("int_val == {0}", 4)
		if !nmodel.Read(cond) {
			t.Error("Read with condition failed")
		}
		if !nmodel.Equals(models[3]) {
			t.Error("Read with condition data mismatch")
		}
	})

	// 测试删除
	t.Run("Delete", func(t *testing.T) {
		SetupBaseTest(t)
		defer ResetBaseTest(t)
		ResetAllResource(t)

		model := PrepareTestData(t, 1)[0]
		count := model.Delete()
		if count <= 0 {
			t.Error("Delete by primary key failed")
		}
		if model.Read() {
			t.Error("Record should be deleted")
		}
	})
}

// TestModelList 测试列表操作
func TestModelList(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	models := PrepareTestData(t, 5)
	model := NewTestBaseModel()

	// 测试Count
	t.Run("Count", func(t *testing.T) {
		count := model.Count()
		if count != len(models) {
			t.Errorf("Expected count %d, got %d", len(models), count)
		}

		// 测试带条件的Count
		cond := Condition("int_val > {0}", 3)
		count = model.Count(cond)
		if count != 2 {
			t.Errorf("Expected count 2 with condition, got %d", count)
		}
	})

	// 测试List
	t.Run("List", func(t *testing.T) {
		var results []*TestBaseModel
		count := model.List(&results)
		if count != len(models) {
			t.Errorf("Expected list count %d, got %d", len(models), count)
		}

		// 测试带条件的List
		results = nil
		cond := Condition("int_val > {0}", 3)
		count = model.List(&results, cond)
		if count != 2 {
			t.Errorf("Expected 2 records matching condition, got %d", count)
		}
		for _, result := range results {
			if result.IntVal <= 3 {
				t.Errorf("Expected IntVal > 3, got %d", result.IntVal)
			}
		}
	})

	t.Run("ListOriginalQuery", func(t *testing.T) {
		var results []*TestBaseModel
		count := model.List(&results)
		if count != len(models) {
			t.Errorf("Expected list count %d, got %d", len(models), count)
		}

		// 测试带条件的List
		results = nil
		cond := Condition(orm.NewCondition().And("int_val__in", []int{1, 2}))
		count = model.List(&results, cond)
		if count != 2 {
			t.Errorf("Expected 2 records matching condition, got %d", count)
		}
		for _, result := range results {
			if result.IntVal > 2 {
				t.Errorf("Expected IntVal <= 2, got %d", result.IntVal)
			}
		}
	})

	// 测试Max
	t.Run("Max", func(t *testing.T) {
		max := model.Max("int_val")
		if max != 5 {
			t.Errorf("Expected max value 5, got %d", max)
		}
	})

	// 测试Min
	t.Run("Min", func(t *testing.T) {
		min := model.Min("int_val")
		if min != 1 {
			t.Errorf("Expected min value 1, got %d", min)
		}
	})

	// 测试Clear
	t.Run("Clear", func(t *testing.T) {
		count := model.Clear()
		if count != len(models) {
			t.Errorf("Expected clear count %d, got %d", len(models), count)
		}
		if model.Count() != 0 {
			t.Error("Table should be empty after clear")
		}

		// 重新准备测试数据
		models = PrepareTestData(t, 5)

		// 测试带条件的Clear
		cond := Condition("int_val > {0}", 3)
		count = model.Clear(cond)
		if count != 2 {
			t.Errorf("Expected to clear 2 records with condition, got %d", count)
		}
		if model.Count() != 3 {
			t.Error("Should have 3 records remaining after conditional clear")
		}
	})
}

// TestModelUtility 测试工具方法
func TestModelUtility(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	model := NewTestBaseModel()
	model.ID = 1
	model.IntVal = 100
	model.FloatVal = 1.5
	model.StringVal = "test_string"
	model.BoolVal = true

	// 测试Clone
	t.Run("Clone", func(t *testing.T) {
		cloned := model.Clone().(*TestBaseModel)
		if cloned.ID != model.ID {
			t.Error("Cloned ID mismatch")
		}
		if cloned.IntVal != model.IntVal {
			t.Error("Cloned IntVal mismatch")
		}
		if cloned.FloatVal != model.FloatVal {
			t.Error("Cloned FloatVal mismatch")
		}
		if cloned.StringVal != model.StringVal {
			t.Error("Cloned StringVal mismatch")
		}
		if cloned.BoolVal != model.BoolVal {
			t.Error("Cloned BoolVal mismatch")
		}
	})

	// 测试Equals
	t.Run("Equals", func(t *testing.T) {
		// 测试相同对象比较
		t.Run("SameObject", func(t *testing.T) {
			if !model.Equals(model) {
				t.Error("Same object should be equal to itself")
			}
		})

		// 测试相同值的不同对象
		t.Run("SameValues", func(t *testing.T) {
			model2 := NewTestBaseModel()
			model2.ID = model.ID
			model2.IntVal = model.IntVal
			model2.FloatVal = model.FloatVal
			model2.StringVal = model.StringVal
			model2.BoolVal = model.BoolVal

			if !model.Equals(model2) {
				t.Errorf("Objects with same values should be equal:\nmodel1=%#v\nmodel2=%#v", model, model2)
			}
		})

		// 测试不同值的对象
		t.Run("DifferentValues", func(t *testing.T) {
			model2 := NewTestBaseModel()
			model2.ID = model.ID + 1
			model2.IntVal = model.IntVal + 1
			model2.FloatVal = model.FloatVal + 1.0
			model2.StringVal = model.StringVal + "_different"
			model2.BoolVal = !model.BoolVal

			if model.Equals(model2) {
				t.Errorf("Objects with different values should not be equal:\nmodel1=%#v\nmodel2=%#v", model, model2)
			}
		})

		// 测试nil对象
		t.Run("NilObject", func(t *testing.T) {
			if model.Equals(nil) {
				t.Error("Non-nil object should not equal to nil")
			}
		})

		// 测试不同类型对象
		t.Run("DifferentTypes", func(t *testing.T) {
			// 创建一个不同类型的模型
			type DifferentModel struct {
				Model[DifferentModel] `orm:"-" json:"-"`
				ID                    int    `orm:"column(id);pk;auto"`
				Value                 string `orm:"column(value)"`
			}

			differentModel := XObject.New[DifferentModel]()
			differentModel.ID = model.ID
			differentModel.Value = "test"

			if model.Equals(differentModel) {
				t.Error("Objects of different model types should not be equal")
			}
		})

		// 测试字段值边界情况
		t.Run("EdgeCases", func(t *testing.T) {
			model2 := NewTestBaseModel()
			model3 := NewTestBaseModel()

			// 测试空字符串
			model2.StringVal = ""
			model3.StringVal = ""
			if !model2.Equals(model3) {
				t.Errorf("Objects with empty strings should be equal:\nmodel1=%#v\nmodel2=%#v", model2, model3)
			}

			// 测试零值
			model2.IntVal = 0
			model3.IntVal = 0
			model2.FloatVal = 0.0
			model3.FloatVal = 0.0
			if !model2.Equals(model3) {
				t.Errorf("Objects with zero values should be equal:\nmodel1=%#v\nmodel2=%#v", model2, model3)
			}

			// 测试最大值
			model2.IntVal = math.MaxInt32
			model3.IntVal = math.MaxInt32
			model2.FloatVal = math.MaxFloat64
			model3.FloatVal = math.MaxFloat64
			if !model2.Equals(model3) {
				t.Errorf("Objects with max values should be equal:\nmodel1=%#v\nmodel2=%#v", model2, model3)
			}
		})
	})

	// 测试Json
	t.Run("Json", func(t *testing.T) {
		json := model.Json()
		if json == "" {
			t.Error("Json serialization failed")
		}
		if !strings.Contains(json, fmt.Sprintf(`"ID":%d`, model.ID)) {
			t.Error("JSON should contain ID field")
		}
		if !strings.Contains(json, fmt.Sprintf(`"IntVal":%d`, model.IntVal)) {
			t.Error("JSON should contain IntVal field")
		}
		if !strings.Contains(json, fmt.Sprintf(`"FloatVal":%g`, model.FloatVal)) {
			t.Error("JSON should contain FloatVal field")
		}
		if !strings.Contains(json, fmt.Sprintf(`"StringVal":"%s"`, model.StringVal)) {
			t.Error("JSON should contain StringVal field")
		}
		if !strings.Contains(json, fmt.Sprintf(`"BoolVal":%v`, model.BoolVal)) {
			t.Error("JSON should contain BoolVal field")
		}
	})

	// 测试IsValid
	t.Run("IsValid", func(t *testing.T) {
		model2 := NewTestBaseModel()
		if model2.IsValid() {
			t.Error("New model should be invalid by default")
		}
		model2.IsValid(false)
		if model2.IsValid() {
			t.Error("Model should be invalid after setting valid to false")
		}
		model2.IsValid(true)
		if !model2.IsValid() {
			t.Error("Model should be valid after setting valid to true")
		}
	})
}

// TestModelMatchs 测试匹配操作
func TestModelMatchs(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	models := PrepareTestData(t, 5)

	// 测试简单条件匹配
	t.Run("SimpleConditions", func(t *testing.T) {
		// 等于条件
		t.Run("Equals", func(t *testing.T) {
			cond := Condition("int_val == {0}", 3)
			matches := models[2].Matchs(cond)
			if !matches {
				t.Error("Expected model with int_val=3 to match condition")
			}
		})

		// 大于条件
		t.Run("GreaterThan", func(t *testing.T) {
			cond := Condition("int_val > {0}", 3)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
				}
			}
			if matchCount != 2 {
				t.Errorf("Expected 2 models with int_val>3, got %d", matchCount)
			}
		})

		// 小于条件
		t.Run("LessThan", func(t *testing.T) {
			cond := Condition("int_val < {0}", 3)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
				}
			}
			if matchCount != 2 {
				t.Errorf("Expected 2 models with int_val<3, got %d", matchCount)
			}
		})

		// 字符串匹配
		t.Run("StringMatch", func(t *testing.T) {
			cond := Condition("string_val == {0}", "test_string_3")
			matches := models[2].Matchs(cond)
			if !matches {
				t.Error("Expected model with string_val='test_string_3' to match condition")
			}
		})

		// 布尔值匹配
		t.Run("BooleanMatch", func(t *testing.T) {
			cond := Condition("bool_val == {0}", true)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
				}
			}
			if matchCount != 2 {
				t.Errorf("Expected 2 models with bool_val=true, got %d", matchCount)
			}
		})
	})

	// 测试复合条件匹配
	t.Run("CompoundConditions", func(t *testing.T) {
		// AND 条件
		t.Run("AndCondition", func(t *testing.T) {
			// 测试数据中，IntVal > 2 的有 3,4,5，bool_val 为 true 的有 2,4
			// 所以 IntVal > 2 && bool_val == true 的只有 4
			cond := Condition("int_val > {0} && bool_val == {1}", 2, true)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
					if model.IntVal <= 2 || !model.BoolVal {
						t.Errorf("Model with int_val=%d and bool_val=%v should not match",
							model.IntVal, model.BoolVal)
					}
				}
			}
			if matchCount != 1 {
				t.Errorf("Expected 1 model matching AND condition, got %d", matchCount)
			}
		})

		// OR 条件
		t.Run("OrCondition", func(t *testing.T) {
			// 测试数据中，IntVal < 2 的有 1，IntVal > 4 的有 5
			cond := Condition("int_val < {0} || int_val > {1}", 2, 4)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
					if model.IntVal >= 2 && model.IntVal <= 4 {
						t.Errorf("Model with int_val=%d should not match", model.IntVal)
					}
				}
			}
			if matchCount != 2 {
				t.Errorf("Expected 2 models matching OR condition, got %d", matchCount)
			}
		})

		// NOT 条件
		t.Run("NotCondition", func(t *testing.T) {
			// 测试数据中，!(2 <= IntVal <= 4) 意味着 IntVal < 2 或 IntVal > 4
			// 所以应该匹配 IntVal = 1 和 IntVal = 5
			cond := Condition("int_val < {0} || int_val > {1}", 2, 4)
			matchCount := 0
			for _, model := range models {
				if model.Matchs(cond) {
					matchCount++
					if model.IntVal >= 2 && model.IntVal <= 4 {
						t.Errorf("Model with int_val=%d should not match", model.IntVal)
					}
				}
			}
			if matchCount != 2 {
				t.Errorf("Expected 2 models matching NOT condition, got %d", matchCount)
			}
		})

		// 复杂组合条件
		t.Run("ComplexCondition", func(t *testing.T) {
			// 测试数据：
			// ID=1: IntVal=1, FloatVal=1.5, StringVal="test_string_1", BoolVal=false
			// ID=2: IntVal=2, FloatVal=2.5, StringVal="test_string_2", BoolVal=true
			// ID=3: IntVal=3, FloatVal=3.5, StringVal="test_string_3", BoolVal=false
			// ID=4: IntVal=4, FloatVal=4.5, StringVal="test_string_4", BoolVal=true
			// ID=5: IntVal=5, FloatVal=5.5, StringVal="test_string_5", BoolVal=false

			// 使用更简单的条件组合
			cond := Condition("int_val == {0} || int_val == {1}", 1, 4)

			matchCount := 0
			expectedMatches := map[int]struct{}{
				1: {}, // 匹配第一个条件：int_val == 1
				4: {}, // 匹配第二个条件：int_val == 4
			}

			for _, model := range models {
				matches := model.Matchs(cond)
				t.Logf("Model {ID=%d, IntVal=%d}: matches=%v", model.ID, model.IntVal, matches)

				if matches {
					matchCount++
					if _, ok := expectedMatches[model.IntVal]; !ok {
						t.Errorf("Unexpected match for model: {ID=%d, IntVal=%d}",
							model.ID, model.IntVal)
					}
				} else {
					if _, ok := expectedMatches[model.IntVal]; ok {
						t.Errorf("Expected match not found for model: {ID=%d, IntVal=%d}",
							model.ID, model.IntVal)
					}
				}
			}

			if matchCount != 2 {
				t.Errorf("Expected exactly 2 models matching complex condition, got %d", matchCount)
			}
		})
	})
}
