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

//go:build integration

package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	api "github.com/tigrisdata/tigris/api/server/v1"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func insertDocs(t *testing.T, db string, coll string, extra ...Doc) {
	inputDocument := []Doc{
		{
			"pkey_int":        1,
			"int_value":       10,
			"string_value":    "a",
			"bool_value":      true,
			"double_value":    10.01,
			"bytes_value":     []byte{1, 2, 3, 4},
			"uuid_value":      uuid.New().String(),
			"date_time_value": "2015-12-21T17:42:34Z",
		},
		{
			"pkey_int":        2,
			"int_value":       1,
			"string_value":    "G",
			"bool_value":      false,
			"double_value":    5.05,
			"bytes_value":     []byte{4, 4, 4},
			"uuid_value":      uuid.New().String(),
			"date_time_value": "2016-10-12T17:42:34Z",
		},
		{
			"pkey_int":        3,
			"int_value":       100,
			"string_value":    "B",
			"bool_value":      false,
			"double_value":    1000,
			"bytes_value":     []byte{3, 4, 4},
			"uuid_value":      uuid.New().String(),
			"date_time_value": "2013-11-01T17:42:34Z",
		},
		{
			"pkey_int":        4,
			"int_value":       5,
			"string_value":    "z",
			"bool_value":      true,
			"double_value":    25.05,
			"bytes_value":     []byte{4, 4, 4},
			"uuid_value":      uuid.New().String(),
			"date_time_value": "2020-10-12T17:42:34Z",
		},
		{
			"pkey_int":        30,
			"int_value":       30,
			"string_value":    "k",
			"bool_value":      false,
			"double_value":    5.05,
			"bytes_value":     []byte{4, 4, 4},
			"uuid_value":      uuid.New().String(),
			"date_time_value": "2014-10-12T17:42:34Z",
		},
	}

	inputDocument = append(inputDocument, extra...)

	e := expect(t)
	e.POST(getDocumentURL(db, coll, "insert")).
		WithJSON(Map{
			"documents": inputDocument,
		}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().
		ValueEqual("status", "inserted")
}

func getIds(docs []map[string]jsoniter.RawMessage) []int {
	var ids []int
	for _, docString := range docs {

		var result map[string]json.RawMessage
		var doc map[string]json.RawMessage

		json.Unmarshal(docString["result"], &result)
		json.Unmarshal(result["data"], &doc)

		var id int
		json.Unmarshal(doc["pkey_int"], &id)
		ids = append(ids, id)
	}

	return ids
}

func TestQuery_EQ(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll)
	defer cleanupTests(t, db)

	cases := []struct {
		filter   Map
		ids      []int
		keyRange []string
	}{
		{
			Map{"int_value": 10},
			[]int{1},
			[]string{"10"},
		},
		{
			Map{"bool_value": false},
			[]int{2, 3, 30},
			[]string{"false"},
		},
		{
			Map{"bool_value": false, "int_value": 3},
			[]int(nil),
			[]string{"false"},
		},
		{
			Map{"bool_value": false, "int_value": 30},
			[]int{30},
			[]string{"false"},
		},
		{
			Map{
				"$and": []any{
					Map{"string_value": Map{"$eq": "G"}},
					Map{"bool_value": false},
				},
			},
			[]int{2},
			[]string{"false"},
		},
		{
			Map{"int_value": 1, "double_value": Map{"$gte": 5}},
			[]int{2},
			[]string{"1"},
		},
		// {
		// 	Map{
		// 		"$or": []any{
		// 			Map{"int_value": Map{"$eq": 10}},
		// 			Map{"int_value": 100},
		// 			Map{"double_value": 25.05},
		// 		},
		// 	},
		// 	[]int{4, 1, 3},
		// },
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids))
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, query.keyRange, explain.KeyRange)
		assert.Equal(t, "secondary index", explain.ReadType)
	}
}

func TestQuery_Range(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll)
	defer cleanupTests(t, db)
	max := "$TIGRIS_MAX"

	cases := []struct {
		filter   Map
		ids      []int
		keyRange []string
	}{
		{
			Map{"int_value": Map{"$gt": 0}},
			[]int{2, 4, 1, 30, 3},
			[]string{"0", max},
		},
		{
			Map{"int_value": Map{"$lt": 30}},
			[]int{2, 4, 1},
			[]string{"null", "30"},
		},
		{
			Map{
				"$and": []any{
					Map{"int_value": Map{"$gte": 30}},
					Map{"int_value": Map{"$lte": 100}},
				},
			},
			[]int{30, 3},
			[]string{"30", "100"},
		},
		{
			Map{"string_value": Map{"$gt": "B"}},
			[]int{2, 30, 4},
			nil,
		},
		{
			Map{"string_value": Map{"$lt": "G"}},
			[]int{1, 3},
			nil,
		},
		{
			Map{
				"$and": []any{
					Map{"string_value": Map{"$gte": "G"}},
					Map{"string_value": Map{"$lt": "z"}},
				},
			},
			[]int{2, 30},
			nil,
		},
		{
			Map{
				"$and": []any{
					Map{"bool_value": Map{"$gte": true}},
					Map{"bool_value": Map{"$lte": true}},
				},
			},
			[]int{1, 4},
			[]string{"true", "true"},
		},
		{
			Map{
				"bool_value": Map{"$gte": true},
			},
			[]int{1, 4},
			[]string{"true", max},
		},
		{
			Map{
				"bool_value": Map{"$lte": true},
			},
			[]int{2, 3, 30, 1, 4},
			[]string{"null", "true"},
		},
		{
			Map{
				"bool_value": Map{"$lt": true},
			},
			[]int{2, 3, 30},
			[]string{"null", "true"},
		},
		{
			Map{
				"bool_value": Map{"$gte": false},
			},
			[]int{2, 3, 30, 1, 4},
			[]string{"false", max},
		},
		{
			Map{
				"bool_value": Map{"$lte": false},
			},
			[]int{2, 3, 30},
			[]string{"null", "false"},
		},
		{
			Map{
				"bool_value": Map{"$lt": true},
			},
			[]int{2, 3, 30},
			[]string{"null", "true"},
		},
		{
			Map{"double_value": Map{"$gt": 10}},
			[]int{1, 4, 3},
			[]string{"10", max},
		},
		{
			Map{"double_value": Map{"$lt": 26}},
			[]int{2, 30, 1, 4},
			[]string{"null", "26"},
		},
		{
			Map{
				"$and": []any{
					Map{"double_value": Map{"$gte": 10.01}},
					Map{"double_value": Map{"$lt": 1000}},
				},
			},
			[]int{1, 4},
			[]string{"10.01", "1000"},
		},
		{
			Map{"date_time_value": Map{"$gt": "2015-12.22T17:42:34Z"}},
			[]int{2, 4},
			[]string{"2015-12.22T17:42:34Z", max},
		},
		{
			Map{"date_time_value": Map{"$lt": "2015-12.22T17:42:34Z"}},
			[]int{3, 30, 1},
			[]string{"null", "2015-12.22T17:42:34Z"},
		},
		{
			Map{
				"$and": []interface{}{
					Map{"_tigris_created_at": Map{"$gt": "2022-12.22T17:42:34Z"}},
					Map{"bool_value": true},
				},
			},
			[]int{1, 4},
			[]string{"true"},
		},
		{
			Map{
				"$and": []any{
					Map{"date_time_value": Map{"$gte": "2013-11-01T17:42:34Z"}},
					Map{"date_time_value": Map{"$lt": "2015-12.22T17:42:34Z"}},
				},
			},
			[]int{3, 30, 1},
			[]string{"2013-11-01T17:42:34Z", "2015-12.22T17:42:34Z"},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		if query.keyRange != nil {
			assert.Equal(t, query.keyRange, explain.KeyRange)
		}

		fieldName := firstFilterName(query.filter)
		if fieldName != "" {
			assert.Equal(t, fieldName, explain.Field, query.filter)
		}
		assert.Equal(t, "secondary index", explain.ReadType)
	}
}

func TestQuery_Sort(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll)
	defer cleanupTests(t, db)
	max := "$TIGRIS_MAX"

	cases := []struct {
		filter   Map
		ids      []int
		keyRange []string
		sort     []Map
	}{
		{
			Map{"int_value": Map{"$gt": 1}},
			[]int{3, 30, 1, 4},
			[]string{"1", max},
			[]Map{{"int_value": "$desc"}},
		},
		{
			Map{"int_value": Map{"$gt": 0}},
			[]int{3, 4, 1, 30, 2},
			[]string{"null", max},
			[]Map{{"double_value": "$desc"}},
		},
		{
			Map{"double_value": Map{"$eq": 5.05}},
			[]int{30, 2},
			[]string{"5.05"},
			[]Map{{"double_value": "$desc"}},
		},
		{
			Map{"double_value": Map{"$eq": 5.05}},
			[]int{2, 30},
			[]string{"5.05"},
			[]Map{{"double_value": "$asc"}},
		},
		{
			Map{"int_value": Map{"$gt": 0}},
			[]int{2, 4, 1, 30, 3},
			[]string{"0", max},
			[]Map{{"int_value": "$asc"}},
		},
		{
			Map{"$and": []interface{}{
				Map{"int_value": Map{"$gte": 5}},
				Map{"int_value": Map{"$lt": 100}},
			},
			},
			[]int{30, 1, 4},
			[]string{"5", "100"},
			[]Map{{"int_value": "$desc"}},
		},
		{
			Map{"string_value": Map{"$gt": "B"}},
			[]int{2, 30, 4},
			nil,
			[]Map{{"string_value": "$asc"}},
		},
		{
			Map{"string_value": Map{"$gt": "B"}},
			[]int{4, 30, 2},
			nil,
			[]Map{{"string_value": "$desc"}},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, query.sort)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, query.sort)
		if query.keyRange != nil {
			assert.Equal(t, query.keyRange, explain.KeyRange)
		}

		fieldName := firstFilterName(query.sort[0])
		if fieldName != "" {
			assert.Equal(t, fieldName, explain.Field, query.filter)
		}
		assert.Equal(t, "secondary index", explain.ReadType)
	}
}

func firstFilterName(filter Map) string {
	fieldName := ""
	for k := range filter {
		if k == "$and" {
			return ""
		} else {
			fieldName = k
		}
		break
	}

	return fieldName
}

func TestQuery_RangeWithNull(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll, Doc{
		"pkey_int":        50,
		"int_value":       nil,
		"string_value":    nil,
		"bool_value":      nil,
		"double_value":    nil,
		"bytes_value":     nil,
		"uuid_value":      nil,
		"date_time_value": nil,
	})
	defer cleanupTests(t, db)

	cases := []struct {
		filter Map
		ids    []int
	}{
		{
			Map{"int_value": Map{"$eq": nil}},
			[]int{50},
		},
		{
			Map{"int_value": Map{"$gt": nil}},
			[]int{2, 4, 1, 30, 3},
		},
		{
			Map{"int_value": Map{"$gte": nil}},
			[]int{50, 2, 4, 1, 30, 3},
		},
		{
			Map{"int_value": Map{"$lt": 30}},
			[]int{50, 2, 4, 1},
		},
		{
			Map{"string_value": Map{"$gt": "B"}},
			[]int{2, 30, 4},
		},
		{
			Map{"string_value": Map{"$lt": "G"}},
			[]int{50, 1, 3},
		},
		{
			Map{
				"$and": []any{
					Map{"string_value": Map{"$gte": "G"}},
					Map{"string_value": Map{"$lt": "z"}},
				},
			},
			[]int{2, 30},
		},
		{
			Map{
				"$and": []any{
					Map{"bool_value": Map{"$gte": true}},
					Map{"bool_value": Map{"$lte": true}},
				},
			},
			[]int{1, 4},
		},
		{
			Map{"double_value": Map{"$gt": nil}},
			[]int{2, 30, 1, 4, 3},
		},
		{
			Map{"double_value": Map{"$gte": nil}},
			[]int{50, 2, 30, 1, 4, 3},
		},
		{
			Map{"double_value": Map{"$gt": 10}},
			[]int{1, 4, 3},
		},
		{
			Map{"double_value": Map{"$lt": 26}},
			[]int{50, 2, 30, 1, 4},
		},
		{
			Map{"date_time_value": Map{"$lt": "2015-12.22T17:42:34Z"}},
			[]int{50, 3, 30, 1},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, explain.ReadType, "secondary index")
	}
}

func TestQuery_LongStrings(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll,
		Doc{
			"pkey_int":        50,
			"int_value":       nil,
			"string_value":    "Hi, this is a very long string that will be cut off at 64 bytes of length",
			"bool_value":      nil,
			"double_value":    nil,
			"bytes_value":     nil,
			"uuid_value":      nil,
			"date_time_value": nil,
		},
		Doc{
			"pkey_int":        60,
			"int_value":       nil,
			"string_value":    "Hi, this is a very long string that will be cut off at 64 bytes of length but it is different to the other",
			"bool_value":      nil,
			"double_value":    nil,
			"bytes_value":     nil,
			"uuid_value":      nil,
			"date_time_value": nil,
		},
		Doc{
			"pkey_int":        70,
			"int_value":       nil,
			"string_value":    "Hi, this is a very long string that will be cut off at 64 bytes of length and then has something different",
			"bool_value":      nil,
			"double_value":    nil,
			"bytes_value":     nil,
			"uuid_value":      nil,
			"date_time_value": nil,
		},
	)
	defer cleanupTests(t, db)

	cases := []struct {
		filter Map
		ids    []int
	}{
		{
			Map{"string_value": Map{"$eq": "Hi, this is a very long string that will be cut off at 64 bytes of length but it is different to the other"}},
			[]int{60},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, explain.ReadType, "secondary index")
	}
}

func TestQuery_MaxValues(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll,
		Doc{
			"pkey_int":        50,
			"int_value":       math.MaxInt64,
			"string_value":    nil,
			"bool_value":      nil,
			"double_value":    math.MaxFloat64,
			"bytes_value":     nil,
			"uuid_value":      nil,
			"date_time_value": nil,
		},
		Doc{
			"pkey_int":        60,
			"int_value":       math.MinInt64,
			"string_value":    "small string",
			"bool_value":      nil,
			"double_value":    math.SmallestNonzeroFloat64,
			"bytes_value":     nil,
			"uuid_value":      nil,
			"date_time_value": nil,
		},
	)
	defer cleanupTests(t, db)

	cases := []struct {
		filter Map
		ids    []int
	}{
		{
			Map{"int_value": Map{"$eq": math.MaxInt64}},
			[]int{50},
		},
		{
			Map{"int_value": Map{"$eq": math.MinInt64}},
			[]int{60},
		},
		{
			Map{"int_value": Map{"$lt": 0}},
			[]int{60},
		},
		{
			Map{"int_value": Map{"$gt": 100000}},
			[]int{50},
		},
		{
			Map{"double_value": Map{"$eq": math.MaxFloat64}},
			[]int{50},
		},
		{
			Map{"double_value": Map{"$eq": math.SmallestNonzeroFloat64}},
			[]int{60},
		},
		{
			Map{"double_value": Map{"$gt": 100000.0}},
			[]int{50},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, explain.ReadType, "secondary index")
	}
}

func TestQuery_AfterUpdates(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll)
	defer cleanupTests(t, db)

	updateByFilter(t,
		db,
		coll,
		Map{
			"filter": Map{
				"int_value": 100,
			},
		},
		Map{
			"fields": Map{
				"$set": Map{
					"int_value":    105,
					"string_value": "updated",
				},
			},
		},
		nil).Status(http.StatusOK).
		JSON().
		Object().
		ValueEqual("modified_count", 1).
		Path("$.metadata").Object()

	cases := []struct {
		filter Map
		ids    []int
	}{
		{
			Map{"int_value": Map{"$eq": 105}},
			[]int{3},
		},
		{
			Map{"int_value": Map{"$eq": 100}},
			[]int(nil),
		},
		{
			Map{"string_value": Map{"$eq": "updated"}},
			[]int{3},
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, explain.ReadType, "secondary index")
	}
}

func TestQuery_AfterDelete(t *testing.T) {
	db, coll := setupTests(t)
	insertDocs(t, db, coll)
	defer cleanupTests(t, db)

	deleteByFilter(t,
		db,
		coll,
		Map{
			"filter": Map{
				"int_value": 100,
			},
		}).Status(http.StatusOK).
		JSON().
		Object().
		ValueEqual("status", "deleted")

	cases := []struct {
		filter Map
		ids    []int
	}{
		{
			Map{"int_value": Map{"$gte": 30}},
			[]int{30},
		},
		{
			Map{"int_value": Map{"$eq": 100}},
			[]int(nil),
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, len(query.ids), len(ids), query.filter)
		assert.Equal(t, query.ids, ids, query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, explain.ReadType, "secondary index")
	}
}

var testBuildIndexSchema = Map{
	"schema": Map{
		"title":       testCollection,
		"description": "this schema is for integration tests",
		"properties": Map{
			"pkey_int": Map{
				"description": "primary key field",
				"type":        "integer",
			},
			"int_value": Map{
				"description": "simple int field",
				"type":        "integer",
				"index":       true,
			},
			"string_value": Map{
				"description": "simple string field",
				"type":        "string",
				"maxLength":   128,
			},
			"bool_value": Map{
				"description": "simple boolean field",
				"type":        "boolean",
			},
			"double_value": Map{
				"description": "simple double field",
				"type":        "number",
			},
			"uuid_value": Map{
				"description": "uuid field",
				"type":        "string",
				"format":      "uuid",
			},
			"date_time_value": Map{
				"description": "date time field",
				"type":        "string",
				"format":      "date-time",
				"index":       true,
			},
		},
		"primary_key": []interface{}{"pkey_int"},
	},
}

func setupIndexBuildTest(t *testing.T, schema Map) (string, string) {
	db := fmt.Sprintf("integration_%s", t.Name())
	deleteProject(t, db)
	createProject(t, db).Status(http.StatusOK)
	createCollection(t, db, testCollection, schema).Status(http.StatusOK)
	return db, testCollection
}

func TestQuery_BuildIndex(t *testing.T) {
	db, coll := setupIndexBuildTest(t, testBuildIndexSchema)
	defer cleanupTests(t, db)

	for i := 0; i < 20; i++ {
		writeDocs(t, db, coll, i*50, 50)
	}

	for _, val := range testBuildIndexSchema["schema"].(Map)["properties"].(Map) {
		prop := val.(Map)
		prop["index"] = true
	}

	createCollection(t, db, coll, testBuildIndexSchema).Status(http.StatusOK)
	str := buildCollectionIndexes(t, db, coll).Body().Raw()
	resp := &api.BuildCollectionIndexResponse{}
	err := jsoniter.Unmarshal([]byte(str), resp)
	assert.NoError(t, err)

	checkIndexesActive(t, resp.Indexes)

	str = describeCollection(t, db, coll, Map{}).Status(http.StatusOK).Body().Raw()
	desc := Map{}
	err = jsoniter.Unmarshal([]byte(str), &desc)
	assert.NoError(t, err)
	i, _ := desc["indexes"]
	raw, err := jsoniter.Marshal(i)
	assert.NoError(t, err)
	var descIndexes []*api.CollectionIndex
	err = jsoniter.Unmarshal(raw, &descIndexes)
	assert.NoError(t, err)
	checkIndexesActive(t, descIndexes)

	cases := []struct {
		filter Map
		count  int
	}{
		{
			Map{"int_value": Map{"$gt": nil}},
			1000,
		},
		{
			Map{"string_value": Map{"$gt": nil}},
			1000,
		},
		{
			Map{"bool_value": Map{"$gt": nil}},
			1000,
		},
		{
			Map{"double_value": Map{"$gt": nil}},
			1000,
		},
		{
			Map{"date_time_value": Map{"$gt": nil}},
			1000,
		},
	}

	for _, query := range cases {
		resp := readByFilter(t, db, coll, query.filter, nil, nil, nil)
		ids := getIds(resp)
		assert.Equal(t, query.count, len(ids), query.filter)
		explain := explainQuery(t, db, coll, query.filter, nil, nil, nil)
		assert.Equal(t, "secondary index", explain.ReadType)
	}
}

func writeDocs(t *testing.T, db string, coll string, startId int, count int) {

	for i := 0; i < count; i++ {
		inputDocument := []Doc{

			{
				"pkey_int":        startId + i,
				"int_value":       i,
				"string_value":    fmt.Sprintf("a-%v", i),
				"bool_value":      true,
				"double_value":    10.01 + float64(i),
				"uuid_value":      uuid.New().String(),
				"date_time_value": "2015-12-21T17:42:34Z",
			},
		}
		e := expect(t)
		e.POST(getDocumentURL(db, coll, "insert")).
			WithJSON(Map{

				"documents": inputDocument,
			}).
			Expect().
			Status(http.StatusOK).
			JSON().
			Object().
			ValueEqual("status", "inserted")
	}
}

func checkIndexesActive(t *testing.T, indexes []*api.CollectionIndex) {
	assert.Len(t, indexes, 9)
	for _, idx := range indexes {
		assert.Equal(t, "INDEX ACTIVE", idx.State)
	}
}

func stringEncoder(input string) string {
	inputBytes := []byte(input)

	if len(inputBytes) > 64 {
		inputBytes = inputBytes[:64]
	}

	collator := collate.New(language.English)
	var buf collate.Buffer

	return string(collator.Key(&buf, inputBytes))
}
