package sstable

// constStepWindow 计算等间隔时间序列与查询窗口的交集下标范围 [start, end)。
// timestamps[i] = base + i*step。
func constStepWindow(base, step int64, count int, query Query) (start, end int) {
	if count <= 0 || query.End < query.Start {
		return 0, 0
	}
	if step == 0 {
		// 全点同一时间戳。
		if base >= query.Start && base <= query.End {
			return 0, count
		}
		return 0, 0
	}
	// 处理负 step：映射到正 step 等价区间较复杂，回退全量。
	if step < 0 {
		return 0, count
	}
	// first index with base+i*step >= query.Start
	if base >= query.Start {
		start = 0
	} else {
		// ceil((query.Start-base)/step)
		delta := query.Start - base
		start = int((delta + step - 1) / step)
	}
	if start >= count {
		return 0, 0
	}
	// last index with base+i*step <= query.End  => i <= (query.End-base)/step
	endExclusive := int((query.End-base)/step) + 1
	if endExclusive > count {
		endExclusive = count
	}
	if endExclusive <= start {
		return 0, 0
	}
	// 再夹紧边界（防溢出/舍入）
	for start < endExclusive && base+int64(start)*step < query.Start {
		start++
	}
	for endExclusive > start && base+int64(endExclusive-1)*step > query.End {
		endExclusive--
	}
	if endExclusive <= start {
		return 0, 0
	}
	return start, endExclusive
}

func materializeConstStepTimestamps(base, step int64, start, end int) []int64 {
	if end <= start {
		return nil
	}
	out := make([]int64, end-start)
	for index := range out {
		out[index] = base + int64(start+index)*step
	}
	return out
}

// sortedTimestampWindow 对有序 timestamps 二分求落在 [Start,End] 的下标 [lo,hi)。
func sortedTimestampWindow(timestamps []int64, query Query) (lo, hi int) {
	n := len(timestamps)
	if n == 0 || query.End < query.Start {
		return 0, 0
	}
	// lower_bound query.Start
	lo, hi = 0, n
	for lo < hi {
		mid := lo + (hi-lo)/2
		if timestamps[mid] < query.Start {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	start := lo
	// upper_bound query.End
	lo, hi = start, n
	for lo < hi {
		mid := lo + (hi-lo)/2
		if timestamps[mid] <= query.End {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return start, lo
}
