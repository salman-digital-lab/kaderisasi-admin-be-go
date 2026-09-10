package form

import (
	"encoding/json"
	"strings"
)

func ValidSchema(raw []byte) bool {
	var schema struct {
		Fields *[]struct {
			Name   *string `json:"section_name"`
			Fields *[]struct {
				Key      string `json:"key"`
				Label    string `json:"label"`
				Type     string `json:"type"`
				Required *bool  `json:"required"`
			} `json:"fields"`
		} `json:"fields"`
	}
	if json.Unmarshal(raw, &schema) != nil || schema.Fields == nil {
		return false
	}
	keys := map[string]bool{}
	for _, section := range *schema.Fields {
		if section.Fields == nil || section.Name == nil {
			return false
		}
		for _, field := range *section.Fields {
			if strings.TrimSpace(field.Key) == "" || strings.TrimSpace(field.Label) == "" || strings.TrimSpace(field.Type) == "" || field.Required == nil || keys[field.Key] {
				return false
			}
			keys[field.Key] = true
		}
	}
	return true
}
