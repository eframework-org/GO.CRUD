// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/petermattis/goid"
)

// TestContextCommit 测试提交操作
func TestContextCommit(t *testing.T) {
	tests := []struct {
		name      string
		modelArgs []bool // [cache, persist, writable]
		checkFunc func(t *testing.T)
	}{
		{
			name:      "CommitAllocationAndBasicOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				gid := goid.Get()

				// 测试提交队列分配
				queue := getCommitBatch(gid)
				if queue == nil {
					t.Error("Commit queue should not be nil after allocation")
				}

				// 测试重复分配相同gid的提交队列
				queue2 := getCommitBatch(gid)
				if queue2 != queue {
					t.Error("Allocating commit queue for same gid should return same queue")
				}

				// 验证提交队列容量
				if cap(queue) != commitBatchCapacity {
					t.Errorf("Commit queue capacity = %v, want %v", cap(queue), commitBatchCapacity)
				}
			},
		},
		{
			name:      "CommitObjectPoolOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				// 测试对象池获取
				obj1 := commitObjectPool.Get().(*commitObject)
				if obj1 == nil {
					t.Error("Object from pool should not be nil")
				}

				// 验证对象重置状态
				if obj1.raw != nil || obj1.create || obj1.delete || obj1.clear || obj1.modify || obj1.cond != nil || obj1.meta != nil {
					t.Error("New object from pool should be in reset state")
				}

				// 测试对象重用
				obj1.raw = NewTestBaseModel()
				obj1.create = true
				commitObjectPool.Put(obj1)

				obj2 := commitObjectPool.Get().(*commitObject)
				if obj2.raw == nil || !obj2.create {
					t.Error("Reused object should be not in reset state")
				}
				obj1.reset()
				obj3 := commitObjectPool.Get().(*commitObject)
				if obj3.raw != nil || obj3.create {
					t.Error("Reused object should be in reset state")
				}
			},
		},
		{
			name:      "CommitBatchPoolOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				// 测试批次池获取
				batch1 := commitBatchPool.Get().(*commitBatch)
				if batch1 == nil {
					t.Error("Batch from pool should not be nil")
				}

				// 验证批次重置状态
				if batch1.tag != nil || batch1.time != 0 || batch1.objs != nil {
					t.Error("New batch from pool should be in reset state")
				}

				// 测试批次重用
				batch1.time = 123
				batch1.objs = make([]*commitObject, 0)
				commitBatchPool.Put(batch1)

				batch2 := commitBatchPool.Get().(*commitBatch)
				if batch2.time != 123 || batch2.objs == nil {
					t.Error("Reused batch should not be in reset state")
				}
			},
		},
		{
			name:      "WriteOperationsThroughCommit",
			modelArgs: []bool{false, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.StringVal = "test_commit"

				Write(model)

				// 验证写入结果
				result := Read(model)
				if result == nil || !result.IsValid() || result.StringVal != "test_commit" {
					t.Error("Write through commit should persist data")
				}

				// 测试删除操作
				Delete(model)

				// 验证删除结果
				result = Read(model)
				if result.IsValid() {
					t.Error("Delete through commit should remove data")
				}
			},
		},
		{
			name:      "ConcurrentCommitOperations",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				var wg sync.WaitGroup
				concurrent := 1
				wg.Add(concurrent)

				for i := 0; i < concurrent; i++ {
					go func(idx int) {
						defer wg.Done()

						// 创建并发送对象到提交队列
						model := NewTestBaseModel()
						model.ID = 100 + idx
						model.StringVal = "concurrent_test"
						meta := getModelInfo(model)

						cobj := commitObjectPool.Get().(*commitObject)
						cobj.raw = model
						cobj.meta = meta
						cobj.create = true

						batch := commitBatchPool.Get().(*commitBatch)
						batch.time = int(time.Now().UnixNano() / 1000)
						batch.objs = []*commitObject{cobj}

						queue := getCommitBatch(goid.Get())
						queue <- batch
					}(i)
				}

				// 等待所有并发操作完成
				wg.Wait()
				time.Sleep(1000 * time.Millisecond) // 给提交队列处理一些时间

				// 验证并发写入的结果
				for i := 0; i < concurrent; i++ {
					model := NewTestBaseModel()
					model.ID = 100 + i
					result := Read(model)
					if result == nil || result.StringVal != "concurrent_test" {
						t.Errorf("Concurrent write %d through commit failed", i)
					}
				}
			},
		},
		{
			name:      "ClearOperationsThroughCommit",
			modelArgs: []bool{true, true, true},
			checkFunc: func(t *testing.T) {
				model := NewTestBaseModel()
				meta := getModelInfo(model)

				// 创建清除操作的提交对象
				cobj := commitObjectPool.Get().(*commitObject)
				cobj.raw = model
				cobj.meta = meta
				cobj.clear = true
				cobj.cond = Condition("id > {0}", 2)

				// 执行清除操作
				handleCommitObject(cobj)

				// 验证清除结果
				results := List(model)
				if len(results) != 2 {
					t.Errorf("Clear through commit should remove correct number of records, got %d, want 2", len(results))
				}
				for _, r := range results {
					if r.ID > 2 {
						t.Errorf("Record with ID %d should have been cleared", r.ID)
					}
				}

				// 清理
				cobj.reset()
				commitObjectPool.Put(cobj)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)

			PrepareTestData(t, 5)
			Cleanup()
			Register(NewTestBaseModel(), tt.modelArgs[0], tt.modelArgs[1], tt.modelArgs[2])

			tt.checkFunc(t)
		})
	}
}

// TestCommitMetrics 测试提交队列统计功能
func TestCommitMetrics(t *testing.T) {
	tests := []struct {
		name       string
		operations int // 执行的写操作数量
		wantWait   bool
		wantCount  int64
	}{
		{
			name:       "SingleWriteOperation",
			operations: 1,
			wantWait:   true,
			wantCount:  1,
		},
		{
			name:       "MultipleWriteOperations",
			operations: 3,
			wantWait:   true,
			wantCount:  3,
		},
		{
			name:       "NoWriteOperation",
			operations: 0,
			wantWait:   false,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupBaseTest(t)
			defer ResetBaseTest(t)
			ResetAllResource(t)

			// 重置计数器
			Metrics().Commit().Count(true)

			// 创建读写上下文
			Watch(true)

			// 执行写操作
			for i := 0; i < tt.operations; i++ {
				model := NewTestBaseModel()
				model.ID = Incre(model)
				model.IntVal = i
				model.FloatVal = float64(i) + 0.5
				model.StringVal = "test_string"
				model.BoolVal = true
				Write(model)
			}

			// 提交事务
			Defer()

			// 验证等待状态
			if tt.wantWait {
				// initialWait := CWait()
				// if initialWait <= 0 {
				// 	t.Error("Should have pending operations")
				// }
			}

			// 等待异步操作完成
			FlushNow()

			// 验证总操作数
			count := Metrics().Commit().Count(false)
			if count != tt.wantCount {
				t.Errorf("CommitCount = %v, want %v", count, tt.wantCount)
			}

			// 验证重置功能
			Metrics().Commit().Count(true)
			if count := Metrics().Commit().Count(false); count != 0 {
				t.Errorf("CommitCount after reset = %v, want 0", count)
			}

			// 验证数据写入结果
			if tt.operations > 0 {
				for i := 0; i < tt.operations; i++ {
					nmodel := NewTestBaseModel()
					if !nmodel.Read(Condition("int_val == {0}", i)) {
						t.Errorf("Operation %d: Data should be written", i)
					}
				}
			}
		})
	}
}

// TestCommitMetricsReset 测试提交队列统计重置功能
func TestCommitMetricsReset(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)
	// 重置计数器
	Metrics().Commit().Count(true)

	// 执行一些写操作
	Watch(true)
	model := NewTestBaseModel()
	Write(model)
	Defer()
	// 等待异步操作完成
	FlushNow()

	// 获取当前计数
	count := Metrics().Commit().Count(false)
	if count <= 0 {
		t.Error("CommitCount should be positive before reset")
	}
	// 测试不重置的获取
	count2 := Metrics().Commit().Count(false)
	if count2 != count {
		t.Errorf("CommitCount changed value from %v to %v", count, count2)
	}

	// 测试重置
	count3 := Metrics().Commit().Count(true)
	if count3 != count {
		t.Errorf("CommitCount(true) returned %v, want %v", count3, count)
	}
	// 验证重置后的值
	count4 := Metrics().Commit().Count(false)
	if count4 != 0 {
		t.Errorf("CommitCount after reset = %v, want 0", count4)
	}
}

// TestCommitFlush 测试提交队列刷新功能
func TestCommitFlush(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 创建多个写操作
	Watch(true)
	for i := 0; i < 3; i++ {
		model := NewTestBaseModel()
		model.ID = i + 1
		Write(model)
	}
	Defer()

	// 执行刷新
	FlushNow()

	// 验证所有批次已处理完成
	if wait := Metrics().Commit().Wait(); wait != 0 {
		t.Errorf("Expected no pending batches after flush, got %d", wait)
	}

	// 验证计数器已更新
	if count := Metrics().Commit().Count(false); count != 3 {
		t.Errorf("Expected commit count of 3, got %d", count)
	}
}

// TestCommitClose 测试提交队列关闭功能
func TestCommitClose(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 创建写操作
	Watch(true)
	model := NewTestBaseModel()
	Write(model)
	Defer()

	// 关闭提交队列
	Close()

	// 验证关闭状态
	if atomic.LoadInt32(&sharedCommitClosed) != 1 {
		t.Error("Expected commit system to be closed")
	}

	// 尝试在关闭后写入
	Watch(true)
	model2 := NewTestBaseModel()
	Write(model2)
	Defer()

	// 验证新的写入被阻止
	queue := getCommitBatch(goid.Get())
	if queue != nil {
		t.Error("Expected nil commit batch queue after close")
	}

	// 验证所有批次已处理完成
	if wait := Metrics().Commit().Wait(); wait != 0 {
		t.Errorf("Expected no pending batches after close, got %d", wait)
	}
}

// TestConcurrentFlush 测试并发刷新功能
func TestConcurrentFlush(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 创建多个写操作
	Watch(true)
	for i := 0; i < 10; i++ {
		model := NewTestBaseModel()
		model.ID = i + 1
		Write(model)
	}
	Defer()

	// 并发执行flush
	var wg sync.WaitGroup
	concurrent := 5
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			Flush()
		}()
	}

	// 等待所有flush完成
	wg.Wait()

	// 验证所有批次已处理完成
	if wait := Metrics().Commit().Wait(); wait != 0 {
		t.Errorf("Expected no pending batches after concurrent flush, got %d", wait)
	}

	// 验证计数器已正确更新（不应该重复计数）
	if count := Metrics().Commit().Count(false); count != 10 {
		t.Errorf("Expected commit count of 10, got %d", count)
	}

	// 验证数据已正确写入
	for i := 0; i < 10; i++ {
		model := NewTestBaseModel()
		model.ID = i + 1
		result := Read(model)
		if !result.IsValid() {
			t.Errorf("Data with ID %d should exist after flush", i+1)
		}
	}
}

// TestMultiBatchFlush 测试多批次刷新功能
func TestMultiBatchFlush(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 使用多个goroutine创建多个batch
	writerCount := 3 // 写入者数量
	batchSize := 5   // 每个batch的大小
	batchCount := 4  // 每个写入者创建的batch数量
	totalCount := writerCount * batchSize * batchCount

	var wg sync.WaitGroup
	wg.Add(writerCount)

	// 启动多个写入者
	for w := 0; w < writerCount; w++ {
		go func(writerID int) {
			defer wg.Done()

			baseID := writerID * batchSize * batchCount
			// 每个写入者创建多个batch
			for b := 0; b < batchCount; b++ {
				Watch(true)
				// 每个batch包含多个写操作
				for i := 0; i < batchSize; i++ {
					model := NewTestBaseModel()
					model.ID = baseID + b*batchSize + i + 1
					model.IntVal = writerID
					model.StringVal = fmt.Sprintf("writer_%d_batch_%d", writerID, b)
					Write(model)
				}
				Defer()
				// 在batch之间添加一些随机延迟
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(w)
	}

	// 等待所有写入操作完成
	wg.Wait()

	// 执行刷新
	Flush()

	// 验证所有批次已处理完成
	if wait := Metrics().Commit().Wait(); wait != 0 {
		t.Errorf("Expected no pending batches after flush, got %d", wait)
	}

	// 验证计数器已正确更新
	if count := Metrics().Commit().Count(false); count != int64(totalCount) {
		t.Errorf("Expected commit count of %d, got %d", totalCount, count)
	}

	// 验证数据已正确写入且保持一致性
	for w := 0; w < writerCount; w++ {
		baseID := w * batchSize * batchCount
		for b := 0; b < batchCount; b++ {
			for i := 0; i < batchSize; i++ {
				model := NewTestBaseModel()
				model.ID = baseID + b*batchSize + i + 1
				result := Read(model)
				if !result.IsValid() {
					t.Errorf("Data with ID %d should exist after flush", model.ID)
					continue
				}
				// 验证数据一致性
				if result.IntVal != w {
					t.Errorf("Data with ID %d has wrong IntVal: got %d, want %d", model.ID, result.IntVal, w)
				}
				expectedStr := fmt.Sprintf("writer_%d_batch_%d", w, b)
				if result.StringVal != expectedStr {
					t.Errorf("Data with ID %d has wrong StringVal: got %s, want %s", model.ID, result.StringVal, expectedStr)
				}
			}
		}
	}
}

// TestComplexQuit 测试在处理批次时收到退出信号的情况
func TestComplexQuit(t *testing.T) {
	SetupBaseTest(t)
	defer ResetBaseTest(t)
	ResetAllResource(t)

	// 创建一些写操作
	batchSize := 5
	Watch(true)
	for i := 0; i < batchSize; i++ {
		model := NewTestBaseModel()
		model.ID = i + 1
		model.StringVal = fmt.Sprintf("data_%d", i)
		Write(model)
	}
	Defer()

	//quit.BroadcastShutdown()
	go func() { Close() }()
	Close()
	go func() { Flush() }()

	// 执行Flush，此时应该会等待所有处理器完成
	Flush()
	Close()
	Flush()
	// 验证所有数据都被正确处理
	count := Metrics().Commit().Count(false)
	if count != int64(batchSize) {
		t.Errorf("Expected %d operations to be processed, got %d", batchSize, count)
	}

	// 验证所有数据都被正确写入
	for i := 0; i < batchSize; i++ {
		model := NewTestBaseModel()
		model.ID = i + 1
		result := Read(model)
		if !result.IsValid() {
			t.Errorf("Data with ID %d should exist after flush", i+1)
			continue
		}

		expectedVal := fmt.Sprintf("data_%d", i)
		if result.StringVal != expectedVal {
			t.Errorf("Data with ID %d has wrong value: got %s, want %s", i+1, result.StringVal, expectedVal)
		}
	}

	// 验证系统状态
	if Metrics().Commit().Wait() != 0 {
		t.Error("No batches should be pending after flush")
	}

	// 验证新的写操作会被拒绝（因为系统已经退出）
	Watch(true)
	model := NewTestBaseModel()
	model.ID = batchSize + 1
	model.StringVal = "after_quit"
	Write(model)
	Defer()

	Flush()

	// 验证新的写操作被拒绝
	model = NewTestBaseModel()
	result := Read(model)
	if result.IsValid() {
		t.Error("New write operation after quit signal should be rejected")
	}
}

// 清除全局commit缓存
func clearCommitMap() {
	initSigMap = sync.Map{}
	commitMap = sync.Map{}
	commitWaitGroup = sync.WaitGroup{}
	flushSigMap = sync.Map{}
	atomic.SwapInt64(&Metrics().Commit().wait, 0)
	Metrics().Commit().Count(true)
	atomic.StoreInt32(&sharedCommitFlush, 0)
	atomic.StoreInt32(&sharedCommitClosed, 0)
}
