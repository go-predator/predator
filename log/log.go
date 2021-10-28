/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: log.go
 * @Created: 2021-08-01 11:09:18
 * @Modified: 2021-10-28 10:54:47
 */

package log

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

const (
	DEBUG   = zerolog.DebugLevel
	INFO    = zerolog.InfoLevel
	WARNING = zerolog.WarnLevel
	ERROR   = zerolog.ErrorLevel
	FATAL   = zerolog.FatalLevel
)

// NewLogger returns a new zerolog instance
func NewLogger(level zerolog.Level, out io.Writer) *zerolog.Logger {
	logger := zerolog.New(out).
		Level(func() zerolog.Level {
			// 环境变量是 DEBUG 时，优先设置日志等级为 DEBUG
			if os.Getenv("DEBUG") != "" {
				return zerolog.DebugLevel
			} else {
				return level
			}
		}()).
		With().
		Timestamp().
		Logger()
	return &logger
}
