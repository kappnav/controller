/*
Copyright 2019 IBM Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
A new function package to manage logging in App Navigator Golang components
*/

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

/*LogLevel values of LogLevel. LogLevel is what user requests*/
type LogLevel int

const (
	// LogLevelNone request no trace
	LogLevelNone LogLevel = 0
	// LogLevelWarning request warning trace
	LogLevelWarning LogLevel = 1
	// LogLevelError request error trace
	LogLevelError LogLevel = 2
	// LogLevelInfo request info trace
	LogLevelInfo LogLevel = 3
	// LogLevelDebug request debug trace
	LogLevelDebug LogLevel = 4
	// LogLevelEntry request entry trace
	LogLevelEntry LogLevel = 5
	// LogLevelAll request all traces
	LogLevelAll LogLevel = 6
)

/*LogType values of LogType. LogType is how code categorizes log message*/
type LogType int

const (
	// LogTypeEntry entry trace type
	LogTypeEntry LogType = 0
	// LogTypeExit exit trace type
	LogTypeExit LogType = 1
	// LogTypeInfo info trace type
	LogTypeInfo LogType = 2
	// LogTypeWarning warning trace type
	LogTypeWarning LogType = 3
	// LogTypeError error trace type
	LogTypeError LogType = 4
	// LogTypeDebug debug trace type
	LogTypeDebug LogType = 5
)

/*logType array*/
var logTypes = [6]string{
	"ENTRY",
	"EXIT",
	"INFO",
	"WARNING",
	"ERROR",
	"DEBUG",
}

/*logLevel array*/
var logLevels = [7]string{
	"LogLevelNone",
	"LogLevelWarning",
	"LogLevelError",
	"LogLevelInfo",
	"LogLevelDebug",
	"LogLevelEntry",
	"LogLevelAll",
}

/*Logger interfaces*/
type Logger interface {
	SetLogLevel(logLevel LogLevel)
	Log(callerName string, logType LogType, logData string)
	IsEnabled(logType LogType) bool
}

/*NewLogger create new Logger*/
func NewLogger(enableJSON bool) Logger {	
	l := &loggerImpl{}
	l.enableJSONLog = enableJSON  //true: log in JSON format, false: log in plain text
	// Set default log level
	l.SetLogLevel(LogLevelInfo)
	return l
}

type loggerImpl struct {
	LogLevel       LogLevel
	LogTypeEnabled [6]bool
	enableJSONLog bool
}

//Message JSON structure
type Message struct {
	Level     string `json:"level"`
	Timestamp float64 `json:"ts"`
	Caller    string `json:"caller"`
	Message   string `json:"msg"`
}


/*Log write log entry to stdout.
  Use getLogMessage func to format message
*/
func (logger *loggerImpl) Log(callerName string, logType LogType, logData string) {	
	if logger.enableJSONLog {
		logger.logInJSON(callerName, logType, logData)
	} else {
		logger.logInPlainText(callerName, logType, logData)
	}	
}

//logInPlainText log message in plain text
func (logger *loggerImpl) logInPlainText(callerName string, logType LogType, logData string) {
	str := "[" + time.Now().Format(time.RFC3339) + " " + logTypes[logType] + " " + callerName + "] " + logger.getLogMessage(logType, logData)
	fmt.Println(str)
}

//logInJSON log message in JSON format
func (logger *loggerImpl) logInJSON(callerName string, logType LogType, logData string) {
	log := Message{
		Level:     logTypes[logType],
		Timestamp: FormatTimestamp(time.Now()),  //use unix seconds float format
		Caller:    callerName,
		Message:   logData,
	}
	data, _ := json.Marshal(log)
	// Convert bytes to string
	str := string(data)
	fmt.Println(str)
}


/*isEnabled guard function to test if desired logType is enabled */
func (logger *loggerImpl) IsEnabled(logType LogType) bool {
	return logger.LogTypeEnabled[logType]
}

/*getLogMessage return log message as string in format:
  [LogType] logData
*/
func (logger *loggerImpl) getLogMessage(logType LogType, logData string) string {
	var msg string
	if logType == LogTypeError {
		msg = ErrorWithStack(logData)
	} else {
		msg = logData
	}
	return msg
}

/*setLogTypes set log types */
func (logger *loggerImpl) setLogTypes(value bool) {
	for index := range logTypes {
		logger.LogTypeEnabled[index] = value
	}
}

/*SetLogLevel set global log level to specified value
  set IsEnabled based on specified LogLevel as follows:

  Log Level	| Enabled Log Types
  -------------+----------------------------------------
  none	        |  set all to false
  error			|  error
  warning		|  error, warning
  info	        |  error, warning, info
  debug			|  error, warning, info, debug
  entry			|  error, warning, info, entry, exit, debug
  all	        |  error, warning, info, entry, exit, debug
*/
func (logger *loggerImpl) SetLogLevel(logLevel LogLevel) {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "Logging level set to "+logLevels[logLevel])
	}
	logger.setLogTypes(false)

	switch logLevel {
	case LogLevelNone:
		break
	case LogLevelError:
		logger.LogTypeEnabled[LogTypeError] = true
		break
	case LogLevelWarning:
		logger.LogTypeEnabled[LogTypeError] = true
		logger.LogTypeEnabled[LogTypeWarning] = true
		break
	case LogLevelInfo:
		logger.LogTypeEnabled[LogTypeError] = true
		logger.LogTypeEnabled[LogTypeWarning] = true
		logger.LogTypeEnabled[LogTypeInfo] = true
		break
	case LogLevelDebug:
		logger.LogTypeEnabled[LogTypeError] = true
		logger.LogTypeEnabled[LogTypeWarning] = true
		logger.LogTypeEnabled[LogTypeInfo] = true
		logger.LogTypeEnabled[LogTypeDebug] = true
		break
	case LogLevelEntry:
		logger.setLogTypes(true)
		break
	case LogLevelAll:
		logger.setLogTypes(true)
		break
	}
}
