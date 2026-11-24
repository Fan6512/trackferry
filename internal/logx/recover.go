package logx

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// Recover, deferred, logs a panic and lets the rest of the service go on.
func Recover(log *slog.Logger, where string) {
	if p := recover(); p != nil {
		log.Error(where+"　程序出错，已跳过这一步", "原因", fmt.Sprint(p), "位置", string(debug.Stack()))
	}
}
