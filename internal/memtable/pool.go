package memtable

import (
	"sync"

	"github.com/openmts/mts/internal/model"
)

const (
	maxPooledTableColumns   = 1 << 16
	maxPooledTableMaps      = 4
	maxPooledColumnKeys     = 1 << 16
	maxPooledColumnKeySets  = 4
	maxPooledColumnCapacity = 16
	maxPooledColumnBuffers  = 1 << 16
)

var tableDataPool tableDataFreeList

var columnBufferPool columnBufferFreeList

var columnKeyPool columnKeyFreeList

var borrowColumnBufferHook func(fromPool bool)

type tableDataFreeList struct {
	mu    sync.Mutex
	items []tableData
}

type columnBufferFreeList struct {
	mu    sync.Mutex
	items []*columnBuffer
}

type columnKeyFreeList struct {
	mu    sync.Mutex
	items [][]columnKey
}

func borrowTableData() tableData {
	data := tableDataPool.get()
	if data == nil {
		return make(tableData)
	}
	clear(data)
	return data
}

func releaseTableData(data tableData) {
	if data == nil {
		return
	}
	count := len(data)
	clear(data)
	if count > maxPooledTableColumns {
		return
	}
	tableDataPool.put(data)
}

func borrowColumnKeys(capacity int) []columnKey {
	keys := columnKeyPool.get(capacity)
	if keys == nil {
		return make([]columnKey, 0, capacity)
	}
	return keys[:0]
}

func releaseColumnKeys(keys []columnKey) {
	if keys == nil {
		return
	}
	if cap(keys) > maxPooledColumnKeys {
		return
	}
	clear(keys)
	columnKeyPool.put(keys[:0])
}

func borrowColumnBuffer(seriesID uint64, fieldID uint32, fieldType model.FieldType) *columnBuffer {
	column, fromPool := columnBufferPool.get()
	if column == nil {
		if borrowColumnBufferHook != nil {
			borrowColumnBufferHook(false)
		}
		column = &columnBuffer{}
	} else if borrowColumnBufferHook != nil {
		borrowColumnBufferHook(fromPool)
	}
	column.seriesID = seriesID
	column.fieldID = fieldID
	column.fieldType = fieldType
	column.count = 0
	column.memBytes = columnBufferBaseBytes + column.capacityBytes()
	return column
}

func releaseColumnBuffer(column *columnBuffer) {
	if column == nil {
		return
	}
	if columnRetainsLargeBacking(column) {
		column.clear()
	} else {
		clear(column.strings)
		column.times = column.times[:0]
		column.writeSeqs = column.writeSeqs[:0]
		column.floats = column.floats[:0]
		column.ints = column.ints[:0]
		column.strings = column.strings[:0]
		column.boolBits = column.boolBits[:0]
	}
	column.seriesID = 0
	column.fieldID = 0
	column.fieldType = 0
	column.count = 0
	column.memBytes = columnBufferBaseBytes + column.capacityBytes()
	columnBufferPool.put(column)
}

func columnRetainsLargeBacking(column *columnBuffer) bool {
	if cap(column.times) > maxPooledColumnCapacity {
		return true
	}
	if cap(column.writeSeqs) > maxPooledColumnCapacity {
		return true
	}
	if cap(column.floats) > maxPooledColumnCapacity {
		return true
	}
	if cap(column.ints) > maxPooledColumnCapacity {
		return true
	}
	if cap(column.strings) > maxPooledColumnCapacity {
		return true
	}
	return cap(column.boolBits) > boolWords(maxPooledColumnCapacity)
}

func (p *tableDataFreeList) get() tableData {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := len(p.items)
	if count == 0 {
		return nil
	}
	data := p.items[count-1]
	p.items[count-1] = nil
	p.items = p.items[:count-1]
	return data
}

func (p *tableDataFreeList) put(data tableData) {
	if data == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) >= maxPooledTableMaps {
		return
	}
	p.items = append(p.items, data)
}

func (p *columnBufferFreeList) get() (*columnBuffer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := len(p.items)
	if count == 0 {
		return nil, false
	}
	column := p.items[count-1]
	p.items[count-1] = nil
	p.items = p.items[:count-1]
	return column, true
}

func (p *columnBufferFreeList) put(column *columnBuffer) {
	if column == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) >= maxPooledColumnBuffers {
		return
	}
	p.items = append(p.items, column)
}

func (p *columnKeyFreeList) get(capacity int) []columnKey {
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := len(p.items) - 1; index >= 0; index-- {
		keys := p.items[index]
		if cap(keys) < capacity {
			continue
		}
		p.items[index] = p.items[len(p.items)-1]
		p.items[len(p.items)-1] = nil
		p.items = p.items[:len(p.items)-1]
		return keys
	}
	return nil
}

func (p *columnKeyFreeList) put(keys []columnKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) >= maxPooledColumnKeySets {
		return
	}
	p.items = append(p.items, keys)
}

func tableDataPoolLenForTest() int {
	tableDataPool.mu.Lock()
	defer tableDataPool.mu.Unlock()
	return len(tableDataPool.items)
}

func resetTableDataPoolForTest() {
	tableDataPool.mu.Lock()
	defer tableDataPool.mu.Unlock()
	clear(tableDataPool.items)
	tableDataPool.items = nil
}

func columnKeyPoolLenForTest() int {
	columnKeyPool.mu.Lock()
	defer columnKeyPool.mu.Unlock()
	return len(columnKeyPool.items)
}

func resetColumnKeyPoolForTest() {
	columnKeyPool.mu.Lock()
	defer columnKeyPool.mu.Unlock()
	clear(columnKeyPool.items)
	columnKeyPool.items = nil
}

func columnBufferPoolLenForTest() int {
	columnBufferPool.mu.Lock()
	defer columnBufferPool.mu.Unlock()
	return len(columnBufferPool.items)
}

func resetColumnBufferPoolForTest() {
	columnBufferPool.mu.Lock()
	defer columnBufferPool.mu.Unlock()
	clear(columnBufferPool.items)
	columnBufferPool.items = nil
}
