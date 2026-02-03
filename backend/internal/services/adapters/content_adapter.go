package adapters

import (
	"encoding/json"
	"fmt"
	"strings"

	"risk-analyzer/internal/models"
)

type ContentAdapter interface {
	CanHandle(sourceType models.ContentSourceType) bool
	ToText(payload interface{}) (string, error)
	ToMetadata(payload interface{}) map[string]interface{}
}

type ContentAdapterRegistry struct {
	adapters []ContentAdapter
}

func NewContentAdapterRegistry() *ContentAdapterRegistry {
	return &ContentAdapterRegistry{
		adapters: []ContentAdapter{
			&JiraAdapter{},
			&NotionAdapter{},
			&MarkdownAdapter{},
			&JSONAdapter{},
		},
	}
}

func (r *ContentAdapterRegistry) GetAdapter(sourceType models.ContentSourceType) ContentAdapter {
	for _, a := range r.adapters {
		if a.CanHandle(sourceType) {
			return a
		}
	}
	return nil
}

type JiraAdapter struct{}

func (a *JiraAdapter) CanHandle(sourceType models.ContentSourceType) bool {
	return sourceType == models.SourceTypeJira
}

func (a *JiraAdapter) ToText(payload interface{}) (string, error) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid jira payload")
	}

	var sb strings.Builder

	if key, ok := data["key"].(string); ok {
		sb.WriteString(fmt.Sprintf("Issue: %s\n", key))
	}
	if summary, ok := data["summary"].(string); ok {
		sb.WriteString(fmt.Sprintf("Summary: %s\n", summary))
	}
	if status, ok := data["status"].(string); ok {
		sb.WriteString(fmt.Sprintf("Status: %s\n", status))
	}
	if priority, ok := data["priority"].(string); ok {
		sb.WriteString(fmt.Sprintf("Priority: %s\n", priority))
	}
	if issueType, ok := data["issue_type"].(string); ok {
		sb.WriteString(fmt.Sprintf("Issue Type: %s\n", issueType))
	}
	if project, ok := data["project"].(string); ok {
		sb.WriteString(fmt.Sprintf("Project: %s\n", project))
	}
	if description, ok := data["description"].(string); ok {
		sb.WriteString(fmt.Sprintf("\nDescription:\n%s\n", description))
	}
	if reporter, ok := data["reporter"].(string); ok {
		sb.WriteString(fmt.Sprintf("\nReporter: %s\n", reporter))
	}
	if assignee, ok := data["assignee"].(string); ok {
		sb.WriteString(fmt.Sprintf("Assignee: %s\n", assignee))
	}

	if comments, ok := data["comments"].([]interface{}); ok && len(comments) > 0 {
		sb.WriteString("\nComments:\n")
		for _, c := range comments {
			if comment, ok := c.(map[string]interface{}); ok {
				if author, ok := comment["author"].(string); ok {
					if body, ok := comment["body"].(string); ok {
						sb.WriteString(fmt.Sprintf("- %s: %s\n", author, body))
					}
				}
			}
		}
	}

	if labels, ok := data["labels"].([]interface{}); ok && len(labels) > 0 {
		sb.WriteString("\nLabels: ")
		for i, l := range labels {
			if i > 0 {
				sb.WriteString(", ")
			}
			if label, ok := l.(string); ok {
				sb.WriteString(label)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (a *JiraAdapter) ToMetadata(payload interface{}) map[string]interface{} {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}

	metadata := make(map[string]interface{})

	if key, ok := data["key"].(string); ok {
		metadata["jira_key"] = key
	}
	if project, ok := data["project"].(string); ok {
		metadata["jira_project"] = project
	}
	if status, ok := data["status"].(string); ok {
		metadata["jira_status"] = status
	}
	if priority, ok := data["priority"].(string); ok {
		metadata["jira_priority"] = priority
	}
	if issueType, ok := data["issue_type"].(string); ok {
		metadata["jira_issue_type"] = issueType
	}
	if labels, ok := data["labels"].([]interface{}); ok {
		labelStrings := make([]string, len(labels))
		for i, l := range labels {
			if label, ok := l.(string); ok {
				labelStrings[i] = label
			}
		}
		metadata["jira_labels"] = labelStrings
	}

	return metadata
}

type NotionAdapter struct{}

func (a *NotionAdapter) CanHandle(sourceType models.ContentSourceType) bool {
	return sourceType == models.SourceTypeNotion
}

func (a *NotionAdapter) ToText(payload interface{}) (string, error) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid notion payload")
	}

	var sb strings.Builder

	if title, ok := data["title"].(string); ok {
		sb.WriteString(fmt.Sprintf("Title: %s\n", title))
	}
	if url, ok := data["url"].(string); ok {
		sb.WriteString(fmt.Sprintf("URL: %s\n", url))
	}

	if content, ok := data["content"].([]interface{}); ok {
		sb.WriteString("\nContent:\n")
		text := convertBlocksToText(content, 0)
		sb.WriteString(text)
	}

	return sb.String(), nil
}

func convertBlocksToText(blocks []interface{}, indent int) string {
	var sb strings.Builder
	indentStr := strings.Repeat("  ", indent)

	for _, b := range blocks {
		if block, ok := b.(map[string]interface{}); ok {
			prefix := indentStr

			if blockType, ok := block["type"].(string); ok {
				switch blockType {
				case "heading_1":
					sb.WriteString(fmt.Sprintf("%s# %s\n\n", indentStr, getBlockText(block)))
				case "heading_2":
					sb.WriteString(fmt.Sprintf("%s## %s\n\n", indentStr, getBlockText(block)))
				case "heading_3":
					sb.WriteString(fmt.Sprintf("%s### %s\n\n", indentStr, getBlockText(block)))
				case "paragraph":
					sb.WriteString(fmt.Sprintf("%s%s\n\n", indentStr, getBlockText(block)))
				case "bulleted_list_item":
					sb.WriteString(fmt.Sprintf("%s- %s\n", indentStr, getBlockText(block)))
				case "numbered_list_item":
					sb.WriteString(fmt.Sprintf("%s1. %s\n", indentStr, getBlockText(block)))
				case "to_do":
					checked := ""
					if checkedVal, ok := block["checked"].(bool); ok && checkedVal {
						checked = "[x]"
					} else {
						checked = "[ ]"
					}
					sb.WriteString(fmt.Sprintf("%s%s %s\n", indentStr, checked, getBlockText(block)))
				case "code":
					if language, ok := block["language"].(string); ok {
						sb.WriteString(fmt.Sprintf("%s```%s\n%s%s```\n\n", indentStr, language, indentStr, getBlockText(block)))
					} else {
						sb.WriteString(fmt.Sprintf("%s```\n%s%s```\n\n", indentStr, getBlockText(block), indentStr))
					}
				case "quote":
					sb.WriteString(fmt.Sprintf("%s> %s\n\n", indentStr, getBlockText(block)))
				case "divider":
					sb.WriteString(fmt.Sprintf("%s---\n\n", indentStr))
				case "toggle":
					sb.WriteString(fmt.Sprintf("%s%s\n", indentStr, getBlockText(block)))
				default:
					sb.WriteString(fmt.Sprintf("%s%s\n\n", prefix, getBlockText(block)))
				}

				if children, ok := block["children"].([]interface{}); ok && len(children) > 0 {
					sb.WriteString(convertBlocksToText(children, indent+1))
				}
			}
		}
	}

	return sb.String()
}

func getBlockText(block map[string]interface{}) string {
	if richText, ok := block["rich_text"].([]interface{}); ok && len(richText) > 0 {
		var parts []string
		for _, rt := range richText {
			if textObj, ok := rt.(map[string]interface{}); ok {
				if text, ok := textObj["text"].(string); ok {
					parts = append(parts, text)
				} else if content, ok := textObj["content"].(string); ok {
					parts = append(parts, content)
				}
			}
		}
		return strings.Join(parts, "")
	}
	if content, ok := block["content"].(string); ok {
		return content
	}
	return ""
}

func (a *NotionAdapter) ToMetadata(payload interface{}) map[string]interface{} {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}

	metadata := make(map[string]interface{})

	if id, ok := data["id"].(string); ok {
		metadata["notion_id"] = id
	}
	if url, ok := data["url"].(string); ok {
		metadata["notion_url"] = url
	}
	if createdBy, ok := data["created_by"].(string); ok {
		metadata["notion_created_by"] = createdBy
	}
	if lastEditedBy, ok := data["last_edited_by"].(string); ok {
		metadata["notion_last_edited_by"] = lastEditedBy
	}

	return metadata
}

type MarkdownAdapter struct{}

func (a *MarkdownAdapter) CanHandle(sourceType models.ContentSourceType) bool {
	return sourceType == models.SourceTypeMarkdown
}

func (a *MarkdownAdapter) ToText(payload interface{}) (string, error) {
	switch v := payload.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprintf("%v", payload), nil
	}
}

func (a *MarkdownAdapter) ToMetadata(payload interface{}) map[string]interface{} {
	return nil
}

type JSONAdapter struct{}

func (a *JSONAdapter) CanHandle(sourceType models.ContentSourceType) bool {
	return sourceType == models.SourceTypeJSON
}

func (a *JSONAdapter) ToText(payload interface{}) (string, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal json: %w", err)
	}
	return string(data), nil
}

func (a *JSONAdapter) ToMetadata(payload interface{}) map[string]interface{} {
	return nil
}
