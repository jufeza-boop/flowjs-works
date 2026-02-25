package activities

import (
"encoding/json"
"strings"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestTransformActivity_JSON2CSV(t *testing.T) {
tests := []struct {
name    string
data    interface{}
wantErr bool
check   func(t *testing.T, result string)
}{
{
name: "basic rows",
data: []interface{}{
map[string]interface{}{"name": "Alice", "age": float64(30)},
map[string]interface{}{"name": "Bob", "age": float64(25)},
},
check: func(t *testing.T, result string) {
assert.Contains(t, result, "age")
assert.Contains(t, result, "name")
assert.Contains(t, result, "Alice")
assert.Contains(t, result, "Bob")
},
},
{
name:  "empty array",
data:  []interface{}{},
check: func(t *testing.T, result string) { assert.Equal(t, "", result) },
},
{
name:    "not an array",
data:    "not an array",
wantErr: true,
},
}
a := &TransformActivity{}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
out, err := a.Execute(nil, map[string]interface{}{
"transform_type": "json2csv",
"data":           tc.data,
}, nil)
if tc.wantErr {
assert.Error(t, err)
return
}
require.NoError(t, err)
tc.check(t, out["result"].(string))
})
}
}

func TestTransformActivity_XML2JSON(t *testing.T) {
a := &TransformActivity{}
xmlInput := `<root><name>Alice</name><age>30</age></root>`
out, err := a.Execute(nil, map[string]interface{}{
"transform_type": "xml2json",
"data":           xmlInput,
}, nil)
require.NoError(t, err)
result := out["result"].(string)
var parsed map[string]interface{}
require.NoError(t, json.Unmarshal([]byte(result), &parsed))
assert.Contains(t, result, "root")
}

func TestTransformActivity_XML2JSON_InvalidXML(t *testing.T) {
a := &TransformActivity{}
// Not a string
_, err := a.Execute(nil, map[string]interface{}{
"transform_type": "xml2json",
"data":           123,
}, nil)
assert.Error(t, err)
}

func TestTransformActivity_JSON2XML(t *testing.T) {
a := &TransformActivity{}
jsonInput := `{"name":"Alice","age":30}`
out, err := a.Execute(nil, map[string]interface{}{
"transform_type": "json2xml",
"data":           jsonInput,
}, nil)
require.NoError(t, err)
result := out["result"].(string)
assert.True(t, strings.HasPrefix(result, "<?xml"))
assert.Contains(t, result, "Alice")
}

func TestTransformActivity_JSON2XML_InvalidJSON(t *testing.T) {
a := &TransformActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"transform_type": "json2xml",
"data":           "not json",
}, nil)
assert.Error(t, err)
}

func TestTransformActivity_UnknownType(t *testing.T) {
a := &TransformActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"transform_type": "yaml2toml",
"data":           "x",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "unknown transform_type")
}

func TestTransformActivity_MissingType(t *testing.T) {
a := &TransformActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"data": "x",
}, nil)
assert.Error(t, err)
}
