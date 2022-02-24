/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: log.go
 * @Created: 2021-08-01 11:09:18
 * @Modified:  2022-02-24 17:13:20
 */

package log

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Level uint8

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

type Logger struct {
	L    *zerolog.Logger
	out  io.Writer
	skip int
}

type Arg struct {
	Key   string
	Value interface{}
}

func guessType(l *zerolog.Event, args ...Arg) *zerolog.Event {
	if len(args) > 0 {
		for _, arg := range args {
			switch v := arg.Value.(type) {
			case string:
				l = l.Str(arg.Key, v)
			case int:
				l = l.Int(arg.Key, v)
			case int8:
				l = l.Int8(arg.Key, v)
			case int16:
				l = l.Int16(arg.Key, v)
			case int32:
				l = l.Int32(arg.Key, v)
			case int64:
				l = l.Int64(arg.Key, v)
			case uint:
				l = l.Uint(arg.Key, v)
			case uint8:
				l = l.Uint8(arg.Key, v)
			case uint16:
				l = l.Uint16(arg.Key, v)
			case uint32:
				l = l.Uint32(arg.Key, v)
			case uint64:
				l = l.Uint64(arg.Key, v)
			case float32:
				l = l.Float32(arg.Key, v)
			case float64:
				l = l.Float64(arg.Key, v)
			case bool:
				l = l.Bool(arg.Key, v)
			case []int:
				l = l.Ints(arg.Key, v)
			case []int8:
				l = l.Ints8(arg.Key, v)
			case []int16:
				l = l.Ints16(arg.Key, v)
			case []int32:
				l = l.Ints32(arg.Key, v)
			case []int64:
				l = l.Ints64(arg.Key, v)
			case []uint:
				l = l.Uints(arg.Key, v)
			case []uint32:
				l = l.Uints32(arg.Key, v)
			case []uint64:
				l = l.Uints64(arg.Key, v)
			case []float32:
				l = l.Floats32(arg.Key, v)
			case []float64:
				l = l.Floats64(arg.Key, v)
			case []bool:
				l = l.Bools(arg.Key, v)
			case []string:
				l = l.Strs(arg.Key, v)
			case []byte:
				l = l.Bytes(arg.Key, v)
			case error:
				l = l.AnErr(arg.Key, v)
			case []error:
				l = l.Errs(arg.Key, v)
			case time.Time:
				l = l.Time(arg.Key, v)
			case []time.Time:
				l = l.Times(arg.Key, v)
			case time.Duration:
				l = l.Dur(arg.Key, v)
			case []time.Duration:
				l = l.Durs(arg.Key, v)
			case net.IP:
				l = l.IPAddr(arg.Key, v)
			case net.IPNet:
				l = l.IPPrefix(arg.Key, v)
			case net.HardwareAddr:
				l = l.MACAddr(arg.Key, v)
			case fmt.Stringer:
				l = l.Stringer(arg.Key, v)
			case []fmt.Stringer:
				l = l.Stringers(arg.Key, v)
			default:
				l = l.Interface(arg.Key, v)
			}
		}
	}
	return l
}

func (log *Logger) Debug(msg string, args ...Arg) {
	l := log.L.Debug().Caller(log.skip)
	l = guessType(l, args...)
	l.Msg(msg)
}

func (log *Logger) Info(msg string, args ...Arg) {
	l := log.L.Info()
	l = guessType(l, args...)
	l.Msg(msg)
}

func (log *Logger) Warning(msg string, args ...Arg) {
	l := log.L.Warn().Caller(log.skip)
	l = guessType(l, args...)
	l.Msg(msg)
}

func (log *Logger) Error(err error, args ...Arg) {
	l := log.L.Error().Caller(log.skip).Err(err)
	l = guessType(l, args...)
	l.Send()
}

func (log *Logger) Fatal(err error, args ...Arg) {
	l := log.L.Fatal().Caller(log.skip).Err(err)
	l = guessType(l, args...)
	l.Send()
}

func (log *Logger) SetLevel(level Level) {
	logger := zerolog.New(log.out).
		Level(func() zerolog.Level {
			// 环境变量是 DEBUG 时，优先设置日志等级为 DEBUG
			if IsDebug() {
				return zerolog.DebugLevel
			} else {
				return zerolog.Level(level)
			}
		}()).
		With().
		Timestamp().
		Logger()

	log.L = &logger
}

func IsDebug() bool {
	return os.Getenv("DEBUG") != "" && os.Getenv("DEBUG") != "0" && strings.ToLower(os.Getenv("DEBUG")) != "false"
}

// NewLogger returns a new zerolog instance
func NewLogger(level Level, out io.Writer, skip ...int) *Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logger := zerolog.New(out).
		Level(func() zerolog.Level {
			// 环境变量是 DEBUG 时，优先设置日志等级为 DEBUG
			if IsDebug() {
				return zerolog.DebugLevel
			} else {
				return zerolog.Level(level)
			}
		}()).
		With().
		Timestamp().
		Logger()

	l := new(Logger)
	l.L = &logger

	l.out = out

	if len(skip) > 0 {
		l.skip = skip[0]
	} else {
		l.skip = 1
	}

	return l
}

func ToConsole() io.Writer {
	return zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
}

func fileWriter(filepath string) (io.Writer, error) {
	return os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func ToFile(filename string) (io.Writer, error) {
	writer, err := fileWriter(filename)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func MustToFile(filename string) io.Writer {
	writer, err := fileWriter(filename)
	if err != nil {
		panic(err)
	}
	return writer
}

func ToConsoleAndFile(filepath string) (io.Writer, error) {
	fw, err := fileWriter(filepath)
	if err != nil {
		return nil, err
	}
	return zerolog.MultiLevelWriter(fw, zerolog.ConsoleWriter{Out: os.Stdout}), nil
}

func MustToConsoleAndFile(filepath string) io.Writer {
	fw, err := fileWriter(filepath)
	if err != nil {
		panic(err)
	}
	return zerolog.MultiLevelWriter(fw, zerolog.ConsoleWriter{Out: os.Stdout})
}
