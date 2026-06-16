package memtable

import (
	"sync"
)

const (
	maxPooledTableColumns = 1 << 14
)

var tableDataPool = sync.Pool{
	New: func() any {
		data := make(tableData)
		return &data
	},
}

func borrowTableData() tableData {
	ptr, ok := tableDataPool.Get().(*tableData)
	if !ok || ptr == nil {
		return make(tableData)
	}
	data := *ptr
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
	tableDataPool.Put(&data)
}
