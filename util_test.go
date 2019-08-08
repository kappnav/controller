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
	"testing"
)

type stringTestData struct {
	before string
	after  string
}

var toDomainNameTestData = []stringTestData{
	{before: "abc.def.ghi", after: "abc.def.ghi"},
	{before: "abc-def.ghi", after: "abc-def.ghi"},
	{before: "abc-def...............ghi", after: "abc-def.ghi"},
	{before: "abc-def!@#$%^&*()_+-={}|[]\\/.,<>ghi", after: "abc-def.-.ghi"},
	{before: "ABC-DEF-GHI", after: "abc-def-ghi"},
	{before: "0ABC-DEF-GHI", after: "0abc-def-ghi"},
	{before: "?*()*<///ABC-DEF-GHI", after: "0.abc-def-ghi"},
	{before: "?*()*<///ABC-DEF-GHI.", after: "0.abc-def-ghi.0"},
}

func TestToDomainName(t *testing.T) {
	for _, data := range toDomainNameTestData {
		result := toDomainName(data.before)
		if result != data.after {
			t.Errorf("toDomainName of %s failed: expecting %s got received %s", data.before, data.after, result)
		}
	}
}

var toLabelTestData = []stringTestData{
	{before: "abc.def.ghi", after: "abc.def.ghi"},
	{before: "abc-def-ghi", after: "abc-def-ghi"},
	{before: "ABC-def-ghi", after: "ABC-def-ghi"},
	{before: "ABC-def_ghi", after: "ABC-def_ghi"},
	{before: "abc.def.ghi/ABC-def-ghi", after: "abc.def.ghi/ABC-def-ghi"},
	{before: "ABC.def.ghi/ABC-def-ghi", after: "abc.def.ghi/ABC-def-ghi"},
	{before: "ABC.def.ghi/ABC-def-ghi", after: "abc.def.ghi/ABC-def-ghi"},
	{before: "ABC.def.ghi/ABC/-def-ghi", after: "abc.def.ghi/ABC.-def-ghi"},
	{before: "/ABC-def-ghi", after: "ABC-def-ghi"},
	{before: "ABC.def.ghi%", after: "ABC.def.ghi.0"},
}

func TestToLabel(t *testing.T) {
	for _, data := range toLabelTestData {
		result := toLabel(data.before)
		if result != data.after {
			t.Errorf("toLabel of %s failed: expecting %s got received %s", data.before, data.after, result)
		}
	}
}

type matchLabelTestData struct {
	matchLabels map[string]string
	labels      map[string]string
	result      bool
}

var matchLabelTestDataArray = []matchLabelTestData{
	{
		matchLabels: nil, // no match if matchLabels is nil
		labels:      nil,
		result:      false,
	},
	{
		matchLabels: map[string]string{}, // no match of matchLabels is empty
		labels:      nil,
		result:      false,
	},
	{
		matchLabels: map[string]string{}, // no match if matchLabels is empty
		labels:      map[string]string{},
		result:      false,
	},
	{
		matchLabels: map[string]string{},
		labels: map[string]string{
			"stage": "dev",
		},
		result: false,
	},
	// simple match
	{
		matchLabels: map[string]string{
			"stage": "dev",
		},
		labels: map[string]string{
			"stage": "dev",
		},
		result: true,
	},
	{
		matchLabels: map[string]string{
			"stage": "dev",
		},
		labels: map[string]string{
			"stage":  "dev",
			"region": "east",
		},
		result: true,
	},
	{
		matchLabels: map[string]string{
			"stage":  "dev",
			"region": "east",
		},
		labels: map[string]string{
			"stage":  "dev",
			"region": "east",
		},
		result: true,
	},
	// same key, different value
	{
		matchLabels: map[string]string{
			"stage":  "dev",
			"region": "east",
		},
		labels: map[string]string{
			"stage":  "test",
			"region": "east",
		},
		result: false,
	},
	// matchLabels superset of labels
	{
		matchLabels: map[string]string{
			"stage":  "dev",
			"region": "east",
			"group":  "software",
		},
		labels: map[string]string{
			"stage":  "dev",
			"region": "east",
		},
		result: false,
	},
}

func TestLabelsMatch(t *testing.T) {
	for index, testData := range matchLabelTestDataArray {
		result := labelsMatch(testData.matchLabels, testData.labels)
		if result != testData.result {
			t.Errorf("unexpected result iteration %d for %s %s, expected: %t\n", index, testData.matchLabels, testData.labels, testData.result)
		}
	}
}

type expressionTestData struct {
	expressions []matchExpression
	labels      map[string]string
	result      bool
}

const (
	operatorUnsupported = "WrongOperator"
)

var expressionTestDataArray = []expressionTestData{
	// empty expression
	{
		expressions: []matchExpression{},
		labels:      map[string]string{},
		result:      false,
	},
	// unspported
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: operatorUnsupported,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: false,
	},
	// test operator IN
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{
			"env": "west",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env": "west",
		},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env": "north",
		},
		result: false,
	},
	// NotIn
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{
			"otherenv": "east",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east"},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env": "west",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
		},
		labels: map[string]string{
			"env":   "north",
			"group": "hardware",
		},
		result: true,
	},
	// test Exists
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorExists,
				values:   []string{},
			},
		},
		labels: map[string]string{},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorExists,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"group": "hardware",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorExists,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorExists,
				values:   []string{"random", "string", "is", "ok"},
			},
		},
		labels: map[string]string{
			"env": "west",
		},
		result: true,
	},
	// test DoesNotExist
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"otherenv": "unkown",
		},
		result: true,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env": "east",
		},
		result: false,
	},
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env": "west",
		},
		result: false,
	},
	// test combinations
	// true, true, true, true
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"dev", "test"},
			},
			{
				key:      "region",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
			{
				key:      "group",
				operator: OperatorExists,
				values:   []string{},
			},
			{
				key:      "special",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env":    "dev",
			"region": "north",
			"group":  "hardware",
		},
		result: true,
	},
	// false, true, true, true
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"dev", "test"},
			},
			{
				key:      "region",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
			{
				key:      "group",
				operator: OperatorExists,
				values:   []string{},
			},
			{
				key:      "special",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env":    "prod",
			"region": "north",
			"group":  "hardware",
		},
		result: false,
	},
	// true, false, true, true
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"dev", "test"},
			},
			{
				key:      "region",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
			{
				key:      "group",
				operator: OperatorExists,
				values:   []string{},
			},
			{
				key:      "special",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env":    "dev",
			"region": "east",
			"group":  "hardware",
		},
		result: false,
	},
	// true, true, false, true
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"dev", "test"},
			},
			{
				key:      "region",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
			{
				key:      "group",
				operator: OperatorExists,
				values:   []string{},
			},
			{
				key:      "special",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env":    "dev",
			"region": "north",
		},
		result: false,
	},
	// true, true, true, false
	{
		expressions: []matchExpression{
			{
				key:      "env",
				operator: OperatorIn,
				values:   []string{"dev", "test"},
			},
			{
				key:      "region",
				operator: OperatorNotIn,
				values:   []string{"east", "west"},
			},
			{
				key:      "group",
				operator: OperatorExists,
				values:   []string{},
			},
			{
				key:      "special",
				operator: OperatorDoesNotExist,
				values:   []string{},
			},
		},
		labels: map[string]string{
			"env":     "dev",
			"region":  "north",
			"group":   "hardware",
			"special": "special",
		},
		result: false,
	},
}

func TestExpressionsMatch(t *testing.T) {
	for _, expressionData := range expressionTestDataArray {
		result := expressionsMatch(expressionData.expressions, expressionData.labels)
		if result != expressionData.result {
			t.Errorf("unexpected result %s %s, expected: %t\n", expressionData.expressions, expressionData.labels, expressionData.result)
		}
	}
}
