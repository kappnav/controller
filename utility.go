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

package main

import (
	"bytes"
	"strings"
	"time"

	"k8s.io/klog"
)

const (
	maxLabelLength = 63  // max length of a label in Kubernetes
	maxNameLength  = 253 // max length of a name in Kubernetes

	// minimum interval between logs for sample logging
	samplingLogInterval = time.Minute * 5
)

/* @Return true if character is valid for a domain name */
func isValidDomainNameChar(ch byte) bool {
	return (ch == '.' || ch == '-' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9'))
}

/* Convert a name to domain name format.
 The name must
 - Start with [a-z0-9]. If not, "0" is prepended.
 - lower case. If not, lower case is used.
 - contain only '.', '-', and [a-z0-9]. If not, "." is used insteaad.
 - end with alpha numeric characters. Otherwise, '0' is appended
 - can't have consecutive '.'.  Consecutivie ".." is substituted with ".".
Return emtpy string if the name is empty after conversion
*/
func toDomainName(name string) string {
	maxLength := maxNameLength
	name = strings.ToLower(name)
	ret := bytes.Buffer{}
	chars := []byte(name)
	for i, ch := range chars {
		if i == 0 {
			// first character must be [a-z0-9]
			if (ch >= 'a' && ch <= 'z') ||
				(ch >= '0' && ch <= '9') {
				ret.WriteByte(ch)
			} else {
				ret.WriteByte('0')
				if isValidDomainNameChar(ch) {
					ret.WriteByte(ch)
				} else {
					ret.WriteByte('.')
				}
			}
		} else {
			if isValidDomainNameChar(ch) {
				ret.WriteByte(ch)
			} else {
				ret.WriteByte('.')
			}
		}
	}

	// change all ".." to ".
	retStr := ret.String()
	for strings.Index(retStr, "..") > 0 {
		retStr = strings.ReplaceAll(retStr, "..", ".")
	}

	strLen := len(retStr)
	if strLen == 0 {
		return retStr
	}
	if strLen > maxLength {
		strLen = maxLength
		retStr = retStr[0:strLen]
	}

	ch := retStr[strLen-1]
	if (ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9') {
		// last char is alphanumeric
		return retStr
	}
	if strLen < maxLength-1 {
		//  append alphanumeric
		return retStr + "0"
	}
	// replace last char to be alphanumeric
	return retStr[0:strLen-2] + "0"
}

func isValidLabelChar(ch byte) bool {
	return (ch == '.' || ch == '-' || (ch == '_') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9'))
}

/* Convert the  name part of  a label
   The name must
   - Start with [a-z0-9A-Z]. If not, "0" is prepended.
   - End with [a-z0-9A-Z]. If not, "0" is appended
   - Intermediate characters can only be: [a-z0-9A-Z] or '_', '-', and '.' If not, '.' is used.
- be maximum maxLabelLength characters long
*/
func toLabelName(name string) string {
	chars := []byte(name)
	ret := bytes.Buffer{}
	for i, ch := range chars {
		if i == 0 {
			// first character must be [a-z0-9]
			if (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') {
				ret.WriteByte(ch)
			} else {
				ret.WriteByte('0')
				if isValidLabelChar(ch) {
					ret.WriteByte(ch)
				} else {
					ret.WriteByte('.')
				}
			}
		} else {
			if isValidLabelChar(ch) {
				ret.WriteByte(ch)
			} else {
				ret.WriteByte('.')
			}
		}
	}

	retStr := ret.String()
	strLen := len(retStr)
	if strLen == 0 {
		return retStr
	}
	if strLen > maxLabelLength {
		strLen = maxLabelLength
		retStr = retStr[0:strLen]
	}

	ch := retStr[strLen-1]
	if (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') {
		// last char is alphanumeric
		return retStr
	} else if strLen < maxLabelLength-1 {
		//  append alphanumeric
		return retStr + "0"
	} else {
		// replace last char to be alphanumeric
		return retStr[0:strLen-2] + "0"
	}
}

func toLabel(input string) string {
	slashIndex := strings.Index(input, "/")
	var prefix, label string
	if slashIndex < 0 {
		prefix = ""
		label = input
	} else if slashIndex == len(input)-1 {
		prefix = input[0:slashIndex]
		label = ""
	} else {
		prefix = input[0:slashIndex]
		label = input[slashIndex+1:]
	}

	newPrefix := toDomainName(prefix)
	newLabel := toLabelName(label)
	ret := ""
	if newPrefix == "" {
		if newLabel == "" {
			// shouldn't happen
			newLabel = "nolabel"
		} else {
			ret = newLabel
		}
	} else if newLabel == "" {
		ret = newPrefix
	} else {
		ret = newPrefix + "/" + newLabel
	}
	return ret
}

// sampling looger will output logs no more fequent than a defined
// time interval. This is used to reduce amount of logs when processing
// errors and retries that may occur frequently
type samplingLogger struct {
	interval   time.Duration // interval between log output
	lastOutput time.Time     // time of last output
}

// Return a new sample logger
func newSamplingLogger() *samplingLogger {
	return &samplingLogger{
		interval:   samplingLogInterval,
		lastOutput: time.Now().Add(-1 * samplingLogInterval),
	}
}

func (sl *samplingLogger) logError(err error) {
	now := time.Now()
	if now.Sub(sl.lastOutput) >= sl.interval {
		// enough time elapsed since last output
		sl.lastOutput = now
		klog.Error(err)
	}
}

func logString(str string) string {
	return "\"" + str + "\""
}
