package log

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jakebowkett/go-gen/gen"
)

type logLevel struct {
	name string
}

func (ll logLevel) String() string {
	return ll.name
}

var (
	levelInfo  = logLevel{"Info"}
	levelError = logLevel{"Error"}
	levelDebug = logLevel{"Debug"}
)

type logKind struct {
	name string
}

func (lk logKind) String() string {
	return lk.name
}

var (
	kindRequest = logKind{"request"}
	kindSession = logKind{"session"}
)

type Log struct {
	Date     time.Time
	Kind     logKind
	ThreadId string
	Route    string
	Status   int
	Duration int
	Entries  []*entry
}

type entry struct {
	logger   *Logger
	EntryId  string
	ThreadId string
	Level    string
	Function string
	File     string
	Message  string
	Line     int
	KeyVals  []kv
}

func (e *entry) Data(k fmt.Stringer, v interface{}) Entry {
	e.KeyVals = append(e.KeyVals, kv{k, v})
	return e
}

type kv struct {
	Key fmt.Stringer
	Val interface{}
}

type Logger struct {
	OnLogEvent     func(Log)
	DisableDebug   bool
	DisableRuntime bool
	logs           sync.Map
}

func (l *Logger) Info(reqId string, msg string) Entry {
	return l.logEntry(levelInfo, reqId, msg)
}
func (l *Logger) Error(reqId string, msg string) Entry {
	return l.logEntry(levelError, reqId, msg)
}
func (l *Logger) Debug(reqId string, msg string) Entry {
	return l.logEntry(levelDebug, reqId, msg)
}

func (l *Logger) InfoF(reqId string, format string, a ...interface{}) Entry {
	return l.logEntry(levelInfo, reqId, fmt.Sprintf(format, a...))
}
func (l *Logger) ErrorF(reqId string, format string, a ...interface{}) Entry {
	return l.logEntry(levelError, reqId, fmt.Sprintf(format, a...))
}
func (l *Logger) DebugF(reqId string, format string, a ...interface{}) Entry {
	return l.logEntry(levelDebug, reqId, fmt.Sprintf(format, a...))
}

func (l *Logger) End(reqId, route string, status, duration int) {
	l.end(kindRequest, reqId, route, status, duration)
}

func (l *Logger) logEntry(level logLevel, threadId, msg string) Entry {

	if l.DisableDebug {
		return &entry{}
	}

	var file string
	var function string
	var line int

	if !l.DisableRuntime {
		pc, fn, ln, ok := runtime.Caller(2)
		if !ok {
			file = "Unable to obtain call site."
			function = "Unknown"
		} else {
			function = runtime.FuncForPC(pc).Name()
			if idx := strings.LastIndex(function, "/"); idx != -1 {
				function = function[idx+1 : len(function)]
			}
			file = fn
		}
		line = ln
	}

	entryId, _ := gen.Base64(16)
	e := &entry{
		logger:   l,
		EntryId:  entryId,
		ThreadId: threadId,
		Level:    level.String(),
		Function: function,
		File:     file,
		Line:     line,
		Message:  msg,
	}

	entries, ok := l.logs.Load(threadId)
	if !ok {
		l.logs.Store(threadId, []*entry{e})
		return e
	}

	// We know the map only has this type as values.
	ee := entries.([]*entry)
	ee = append(ee, e)
	l.logs.Store(threadId, ee)

	return e
}

func (l *Logger) end(kind logKind, threadId, route string, status, duration int) {

	var ee []*entry
	entries, ok := l.logs.Load(threadId)
	if ok {
		l.logs.Delete(threadId)
		ee = entries.([]*entry)
	}

	// Unlike requests there's no value in logging a
	// session with no entries because it doesn't have
	// an overall HTTP status or duration to report.
	if kind == kindSession && len(ee) == 0 {
		return
	}

	if l.OnLogEvent == nil {
		return
	}

	l.OnLogEvent(Log{
		Date:     time.Now(),
		ThreadId: threadId,
		Kind:     kind,
		Route:    route,
		Status:   status,
		Duration: duration,
		Entries:  ee,
	})
}

type session struct {
	logger *Logger
	name   string
	id     string
}

func (l *Logger) Sess(name string) Session {
	id, _ := gen.Base64(16)
	return &session{
		id:     id,
		name:   name,
		logger: l,
	}
}

func (s *session) Info(msg string) Entry {
	return s.logger.logEntry(levelInfo, s.id, msg)
}
func (s *session) Error(msg string) Entry {
	return s.logger.logEntry(levelError, s.id, msg)
}
func (s *session) Debug(msg string) Entry {
	return s.logger.logEntry(levelDebug, s.id, msg)
}

func (s *session) InfoF(format string, a ...interface{}) Entry {
	return s.logger.logEntry(levelInfo, s.id, fmt.Sprintf(format, a...))
}
func (s *session) ErrorF(format string, a ...interface{}) Entry {
	return s.logger.logEntry(levelError, s.id, fmt.Sprintf(format, a...))
}
func (s *session) DebugF(format string, a ...interface{}) Entry {
	return s.logger.logEntry(levelDebug, s.id, fmt.Sprintf(format, a...))
}

func (s *session) End() {
	s.logger.end(kindSession, s.id, s.name, 0, 0)
}