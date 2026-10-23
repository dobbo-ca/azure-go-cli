package routetable

import (
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
)

// pyShape converts SDK models to the shape Python az prints, so the same
// --query expressions work: "properties" is lifted into its parent and
// resourceGroup is added, on the table and on each route and subnet.
func pyShape(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if list, ok := out.([]interface{}); ok {
		for _, item := range list {
			flattenTable(item)
		}
	} else {
		flattenTable(out)
	}
	return out, nil
}

func flattenTable(v interface{}) {
	m := flatten(v)
	if m == nil {
		return
	}
	for _, key := range []string{"routes", "subnets"} {
		if items, ok := m[key].([]interface{}); ok {
			for _, item := range items {
				flatten(item)
			}
		}
	}
}

func flatten(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		delete(m, "properties")
		for k, val := range props {
			m[k] = val
		}
	}
	if id, ok := m["id"].(string); ok {
		if parsed, err := arm.ParseResourceID(id); err == nil {
			m["resourceGroup"] = parsed.ResourceGroupName
		}
	}
	return m
}
