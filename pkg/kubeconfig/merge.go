package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func UpdateContext(kubeConfig []byte, contextName string) ([]byte, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(kubeConfig, &config); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Update context name
	config["current-context"] = contextName

	// Update context in contexts list
	if contexts, ok := config["contexts"].([]interface{}); ok {
		for _, ctx := range contexts {
			if ctxMap, ok := ctx.(map[string]interface{}); ok {
				ctxMap["name"] = contextName
			}
		}
	}

	// Marshal back
	updated, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	return updated, nil
}

func Merge(file string, newConfig []byte) error {
	// Parse new kubeconfig
	var newKube map[string]interface{}
	if err := yaml.Unmarshal(newConfig, &newKube); err != nil {
		return fmt.Errorf("failed to parse new kubeconfig: %w", err)
	}

	// Check if file exists
	var existingKube map[string]interface{}
	if data, err := os.ReadFile(file); err == nil {
		if err := yaml.Unmarshal(data, &existingKube); err != nil {
			return fmt.Errorf("failed to parse existing kubeconfig: %w", err)
		}
	} else {
		// File doesn't exist, create basic structure
		existingKube = map[string]interface{}{
			"apiVersion":      "v1",
			"kind":            "Config",
			"clusters":        []interface{}{},
			"contexts":        []interface{}{},
			"users":           []interface{}{},
			"current-context": "",
		}
	}

	// Merge clusters, contexts, and users
	existingKube["clusters"] = mergeItems(existingKube["clusters"], newKube["clusters"])
	existingKube["contexts"] = mergeItems(existingKube["contexts"], newKube["contexts"])
	existingKube["users"] = mergeItems(existingKube["users"], newKube["users"])
	existingKube["current-context"] = newKube["current-context"]

	// Write merged kubeconfig
	merged, err := yaml.Marshal(existingKube)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(file, merged, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

func mergeItems(existing, new interface{}) []interface{} {
	existingList, ok1 := existing.([]interface{})
	newList, ok2 := new.([]interface{})
	if !ok1 || !ok2 {
		if ok2 {
			return newList
		}
		return existingList
	}

	// Create map of existing items by name
	existingMap := make(map[string]interface{})
	for _, item := range existingList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if name, ok := itemMap["name"].(string); ok {
				existingMap[name] = item
			}
		}
	}

	// Add or update with new items
	for _, item := range newList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if name, ok := itemMap["name"].(string); ok {
				existingMap[name] = item
			}
		}
	}

	// Convert back to list
	result := make([]interface{}, 0, len(existingMap))
	for _, item := range existingMap {
		result = append(result, item)
	}

	return result
}
