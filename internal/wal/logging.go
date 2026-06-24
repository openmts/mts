package wal

import (
	"context"
	"log/slog"
)

// nopHandler 是 slog.Handler 的空操作实现，用于 nil Logger 归一化。
// Enabled 恒返回 false，Handle/WithAttrs/WithGroup 均为空操作，
// 保证下游代码永不需 nil 检查且零开销。
type nopHandler struct{}

func (nopHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (nopHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h nopHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return h }
func (h nopHandler) WithGroup(_ string) slog.Handler             { return h }

// nopLogger 返回一个丢弃所有日志的 *slog.Logger，零开销。
func nopLogger() *slog.Logger {
	return slog.New(nopHandler{})
}
