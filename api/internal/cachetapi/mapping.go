package cachetapi

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Cachet status integers and their Vigie equivalents.
var (
	componentStatusToVigie = map[int]string{
		1: "operational",
		2: "degraded_performance",
		3: "partial_outage",
		4: "major_outage",
	}
	componentStatusNames = map[int]string{
		0: "Unknown",
		1: "Operational",
		2: "Performance Issues",
		3: "Partial Outage",
		4: "Major Outage",
	}
	incidentStatusToVigie = map[int]string{
		1: "investigating",
		2: "identified",
		3: "monitoring",
		4: "resolved",
	}
	incidentStatusNames = map[int]string{
		1: "Investigating",
		2: "Identified",
		3: "Watching",
		4: "Fixed",
	}
	scheduleStatusToVigie = map[int]string{
		0: "scheduled",
		1: "in_progress",
		2: "completed",
	}
	scheduleStatusNames = map[int]string{
		0: "Upcoming",
		1: "In Progress",
		2: "Complete",
	}
)

func invert(m map[int]string) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	componentStatusFromVigie = invert(componentStatusToVigie)
	incidentStatusFromVigie  = invert(incidentStatusToVigie)
	scheduleStatusFromVigie  = invert(scheduleStatusToVigie)
)

// cachetComponentStatus maps a Vigie component status to Cachet's integer.
// Cachet 2.4 has no maintenance status: under_maintenance is reported as 0
// (Unknown), which clients do not treat as an outage.
func cachetComponentStatus(status string) int {
	if v, ok := componentStatusFromVigie[status]; ok {
		return v
	}
	return 0
}

func cachetComponentStatusName(status string) string {
	if status == "under_maintenance" {
		return "Under Maintenance"
	}
	return componentStatusNames[cachetComponentStatus(status)]
}

// Collapse and visibility of component groups. Cachet's collapsed is 0
// (never), 1 (always) or 2 (while all components are operational); visible
// is 0 (logged-in users only) or 1 (everyone).
var (
	groupCollapseToVigie   = map[int]string{0: "never", 1: "always", 2: "operational"}
	groupCollapseFromVigie = invert(groupCollapseToVigie)
	groupVisibleToVigie    = map[int]string{0: "authenticated", 1: "public"}
	groupVisibleFromVigie  = invert(groupVisibleToVigie)
)

// cachetTime formats times as Cachet does ("2006-01-02 15:04:05"), in UTC.
func cachetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func cachetTimePtr(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return cachetTime(*t)
}

// parseCachetTime reads the date formats Cachet clients send.
func parseCachetTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"02/01/2006 15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", s)
}

// contentLocale is the locale Cachet's single-language text (descriptions,
// messages) is stored under. CACHET_API_LOCALE overrides the default, "en".
func contentLocale() string {
	if l := strings.ToLower(strings.TrimSpace(os.Getenv("CACHET_API_LOCALE"))); l == "en" || l == "fr" {
		return l
	}
	return "en"
}

// publicURL is the public status page, used for permalinks.
func publicURL() string {
	if u := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"); u != "" {
		return u
	}
	return "http://localhost:3000"
}
