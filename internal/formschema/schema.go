package formschema

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
	var routing Schema
	return json.Unmarshal(raw, &routing) == nil && ValidRouting(routing)
}

func ValidRouting(schema Schema) bool {
	if schema.Version != nil && *schema.Version != 2 {
		return false
	}
	ids := map[string]int{}
	keys := map[string]bool{}
	profiles := 0
	for index, section := range schema.Fields {
		for _, field := range section.Fields {
			if strings.TrimSpace(field.Key) == "" || keys[field.Key] {
				return false
			}
			keys[field.Key] = true
		}
		if section.Name == "profile_data" {
			profiles++
			if profiles > 1 || index != 0 {
				return false
			}
		}
		if schema.Version != nil && (section.ID == nil || strings.TrimSpace(*section.ID) == "") {
			return false
		}
		if section.ID != nil {
			if _, exists := ids[*section.ID]; exists || strings.TrimSpace(*section.ID) == "" {
				return false
			}
			ids[*section.ID] = index
		}
	}
	for index, section := range schema.Fields {
		nav := section.Navigation
		if nav == nil {
			continue
		}
		if section.ID == nil || section.Name == "profile_data" {
			return false
		}
		validTarget := func(target Destination) bool {
			switch target.Type {
			case "next", "submit":
				return true
			case "section":
				destination, exists := ids[target.SectionID]
				return exists && destination > index && schema.Fields[destination].Name != "profile_data"
			default:
				return false
			}
		}
		if !validTarget(nav.DefaultTarget) {
			return false
		}
		allowed := map[string]bool{}
		if nav.QuestionKey != nil {
			var question *Field
			for i := range section.Fields {
				if section.Fields[i].Key == *nav.QuestionKey {
					question = &section.Fields[i]
				}
			}
			if question == nil || (question.Type != "radio" && question.Type != "select") || question.Hidden != nil && *question.Hidden || question.Disabled != nil && *question.Disabled {
				return false
			}
			if question.Options != nil {
				for _, option := range *question.Options {
					if option.Disabled != nil && *option.Disabled {
						continue
					}
					value := option.Label
					var raw any
					if len(option.Value) > 0 {
						if json.Unmarshal(option.Value, &raw) != nil {
							return false
						}
						if raw != nil && optionText(raw) != "" {
							value = optionText(raw)
						}
					}
					if allowed[value] {
						return false
					}
					allowed[value] = true
				}
			}
		}
		seen := map[string]bool{}
		for _, route := range nav.Routes {
			if !allowed[route.OptionValue] || seen[route.OptionValue] || !validTarget(route.Target) {
				return false
			}
			seen[route.OptionValue] = true
		}
	}
	return true
}
