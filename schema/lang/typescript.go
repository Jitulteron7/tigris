// Copyright 2022-2023 Tigris Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:dupl
package schema

import (
	"fmt"

	"github.com/tigrisdata/tigris/templates"
)

type JSONToTypeScript struct{}

func getTSStringType(format string) string {
	switch format {
	case formatDateTime:
		return "DATE_TIME"
	case formatByte:
		return "BYTE_STRING"
	case formatUUID:
		return "UUID"
	default:
		return "STRING"
	}
}

func (*JSONToTypeScript) GetType(tp string, format string) (string, error) {
	var resType string

	switch tp {
	case typeString:
		return getTSStringType(format), nil
	case typeInteger:
		switch format {
		case formatInt32:
			resType = "INT32"
		default:
			resType = "INT64"
		}
	case typeNumber:
		resType = "NUMBER"
	case typeBoolean:
		resType = "BOOLEAN"
	}

	if resType == "" {
		return "", fmt.Errorf("%w type=%s, format=%s", ErrUnsupportedType, tp, format)
	}

	return resType, nil
}

func (*JSONToTypeScript) GetObjectTemplate() string {
	return templates.SchemaTypeScriptObject
}
