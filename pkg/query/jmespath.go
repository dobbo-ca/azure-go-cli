package query

import (
	"encoding/json"
	"fmt"

	"github.com/jmespath/go-jmespath"
)

// ApplyJMESPath applies a JMESPath query to JSON data
func ApplyJMESPath(data interface{}, queryStr string) (interface{}, error) {
	if queryStr == "" {
		return data, nil
	}

	result, err := jmespath.Search(queryStr, data)
	if err != nil {
		return nil, fmt.Errorf("invalid JMESPath query: %w", err)
	}

	return result, nil
}

// ApplyJMESPathToJSON applies a JMESPath query to JSON bytes
func ApplyJMESPathToJSON(jsonData []byte, queryStr string) ([]byte, error) {
	if queryStr == "" {
		return jsonData, nil
	}

	// Parse JSON into interface{}
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Apply query
	result, err := ApplyJMESPath(data, queryStr)
	if err != nil {
		return nil, err
	}

	// Marshal result back to JSON
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return output, nil
}
