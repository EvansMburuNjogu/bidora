package browser

import "strings"

// Name reduces client-supplied User-Agent text to a bounded display label.
// It is informational, never a verified identity or authorization signal.
func Name(agent string) string {
	if agent == "" {
		return ""
	}
	for _, name := range []string{"Edge", "Opera", "Firefox", "Chrome", "Safari", "Internet Explorer", "Unknown browser"} {
		if agent == name {
			return name
		}
	}
	switch {
	case strings.Contains(agent, "Edg/") || strings.Contains(agent, "EdgA/") || strings.Contains(agent, "EdgiOS/"):
		return "Edge"
	case strings.Contains(agent, "OPR/") || strings.Contains(agent, "Opera"):
		return "Opera"
	case strings.Contains(agent, "Firefox/") || strings.Contains(agent, "FxiOS/"):
		return "Firefox"
	case strings.Contains(agent, "Chrome/") || strings.Contains(agent, "CriOS/"):
		return "Chrome"
	case strings.Contains(agent, "Safari/") && strings.Contains(agent, "Version/"):
		return "Safari"
	case strings.Contains(agent, "MSIE ") || strings.Contains(agent, "Trident/"):
		return "Internet Explorer"
	default:
		return "Unknown browser"
	}
}
