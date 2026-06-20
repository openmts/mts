package queryexec

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

const (
	cursorMagic   uint32 = 0x4d545343
	cursorVersion uint8  = 1
	cursorSize           = 22
)

var ErrInvalidCursor = errors.New("invalid query cursor")

type CursorPosition struct {
	SeriesID  uint64
	Timestamp int64
	Direction model.QuerySortDirection
}

type cursorRowStream struct {
	source   RowStream
	position CursorPosition
	current  model.Row
}

type cursorColumnStream struct {
	source   ColumnStream
	position CursorPosition
	current  model.ColumnSeries
}

func EncodeCursor(position CursorPosition) (string, error) {
	if !validCursorDirection(position.Direction) {
		return "", fmt.Errorf("%w: unsupported direction %d", ErrInvalidCursor, position.Direction)
	}
	var raw [cursorSize]byte
	binary.BigEndian.PutUint32(raw[0:4], cursorMagic)
	raw[4] = cursorVersion
	raw[5] = byte(position.Direction)
	binary.BigEndian.PutUint64(raw[6:14], position.SeriesID)
	binary.BigEndian.PutUint64(raw[14:22], uint64(position.Timestamp))
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func DecodeCursor(token string) (CursorPosition, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return CursorPosition{}, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}
	if len(raw) != cursorSize ||
		binary.BigEndian.Uint32(raw[0:4]) != cursorMagic ||
		raw[4] != cursorVersion {
		return CursorPosition{}, ErrInvalidCursor
	}
	position := CursorPosition{
		Direction: model.QuerySortDirection(raw[5]),
		SeriesID:  binary.BigEndian.Uint64(raw[6:14]),
		Timestamp: int64(binary.BigEndian.Uint64(raw[14:22])),
	}
	if !validCursorDirection(position.Direction) {
		return CursorPosition{}, fmt.Errorf("%w: unsupported direction %d", ErrInvalidCursor, position.Direction)
	}
	return position, nil
}

func CursorFromRow(row model.Row, order model.QueryOrder) CursorPosition {
	return CursorPosition{
		SeriesID:  row.SeriesID,
		Timestamp: row.Timestamp,
		Direction: cursorDirection(order),
	}
}

func NewCursorRowStream(source RowStream, position CursorPosition) RowStream {
	return &cursorRowStream{source: source, position: position}
}

func NewCursorColumnStream(source ColumnStream, position CursorPosition) ColumnStream {
	return &cursorColumnStream{source: source, position: position}
}

func (s *cursorRowStream) Next() bool {
	for s.source != nil && s.source.Next() {
		row := s.source.Row()
		if rowAfterCursor(row.SeriesID, row.Timestamp, s.position) {
			s.current = row
			return true
		}
	}
	return false
}

func (s *cursorRowStream) Row() model.Row {
	return s.current
}

func (s *cursorRowStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *cursorRowStream) Close() error {
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func (s *cursorColumnStream) Next() bool {
	for s.source != nil && s.source.Next() {
		column := filterColumnAfterCursor(s.source.Column(), s.position)
		if len(column.Values) > 0 {
			s.current = column
			return true
		}
	}
	return false
}

func (s *cursorColumnStream) Column() model.ColumnSeries {
	return s.current
}

func (s *cursorColumnStream) Err() error {
	if s.source == nil {
		return nil
	}
	return s.source.Err()
}

func (s *cursorColumnStream) Close() error {
	if s.source == nil {
		return nil
	}
	return s.source.Close()
}

func filterColumnAfterCursor(column model.ColumnSeries, position CursorPosition) model.ColumnSeries {
	out := column
	out.Timestamps = out.Timestamps[:0:0]
	out.Values = out.Values[:0:0]
	for index, timestamp := range column.Timestamps {
		if rowAfterCursor(column.SeriesID, timestamp, position) {
			out.Timestamps = append(out.Timestamps, timestamp)
			out.Values = append(out.Values, column.Values[index])
		}
	}
	return out
}

func rowAfterCursor(seriesID uint64, timestamp int64, position CursorPosition) bool {
	if position.Direction == model.QuerySortDesc {
		if timestamp != position.Timestamp {
			return timestamp < position.Timestamp
		}
		return seriesID > position.SeriesID
	}
	if timestamp != position.Timestamp {
		return timestamp > position.Timestamp
	}
	return seriesID > position.SeriesID
}

func cursorDirection(order model.QueryOrder) model.QuerySortDirection {
	if order.By == model.QueryOrderByTime && order.Direction == model.QuerySortDesc {
		return model.QuerySortDesc
	}
	return model.QuerySortAsc
}

func validCursorDirection(direction model.QuerySortDirection) bool {
	return direction == model.QuerySortAsc || direction == model.QuerySortDesc
}
