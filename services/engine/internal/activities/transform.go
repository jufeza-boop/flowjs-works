package activities

import (
"bytes"
"encoding/csv"
"encoding/json"
"encoding/xml"
"fmt"
"sort"
"strings"

"flowjs-works/engine/internal/models"
)

// TransformActivity implements the `transform` node type.
// config fields:
//   transform_type: "json2csv" | "xml2json" | "json2xml"
//   data:           the input data (map, slice, or string)
//   spec:           optional spec/hints (string, may be empty)
type TransformActivity struct{}

func (a *TransformActivity) Name() string { return "transform" }

func (a *TransformActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
transformType, ok := config["transform_type"].(string)
if !ok || transformType == "" {
return nil, fmt.Errorf("transform activity: missing required config field 'transform_type'")
}
data := config["data"]

switch transformType {
case "json2csv":
return transformJSON2CSV(data)
case "xml2json":
return transformXML2JSON(data)
case "json2xml":
return transformJSON2XML(data)
default:
return nil, fmt.Errorf("transform activity: unknown transform_type %q", transformType)
}
}

func transformJSON2CSV(data interface{}) (map[string]interface{}, error) {
rows, ok := data.([]interface{})
if !ok {
return nil, fmt.Errorf("transform json2csv: data must be an array of objects")
}
if len(rows) == 0 {
return map[string]interface{}{"result": ""}, nil
}
firstRow, ok := rows[0].(map[string]interface{})
if !ok {
return nil, fmt.Errorf("transform json2csv: each row must be an object")
}
// Stable header order
headers := make([]string, 0, len(firstRow))
for k := range firstRow {
headers = append(headers, k)
}
sort.Strings(headers)

var buf bytes.Buffer
w := csv.NewWriter(&buf)
if err := w.Write(headers); err != nil {
return nil, fmt.Errorf("transform json2csv: %w", err)
}
for _, rowRaw := range rows {
row, ok := rowRaw.(map[string]interface{})
if !ok {
return nil, fmt.Errorf("transform json2csv: each row must be an object")
}
record := make([]string, len(headers))
for i, h := range headers {
v := row[h]
if v == nil {
record[i] = ""
} else {
record[i] = fmt.Sprintf("%v", v)
}
}
if err := w.Write(record); err != nil {
return nil, fmt.Errorf("transform json2csv: %w", err)
}
}
w.Flush()
if err := w.Error(); err != nil {
return nil, fmt.Errorf("transform json2csv: %w", err)
}
return map[string]interface{}{"result": buf.String()}, nil
}

func transformXML2JSON(data interface{}) (map[string]interface{}, error) {
xmlStr, ok := data.(string)
if !ok {
return nil, fmt.Errorf("transform xml2json: data must be an XML string")
}
result, err := xmlToMap(xmlStr)
if err != nil {
return nil, fmt.Errorf("transform xml2json: %w", err)
}
jsonBytes, err := json.Marshal(result)
if err != nil {
return nil, fmt.Errorf("transform xml2json: %w", err)
}
return map[string]interface{}{"result": string(jsonBytes)}, nil
}

// xmlToMap parses an XML string into a map[string]interface{} using token walking.
func xmlToMap(xmlStr string) (map[string]interface{}, error) {
decoder := xml.NewDecoder(strings.NewReader(xmlStr))
var root map[string]interface{}
var stack []map[string]interface{}
var keyStack []string

for {
token, err := decoder.Token()
if err != nil {
break
}
switch t := token.(type) {
case xml.StartElement:
node := make(map[string]interface{})
for _, attr := range t.Attr {
node["@"+attr.Name.Local] = attr.Value
}
stack = append(stack, node)
keyStack = append(keyStack, t.Name.Local)
case xml.EndElement:
if len(stack) == 0 {
break
}
node := stack[len(stack)-1]
key := keyStack[len(keyStack)-1]
stack = stack[:len(stack)-1]
keyStack = keyStack[:len(keyStack)-1]
if len(stack) == 0 {
root = map[string]interface{}{key: node}
} else {
parent := stack[len(stack)-1]
if existing, ok := parent[key]; ok {
switch v := existing.(type) {
case []interface{}:
parent[key] = append(v, node)
default:
parent[key] = []interface{}{v, node}
}
} else {
parent[key] = node
}
}
case xml.CharData:
text := strings.TrimSpace(string(t))
if text != "" && len(stack) > 0 {
node := stack[len(stack)-1]
node["#text"] = text
}
}
}
if root == nil {
return map[string]interface{}{}, nil
}
return root, nil
}

func transformJSON2XML(data interface{}) (map[string]interface{}, error) {
jsonStr, ok := data.(string)
if !ok {
return nil, fmt.Errorf("transform json2xml: data must be a JSON string")
}
var parsed interface{}
if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
return nil, fmt.Errorf("transform json2xml: invalid JSON: %w", err)
}
var buf bytes.Buffer
buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
if err := marshalXML(&buf, "root", parsed); err != nil {
return nil, fmt.Errorf("transform json2xml: %w", err)
}
return map[string]interface{}{"result": buf.String()}, nil
}

func marshalXML(buf *bytes.Buffer, tag string, v interface{}) error {
buf.WriteString("<" + tag + ">")
switch val := v.(type) {
case map[string]interface{}:
keys := make([]string, 0, len(val))
for k := range val {
keys = append(keys, k)
}
sort.Strings(keys)
for _, k := range keys {
if err := marshalXML(buf, k, val[k]); err != nil {
return err
}
}
case []interface{}:
for _, item := range val {
if err := marshalXML(buf, "item", item); err != nil {
return err
}
}
case nil:
// empty
default:
var esc bytes.Buffer
if err := xml.EscapeText(&esc, []byte(fmt.Sprintf("%v", val))); err != nil {
return err
}
buf.Write(esc.Bytes())
}
buf.WriteString("</" + tag + ">")
return nil
}
