// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"slices"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XObject"
	"github.com/eframework-org/GO.UTIL/XString"
)

// IModel 定义了数据模型的基础接口。
// 实现此接口的类型可以参与数据库的 CRUD 操作和缓存管理。
type IModel interface {
	// Ctor 执行模型的构造初始化。
	// obj 为模型实例，必须是实现了 IModel 接口的结构体指针。
	// 此方法会在模型创建时自动调用，用于初始化模型的基本状态。
	Ctor(obj any)

	// OnEncode 在对象编码前调用。
	// 子类可以重写此方法以实现自定义的编码逻辑。
	// 通常用于在数据持久化前对字段进行预处理。
	OnEncode()

	// OnDecode 在对象解码后调用。
	// 子类可以重写此方法以实现自定义的解码逻辑。
	// 通常用于在数据读取后对字段进行后处理。
	OnDecode()

	// AliasName 返回数据库别名。
	// 返回值用于标识不同的数据库连接。
	// 此方法必须由子类实现。
	AliasName() string

	// TableName 返回数据表名称。
	// 返回值用于标识数据库中的具体表。
	// 此方法必须由子类实现。
	TableName() string

	// ModelUnique 返回模型的唯一标识。
	// 返回值格式为 "数据库别名_表名"。
	// 用于在缓存和其他场景中唯一标识一个模型类型。
	ModelUnique() string

	// DataUnique 返回数据记录的唯一标识。
	// 返回值格式为 "模型标识_主键值"。
	// 用于在缓存和其他场景中唯一标识一条记录。
	DataUnique() string

	// DataValue 获取指定字段的值。
	// field 为字段名称。
	// 返回字段值，若字段不存在则返回 nil。
	DataValue(field string) any

	// Count 统计符合条件的记录数量。
	// cond 为可选的查询条件。
	// 返回记录数量，如果发生错误则返回 -1。
	Count(cond ...*condition) int

	// Max 获取指定列的最大值。
	// column 为可选的列名，若不指定则使用主键列。
	// 返回最大值，如果发生错误则返回 -1。
	Max(column ...string) int

	// Min 获取指定列的最小值。
	// column 为可选的列名，若不指定则使用主键列。
	// 返回最小值，如果发生错误则返回 -1。
	Min(column ...string) int

	// Delete 删除当前记录。
	// 使用主键作为删除条件。
	// 返回受影响的行数，如果发生错误则返回 -1。
	Delete() int

	// Write 写入或更新当前记录。
	// 在写入前会调用 OnEncode 进行编码处理。
	// 返回受影响的行数，如果发生错误则返回 -1。
	Write() int

	// Read 读取符合条件的记录。
	// cond 为可选的查询条件，若不指定则使用主键作为查询条件。
	// 读取成功后会调用 OnDecode 进行解码处理。
	// 返回是否成功读取到记录。
	Read(cond ...*condition) bool

	// List 查询符合条件的记录列表。
	// rets 必须是指向切片的指针，用于存储查询结果。
	// cond 为可选的查询条件，可以指定偏移量和限制数量。
	// 返回查询到的记录数量，如果发生错误则返回 -1。
	List(rets any, cond ...*condition) int

	// Clear 清理符合条件的记录。
	// cond 为可选的查询条件，若不指定则清理所有记录。
	// 返回受影响的行数，如果发生错误则返回 -1。
	Clear(cond ...*condition) int

	// IsValid 检查或设置对象的有效性。
	// value 为可选的设置值，如果提供则设置对象的有效性状态。
	// 返回对象当前的有效性状态。
	IsValid(value ...bool) bool

	// Clone 创建对象的深度拷贝。
	// 拷贝后会调用 OnDecode 进行解码处理。
	// 返回新的对象实例，如果拷贝失败则返回 nil。
	Clone() IModel

	// Json 将对象转换为 JSON 字符串。
	// 返回 JSON 格式的字符串表示。
	Json() string

	// Equals 比较两个对象是否相等。
	// model 为待比较的对象。
	// 返回两个对象的所有数据库字段是否完全相等。
	Equals(model IModel) bool

	// Matchs 检查对象是否匹配指定条件。
	// cond 为可选的匹配条件。
	// 返回对象是否满足所有条件。
	Matchs(cond ...*condition) bool
}

// Model 实现了 IModel 接口的基础模型。
// T 为具体的模型类型，必须是结构体类型。
// 所有的具体模型类型都应该嵌入此类型。
type Model[T any] struct {
	this        IModel `orm:"-" json:"-"` // 模型实例
	modelUnique string `orm:"-" json:"-"` // 模型标识
	dataUnique  string `orm:"-" json:"-"` // 数据标识
	isValid     bool   `orm:"-" json:"-"` // 有效标志
}

// Ctor 初始化模型实例。
// obj 必须实现 IModel 接口。
// 此方法会在模型创建时自动调用。
func (md *Model[T]) Ctor(obj any) {
	md.this = obj.(IModel)
	md.modelUnique = ""
	md.dataUnique = ""
	md.isValid = false
}

// OnEncode 在对象编码前调用。
// 子类可以重写此方法以实现自定义的编码逻辑。
func (md *Model[T]) OnEncode() {}

// OnDecode 在对象解码后调用。
// 子类可以重写此方法以实现自定义的解码逻辑。
func (md *Model[T]) OnDecode() {}

// AliasName 返回数据库别名。
// 此方法需要被子类重写，默认会触发 panic。
func (md *Model[T]) AliasName() string { XLog.Panic("Alias name is nil."); return "" }

// TableName 返回数据表名称。
// 此方法需要被子类重写，默认会触发 panic。
func (md *Model[T]) TableName() string { XLog.Panic("Table name is nil."); return "" }

// ModelUnique 返回模型的唯一标识。
// 返回值格式为 "数据库别名_表名"。
func (md *Model[T]) ModelUnique() string {
	if XString.IsEmpty(md.modelUnique) {
		md.modelUnique = fmt.Sprintf("%v_%v", md.this.AliasName(), md.this.TableName())
	}
	return md.modelUnique
}

// DataUnique 返回数据记录的唯一标识。
// 返回值格式为 "模型标识_主键值"。
// 如果模型信息或主键未找到，将返回空字符串。
func (md *Model[T]) DataUnique() string {
	if XString.IsEmpty(md.dataUnique) {
		meta := getModelInfo(md.this)
		if meta == nil {
			XLog.Warn("XOrm.Model.DataUnique(%v): model info is nil.", md.this.ModelUnique())
			return ""
		}
		if meta.Fields.Pk == nil {
			XLog.Warn("XOrm.Model.DataUnique(%v): primary key was not found.", md.this.ModelUnique())
			return ""
		}
		fvalue := md.this.DataValue(meta.Fields.Pk.Name)
		md.dataUnique = fmt.Sprintf("%v_%v", md.this.ModelUnique(), fvalue)
	}
	return md.dataUnique
}

// DataValue 获取指定字段的值。
// field 为字段名称。
// 返回字段值，若字段不存在则返回 nil。
func (md *Model[T]) DataValue(field string) any {
	vtp := reflect.ValueOf(md.this).Elem()
	fld := vtp.FieldByName(field)
	if fld.IsValid() {
		return fld.Interface()
	}
	return nil
}

// Count 统计符合条件的记录数量。
// cond 为可选的查询条件。
// 返回记录数量，如果发生错误则返回 -1。
func (md *Model[T]) Count(cond ...*condition) int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Count(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		qsetter := ormer.QueryTable(md.this)
		if len(cond) > 0 && cond[0] != nil {
			qsetter = qsetter.SetCond(cond[0].Base)
		}
		cnt, err := qsetter.Count()
		if err != nil {
			XLog.Error("XOrm.Model.Count(%v): %v", md.this.TableName(), err)
			return -1
		} else {
			return int(cnt)
		}
	}
}

// Max 获取指定列的最大值。
// column 为可选的列名，若不指定则使用主键列。
// 返回最大值，如果发生错误则返回 -1。
func (md *Model[T]) Max(column ...string) int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Max(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		name := ""
		if len(column) > 0 {
			name = column[0]
		}
		if name == "" {
			meta := getModelInfo(md.this)
			if meta != nil && meta.Fields.Pk != nil {
				name = meta.Fields.Pk.Column
			}
		}
		if name == "" {
			XLog.Error("XOrm.Model.Max(%v): column was empty.", md.this.ModelUnique())
			return -1
		}

		name = fmt.Sprintf("MAX(`%v`)", name)
		sql := fmt.Sprintf("SELECT %v FROM `%v`", name, md.this.TableName())
		res := ormer.Raw(sql)
		if _, err := res.Exec(); err != nil {
			XLog.Error("XOrm.Model.Max(%v): %v", md.this.TableName(), err)
			return -1
		}

		var rows []orm.Params
		res.Values(&rows)
		if len(rows) > 0 && rows[0] != nil && rows[0][name] != nil {
			return XString.ToInt(rows[0][name].(string))
		}

		return 0
	}
}

// Min 获取指定列的最小值。
// column 为可选的列名，若不指定则使用主键列。
// 返回最小值，如果发生错误则返回 -1。
func (md *Model[T]) Min(column ...string) int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Min(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		name := ""
		if len(column) > 0 {
			name = column[0]
		}
		if name == "" {
			meta := getModelInfo(md.this)
			if meta != nil && meta.Fields.Pk != nil {
				name = meta.Fields.Pk.Column
			}
		}
		if name == "" {
			XLog.Error("XOrm.Model.Min(%v): column was empty.", md.this.ModelUnique())
			return -1
		}

		name = fmt.Sprintf("MIN(`%v`)", name)
		sql := fmt.Sprintf("SELECT %v FROM `%v`", name, md.this.TableName())
		res := ormer.Raw(sql)
		if _, err := res.Exec(); err != nil {
			XLog.Error("XOrm.Model.Min(%v): %v", md.this.TableName(), err)
			return -1
		}

		var rows []orm.Params
		res.Values(&rows)
		if len(rows) > 0 && rows[0] != nil && rows[0][name] != nil {
			return XString.ToInt(rows[0][name].(string))
		}
		return 0
	}
}

// Delete 删除当前记录。
// 使用主键作为删除条件。
// 返回受影响的行数，如果发生错误则返回 -1。
func (md *Model[T]) Delete() int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Delete(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		meta := getModelInfo(md.this)
		if meta == nil {
			XLog.Error("XOrm.Model.Delete(%v): model info is nil", md.this.TableName())
			return -1
		}
		if meta.Fields.Pk == nil {
			XLog.Error("XOrm.Model.Delete(%v): primary key was not found", md.this.TableName())
			return -1
		}
		cond := orm.NewCondition().And(meta.Fields.Pk.Column, md.this.DataValue(meta.Fields.Pk.Name)) // 附加主键值
		qsetter := ormer.QueryTable(md.this).SetCond(cond)
		count, err := qsetter.Delete()
		if err != nil {
			XLog.Error("XOrm.Model.Delete(%v): %v", md.this.TableName(), err)
			return -1
		}
		return int(count)
	}
}

// Write 写入或更新当前记录。
// 在写入前会调用 OnEncode 进行编码处理。
// 返回受影响的行数，如果发生错误则返回 -1。
func (md *Model[T]) Write() int {
	md.this.IsValid(true)
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Write(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		md.this.OnEncode()
		count, err := ormer.InsertOrUpdate(md.this)
		if err != nil {
			XLog.Error("XOrm.Model.Write(%v): %v", md.this.TableName(), err)
			return -1
		}
		return int(count)
	}
}

// Read 读取符合条件的记录。
// cond 为可选的查询条件，若不指定则使用主键作为查询条件。
// 读取成功后会调用 OnDecode 进行解码处理。
// 返回是否成功读取到记录。
func (md *Model[T]) Read(cond ...*condition) bool {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Read(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return false
	} else {
		meta := getModelInfo(md.this)
		if meta == nil {
			XLog.Error("XOrm.Model.Read(%v): model info is nil", md.this.TableName())
			return false
		}
		if meta.Fields.Pk == nil {
			XLog.Error("XOrm.Model.Read(%v): primary key was not found", md.this.TableName())
			return false
		}
		qsetter := ormer.QueryTable(md.this)
		if len(cond) > 0 && cond[0] != nil {
			ncond := cond[0]
			qsetter = qsetter.SetCond(ncond.Base)
		} else {
			qsetter = qsetter.SetCond(orm.NewCondition().And(meta.Fields.Pk.Column, md.this.DataValue(meta.Fields.Pk.Name))) // 附加主键值
		}
		that := md.this // qsetter.One() 会修改对象，所以需要暂存指针
		e := qsetter.One(that)
		md.this = that // 恢复指针
		if e != nil {
			XLog.Error("XOrm.Model.Read(%v): %v", md.this.TableName(), e)
			return false
		} else {
			md.this.IsValid(true)
			md.this.OnDecode()
			return true
		}
	}
}

// List 查询符合条件的记录列表。
// rets 必须是指向切片的指针，用于存储查询结果。
// cond 为可选的查询条件，可以指定偏移量和限制数量。
// 返回查询到的记录数量，如果发生错误则返回 -1。
func (md *Model[T]) List(rets any, cond ...*condition) int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.List(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		val := reflect.ValueOf(rets)
		if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Slice {
			XLog.Error("XOrm.Model.List(%v): rets must be a pointer to a slice", md.this.TableName())
			return -1
		}

		qsetter := ormer.QueryTable(md.this)
		if len(cond) > 0 && cond[0] != nil {
			ncond := cond[0]
			qsetter = qsetter.SetCond(ncond.Base)
			if ncond.Offset > 0 {
				qsetter = qsetter.Offset(ncond.Offset)
			}
			if ncond.Limit > 0 {
				qsetter = qsetter.Limit(ncond.Limit)
			}
		}

		tcount, terr := qsetter.All(val.Elem().Addr().Interface())
		if terr != nil {
			XLog.Error("XOrm.Model.List(%v): %v", md.this.TableName(), terr)
			return -1
		}

		if tcount > 0 {
			for i := 0; i < val.Elem().Len(); i++ {
				elem := val.Elem().Index(i)
				ev := elem.Interface()
				if model, ok := ev.(IModel); ok {
					model.Ctor(ev)
					model.OnDecode()
					model.IsValid(true)
				}
			}
		}

		return int(tcount)
	}
}

// Clear 清理符合条件的记录。
// cond 为可选的查询条件，若不指定则清理所有记录。
// 返回受影响的行数，如果发生错误则返回 -1。
func (md *Model[T]) Clear(cond ...*condition) int {
	if ormer := orm.NewOrmUsingDB(md.this.AliasName()); ormer == nil {
		XLog.Error("XOrm.Model.Clear(%v): failed to create orm instance of %v.", md.this.TableName(), md.this.AliasName())
		return -1
	} else {
		qsetter := ormer.QueryTable(md.this.TableName())
		var ncond *condition
		if len(cond) > 0 && cond[0] != nil {
			ncond = cond[0]
		} else {
			// beego orm 的 Delete 方法不支持条件，所以需要使用主键字段 >= 0 作为条件，这样可以匹配所有记录
			meta := getModelInfo(md.this)
			if meta == nil {
				XLog.Error("XOrm.Model.Clear(%v): model info is nil", md.this.TableName())
				return -1
			}
			if meta.Fields.Pk == nil {
				XLog.Error("XOrm.Model.Clear(%v): primary key was not found", md.this.TableName())
				return -1
			}
			ncond = Condition(fmt.Sprintf("%v >= {0}", meta.Fields.Pk.Column), 0)
		}

		qsetter = qsetter.SetCond(ncond.Base)
		if ncond.Offset > 0 {
			qsetter = qsetter.Offset(ncond.Offset)
		}
		if ncond.Limit > 0 {
			qsetter = qsetter.Limit(ncond.Limit)
		}

		count, err := qsetter.Delete()
		if err != nil {
			XLog.Error("XOrm.Model.Clear(%v): %v", md.this.TableName(), err)
			return -1
		}
		return int(count)
	}
}

// IsValid 检查或设置对象的有效性。
// value 为可选的设置值。
// 返回对象是否有效。
func (md *Model[T]) IsValid(value ...bool) bool {
	if len(value) > 0 {
		md.isValid = value[0]
	}
	return md.isValid
}

// Clone 创建对象的深度拷贝。
// 拷贝后会调用 OnDecode 进行解码处理。
// 返回新的对象实例，如果拷贝失败则返回 nil。
func (md *Model[T]) Clone() IModel {
	dst := new(T)
	psrc := (*T)(unsafe.Pointer(reflect.ValueOf(md.this).Pointer()))
	pdst := (*T)(unsafe.Pointer(reflect.ValueOf(dst).Pointer()))
	if psrc == nil || pdst == nil {
		XLog.Error("XOrm.Model.Clone(%v): invalid pointer", md.this.TableName())
		return md.this
	}
	*pdst = *psrc

	if model, ok := any(dst).(IModel); ok {
		model.Ctor(dst)
		model.OnDecode()
		model.IsValid(true)
		return model
	} else {
		return nil
	}
}

// Json 将对象转换为 JSON 字符串。
// 返回 JSON 格式的字符串表示。
func (md *Model[T]) Json() string {
	ret, _ := XObject.ToJson(md.this)
	return ret
}

// Equals 比较两个对象是否相等。
// model 为待比较的对象。
// 返回两个对象的所有数据库字段是否完全相等。
func (md *Model[T]) Equals(model IModel) bool {
	if md.this == model {
		return true
	}
	if md.this == nil || model == nil {
		return md.this == model
	}

	meta := getModelInfo(md.this)
	if meta == nil {
		return false
	}

	thisAddr := reflect.ValueOf(md.this).Elem()
	compAddr := reflect.ValueOf(model).Elem()

	for _, field := range meta.Fields.FieldsDB {
		thisVal := thisAddr.FieldByName(field.Name).Interface()
		compFld := compAddr.FieldByName(field.Name)
		if !compFld.IsValid() {
			return false
		}
		compVal := compFld.Interface()
		if thisVal != compVal {
			return false
		}
	}

	return true
}

// Matchs 检查对象是否匹配指定条件。
// cond 为可选的匹配条件。
// 返回对象是否满足所有条件。
func (md *Model[T]) Matchs(cond ...*condition) bool {
	if len(cond) == 0 || cond[0] == nil {
		return true
	}

	meta := getModelInfo(md.this)
	if meta == nil {
		return false
	}

	return doMatch(md.this, meta, getCondParams(cond[0].Base))
}

// doMatch 内部匹配方法
func doMatch(model IModel, meta *modelInfo, conds []beegoCondValue) bool {
	if conds == nil {
		return false
	}

	for i := range conds {
		cond := conds[i]
		hasNext := false
		var nextCond beegoCondValue
		if i < len(conds)-1 {
			nextCond = conds[i+1]
			hasNext = true
		}

		if cond.IsCond {
			if doMatch(model, meta, getCondParams(cond.Cond)) == !cond.IsNot {
				if !hasNext || nextCond.IsOr {
					return true
				}
			} else if !hasNext || !nextCond.IsOr {
				return false
			}
		} else {
			if doComp(model, meta, cond) == !cond.IsNot {
				if !hasNext || nextCond.IsOr {
					return true
				}
			} else if !hasNext || !nextCond.IsOr {
				return false
			}
		}
	}
	return true
}

// 根据指定操作符对字段进行比较
//
//	整型支持: Int,Int32,Int64
//	浮点支持: Float32,Float64
func doComp(model IModel, meta *modelInfo, cond beegoCondValue) bool {
	if !isValidCondition(cond) {
		return false
	}

	field, operator := parseCondition(cond)
	cvalue := getFieldValue(model, meta, field)
	if cvalue == nil {
		return false
	}

	ctype := reflect.TypeOf(cvalue)
	switch operator {
	case "isnull":
		return isNullValue(cvalue, ctype)
	case "in":
		return handleInOperator(cvalue, ctype, cond.Args)
	case "exact", "ne":
		return handleExactOperator(cvalue, ctype, operator, cond.Args[0])
	case "gt", "gte", "lt", "lte":
		return handleComparisonOperator(cvalue, ctype, operator, cond.Args[0])
	case "contains", "startswith", "endswith":
		return handleStringOperator(cvalue, ctype, operator, cond.Args[0])
	default:
		XLog.Error("XOrm.Model.doComp: operator: %v wasn't supported for table: %v", operator, model.TableName())
		return false
	}
}

// 检查条件是否有效
func isValidCondition(cond beegoCondValue) bool {
	return len(cond.Exprs) > 0 && len(cond.Args) > 0
}

// 解析条件表达式
func parseCondition(cond beegoCondValue) (field, operator string) {
	field = cond.Exprs[0]
	operator = "exact"
	if len(cond.Exprs) == 2 {
		operator = cond.Exprs[1]
	}
	return
}

// 获取字段值
func getFieldValue(model IModel, meta *modelInfo, field string) any {
	if fmeta := meta.Fields.Columns[field]; fmeta != nil {
		return model.DataValue(fmeta.Name)
	}
	return nil
}

// 判断是否为空值
func isNullValue(value any, typ reflect.Type) bool {
	if typ.Kind() == reflect.String {
		return value == ""
	}
	return value == nil
}

// 处理 IN 操作符
func handleInOperator(cvalue any, ctype reflect.Type, args []any) bool {
	if len(args) == 0 {
		return false
	}

	// 处理切片参数
	inArgs := normalizeInArgs(args)

	switch {
	case isNumericType(ctype):
		return handleNumericInOperator(cvalue, ctype, inArgs)
	case ctype.Kind() == reflect.String:
		return slices.Contains(inArgs, cvalue)
	default:
		return false
	}
}

// 标准化 IN 操作符的参数
func normalizeInArgs(args []any) []any {
	if len(args) == 0 {
		return args
	}

	firstArg := args[0]
	argType := reflect.TypeOf(firstArg)
	if argType.Kind() != reflect.Slice && argType.Kind() != reflect.Array {
		return args
	}

	val := reflect.ValueOf(firstArg)
	result := make([]any, 0, val.Len())
	for i := range val.Len() {
		v := val.Index(i)
		if v.CanInterface() {
			result = append(result, v.Interface())
		}
	}
	return result
}

// 处理数值类型的 IN 操作
func handleNumericInOperator(cvalue any, ctype reflect.Type, args []any) bool {
	if isIntegerType(ctype) {
		return handleIntegerInOperator(cvalue, args)
	}
	return handleFloatInOperator(cvalue, args)
}

// 处理整数类型的 IN 操作
func handleIntegerInOperator(cvalue any, args []any) bool {
	cval := toInt64(cvalue)
	if cval == nil {
		return false
	}

	for _, arg := range args {
		if val := toInt64(arg); val != nil && *val == *cval {
			return true
		}
	}
	return false
}

// 处理浮点类型的 IN 操作
func handleFloatInOperator(cvalue any, args []any) bool {
	cval := toFloat64(cvalue)
	if cval == nil {
		return false
	}

	for _, arg := range args {
		if val := toFloat64(arg); val != nil && *val == *cval {
			return true
		}
	}
	return false
}

// 处理精确匹配操作符
func handleExactOperator(cvalue any, ctype reflect.Type, operator string, arg any) bool {
	switch {
	case isNumericType(ctype):
		return handleNumericExactOperator(cvalue, ctype, operator, arg)
	case ctype.Kind() == reflect.String || ctype.Kind() == reflect.Bool:
		return operator == "exact" && arg == cvalue || operator == "ne" && arg != cvalue
	default:
		return false
	}
}

// 处理数值类型的精确匹配
func handleNumericExactOperator(cvalue any, ctype reflect.Type, operator string, arg any) bool {
	if isIntegerType(ctype) {
		return handleIntegerExactOperator(cvalue, operator, arg)
	}
	return handleFloatExactOperator(cvalue, operator, arg)
}

// 处理整数类型的精确匹配
func handleIntegerExactOperator(cvalue any, operator string, arg any) bool {
	cval := toInt64(cvalue)
	val := toInt64(arg)
	if cval == nil || val == nil {
		return false
	}

	if operator == "exact" {
		return *cval == *val
	}
	return *cval != *val
}

// 处理浮点类型的精确匹配
func handleFloatExactOperator(cvalue any, operator string, arg any) bool {
	cval := toFloat64(cvalue)
	val := toFloat64(arg)
	if cval == nil || val == nil {
		return false
	}

	if operator == "exact" {
		return *cval == *val
	}
	return *cval != *val
}

// 处理比较操作符
func handleComparisonOperator(cvalue any, ctype reflect.Type, operator string, arg any) bool {
	if !isNumericType(ctype) {
		return false
	}

	if isIntegerType(ctype) {
		return handleIntegerComparisonOperator(cvalue, operator, arg)
	}
	return handleFloatComparisonOperator(cvalue, operator, arg)
}

// 处理整数类型的比较操作
func handleIntegerComparisonOperator(cvalue any, operator string, arg any) bool {
	cval := toInt64(cvalue)
	val := toInt64(arg)
	if cval == nil || val == nil {
		return false
	}

	switch operator {
	case "gt":
		return *cval > *val
	case "gte":
		return *cval >= *val
	case "lt":
		return *cval < *val
	case "lte":
		return *cval <= *val
	default:
		return false
	}
}

// 处理浮点类型的比较操作
func handleFloatComparisonOperator(cvalue any, operator string, arg any) bool {
	cval := toFloat64(cvalue)
	val := toFloat64(arg)
	if cval == nil || val == nil {
		return false
	}

	switch operator {
	case "gt":
		return *cval > *val
	case "gte":
		return *cval >= *val
	case "lt":
		return *cval < *val
	case "lte":
		return *cval <= *val
	default:
		return false
	}
}

// 处理字符串操作符
func handleStringOperator(cvalue any, ctype reflect.Type, operator string, arg any) bool {
	if ctype.Kind() != reflect.String {
		return false
	}

	str := cvalue.(string)
	pattern := arg.(string)

	switch operator {
	case "contains":
		return strings.Contains(str, pattern)
	case "startswith":
		return strings.HasPrefix(str, pattern)
	case "endswith":
		return strings.HasSuffix(str, pattern)
	default:
		return false
	}
}

// 类型判断辅助函数
func isNumericType(t reflect.Type) bool {
	return isIntegerType(t) || isFloatType(t)
}

func isIntegerType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func isFloatType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// 类型转换辅助函数
func toInt64(v any) *int64 {
	if v == nil {
		return nil
	}

	var result int64
	switch val := v.(type) {
	case int:
		result = int64(val)
	case int32:
		result = int64(val)
	case int64:
		result = val
	default:
		rv := reflect.ValueOf(v)
		if isIntegerType(rv.Type()) {
			result = rv.Int()
		} else {
			return nil
		}
	}
	return &result
}

func toFloat64(v any) *float64 {
	if v == nil {
		return nil
	}

	var result float64
	switch val := v.(type) {
	case float32:
		result = float64(val)
	case float64:
		result = val
	default:
		rv := reflect.ValueOf(v)
		if isFloatType(rv.Type()) {
			result = rv.Float()
		} else {
			return nil
		}
	}
	return &result
}
