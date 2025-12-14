package models

type Component int

const (
	Console Component = iota
	BigRedButton
	LOC
	HotFixing
	Jenkins
	Builds
	ACLOS
	China
)

func (c Component) String() string {
	switch c {
	case Console:
		return "Console"
	case BigRedButton:
		return "BigRedButton"
	case LOC:
		return "LOC"
	case HotFixing:
		return "HotFixing"
	case Jenkins:
		return "Jenkins"
	case Builds:
		return "Builds"
	case ACLOS:
		return "ACLOS"
	case China:
		return "China"
	default:
		return "Unknown"
	}
}

// MarshalJSON converts Component to JSON string
func (c Component) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.String() + `"`), nil
}

// UnmarshalJSON converts JSON string to Component
func (c *Component) UnmarshalJSON(data []byte) error {
	// Remove quotes from JSON string
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	switch str {
	case "Console":
		*c = Console
	case "BigRedButton":
		*c = BigRedButton
	case "LOC":
		*c = LOC
	case "HotFixing":
		*c = HotFixing
	case "Jenkins":
		*c = Jenkins
	case "Builds":
		*c = Builds
	case "ACLOS":
		*c = ACLOS
	case "China":
		*c = China
	default:
		*c = Console // Default to Console for unknown values
	}
	return nil
}

type Issue struct {
	Id          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	IssueType   string      `json:"issue_type"`
	Components  []Component `json:"components"`
}
