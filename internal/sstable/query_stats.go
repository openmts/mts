package sstable

func recordPartScanned(query Query) {
	if query.Stats != nil {
		query.Stats.PartsScanned++
	}
}

func recordPartSkipped(query Query) {
	if query.Stats != nil {
		query.Stats.PartsSkipped++
	}
}

func recordIndexRowRead(query Query) {
	if query.Stats != nil {
		query.Stats.IndexRowsRead++
	}
}

func recordIndexRowSkipped(query Query) {
	if query.Stats != nil {
		query.Stats.IndexRowsSkipped++
	}
}

func recordTimeBlockRead(query Query) {
	if query.Stats != nil {
		query.Stats.TimeBlocksRead++
	}
}

func recordValueBlockRead(query Query) {
	if query.Stats != nil {
		query.Stats.ValueBlocksRead++
	}
}

func recordValuePageRead(query Query) {
	if query.Stats != nil {
		query.Stats.ValuePagesRead++
	}
}

func recordValuePagesSkipped(query Query, count int) {
	if query.Stats != nil && count > 0 {
		query.Stats.ValuePagesSkipped += count
	}
}

func recordSamplesRead(query Query, count int) {
	if query.Stats != nil && count > 0 {
		query.Stats.SamplesRead += count
	}
}
