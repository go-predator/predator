/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: log.go
 * @Created:   2021-08-01 11:09:18
 * @Modified:  2022-03-29 15:11:43
 */

package log

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Level defines the log level
type Level uint8

// log level
const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

var (
	// Default time format with nanosecond precision
	TimeFormat = "2006-01-02 15:04:05.999999999"

	// Console default time format with millisecond accuracy
	ConsoleTimeFormat = "15:04:05.000"
)

// Logger records a `zerolog.Logger` pointer and uses this pointer
// to implement all logging methods
type Logger struct {
	L    *zerolog.Logger
	out  io.Writer
	skip int
}

// Arg records the parameters required in the log as key-value pairs,
// the key is of type `string`, and the value can be of any type.
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

// Debug logs a `DEBUG` message with some `Arg`s.
func (log *Logger) Debug(msg string, args ...Arg) {
	l := log.L.Debug().Caller(log.skip)
	l = guessType(l, args...)
	l.Msg(msg)
}

// Info logs a `INFO` message with some `Arg`s.
func (log *Logger) Info(msg string, args ...Arg) {
	l := log.L.Info()
	l = guessType(l, args...)
	l.Msg(msg)
}

// Warning logs a `WARNING` message with some `Arg`s.
func (log *Logger) Warning(msg string, args ...Arg) {
	l := log.L.Warn().Caller(log.skip)
	l = guessType(l, args...)
	l.Msg(msg)
}

func validte(err any) error {
	var e error

	switch t := err.(type) {
	case string:
		e = errors.New(t)
	case error:
		e = t
	default:
		panic("type not allowed")
	}

	return e
}

// Error logs a `ERROR` message with some `Arg`s.
func (log *Logger) Error(err any, args ...Arg) {

	l := log.L.Error().Caller(log.skip).Err(validte(err))
	l = guessType(l, args...)
	l.Send()
}

// Fatal logs a `FATAL` message with some `Arg`s, and calls `os.Exit(1)`
// to exit the application.
func (log *Logger) Fatal(err any, args ...Arg) {
	l := log.L.Error().Caller(log.skip).Err(validte(err))
	l = guessType(l, args...)
	l.Send()
}

func newZerologLogger(level Level, out io.Writer) zerolog.Logger {
	return zerolog.New(out).
		Level(func() zerolog.Level {
			// If the current running environment is DEBUG,
			// set the level to DEBUG first
			if IsDebug() {
				return zerolog.DebugLevel
			} else {
				return zerolog.Level(level)
			}
		}()).
		With().
		Timestamp().
		Logger()
}

// SetLevel will create a `zerolog.Logger` instance with a new `LEVEL`
// using the existing `out`(io.Writer)
func (log *Logger) SetLevel(level Level) {
	logger := newZerologLogger(level, log.out)

	log.L = &logger
}

func (log Logger) Out() io.Writer {
	return log.out
}

// IsDebug determines whether the current environment is `DEBUG` through
// the `DEBUG` variable in the current environment variables.
func IsDebug() bool {
	return os.Getenv("DEBUG") != "" &&
		os.Getenv("DEBUG") != "0" &&
		strings.ToLower(os.Getenv("DEBUG")) != "false"
}

// NewLogger returns a new `Logger` pointer.
func NewLogger(level Level, out io.Writer, skip ...int) *Logger {
	zerolog.TimeFieldFormat = TimeFormat
	logger := newZerologLogger(level, out)

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

// ToConsole returns an `io.Writer` that outputs the log to the
// console or terminal emulator or an `error`.
func ToConsole() io.Writer {
	return zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: ConsoleTimeFormat}
}

func fileWriter(filepath string, flag int) (io.Writer, error) {
	if flag < 0 {
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	}
	return os.OpenFile(filepath, flag, 0666)
}

// ToFile returns an `io.Writer` that saves the log to a local file or an `error`.
//
// The `flag` parameter is passed to the `os.OpenFile` function.
// If flag < 0, it will be assigned the value `os.O_RDWR|os.O_CREATE|os.O_TRUNC`.
func ToFile(filename string, flag int) (io.Writer, error) {
	writer, err := fileWriter(filename, flag)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

// MustToFile returns an `io.Writer` that saves the log to a local file and will
// panic if there is an error opening the file.
//
// The `flag` parameter is passed to the `os.OpenFile` function.
// If flag < 0, it will be assigned the value `os.O_RDWR|os.O_CREATE|os.O_TRUNC`.
func MustToFile(filename string, flag int) io.Writer {
	writer, err := fileWriter(filename, flag)
	if err != nil {
		panic(err)
	}
	return writer
}

// ToConsoleAndFile returns an `io.Writer` that can both output the log to a
// control or terminal emulator and save the log to a local file, or an `error`.
//
// The `flag` parameter is passed to the `os.OpenFile` function.
// If flag < 0, it will be assigned the value `os.O_RDWR|os.O_CREATE|os.O_TRUNC`.
func ToConsoleAndFile(filepath string, flag int) (io.Writer, error) {
	fw, err := fileWriter(filepath, flag)
	if err != nil {
		return nil, err
	}
	return zerolog.MultiLevelWriter(fw, ToConsole()), nil
}

// MustToConsoleAndFile returns an `io.Writer` that can both output the log to a
// control or terminal emulator and save the log to a local file, which will
// panic if there is an error opening the file.
//
// The `flag` parameter is passed to the `os.OpenFile` function.
// If flag < 0, it will be assigned the value `os.O_RDWR|os.O_CREATE|os.O_TRUNC`.
func MustToConsoleAndFile(filepath string, flag int) io.Writer {
	fw, err := fileWriter(filepath, flag)
	if err != nil {
		panic(err)
	}
	return zerolog.MultiLevelWriter(fw, ToConsole())
}
