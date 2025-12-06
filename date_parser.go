package main

import (
	"strings"
	"time"
)

// ParseDateFromTitle extracts and parses a date from the PR title
// Supports formats:
// - [Dec 12] The Rest of the Title
// - Dec 12: The Rest of the Title
// - Dec 12 / The rest of the title
// Returns a zero time.Time if no valid date is found
func ParseDateFromTitle(title string) time.Time {
	// Try bracket format: [Dec 12]
	if dateStr := extractBracketDate(title); dateStr != "" {
		if date := parseDate(dateStr); !date.IsZero() {
			return date
		}
	}

	// Try colon format: Dec 12:
	if dateStr := extractPrefixDate(title, ":"); dateStr != "" {
		if date := parseDate(dateStr); !date.IsZero() {
			return date
		}
	}

	// Try slash format: Dec 12 /
	if dateStr := extractPrefixDate(title, "/"); dateStr != "" {
		if date := parseDate(dateStr); !date.IsZero() {
			return date
		}
	}

	return time.Time{}
}

// extractBracketDate extracts date from [Date] format
func extractBracketDate(title string) string {
	start := strings.Index(title, "[")
	end := strings.Index(title, "]")

	if start == -1 || end == -1 || start >= end {
		return ""
	}

	return strings.TrimSpace(title[start+1 : end])
}

// extractPrefixDate extracts date from "Date<separator>" format at the start of title
func extractPrefixDate(title, separator string) string {
	idx := strings.Index(title, separator)
	if idx == -1 {
		return ""
	}

	return strings.TrimSpace(title[:idx])
}

// parseDate attempts to parse a date string in various formats
// Supports formats like:
// - Dec 12, December 12
// - Dec 12 2024, December 12 2024
// - 12/12, 12/12/2024
// - 2024-12-12
func parseDate(dateStr string) time.Time {
	now := time.Now()
	currentYear := now.Year()

	// List of date formats to try
	formats := []string{
		// Month day formats (will use current year)
		"Jan 2",
		"January 2",
		"Jan 02",
		"January 02",

		// Month day year formats
		"Jan 2 2006",
		"January 2 2006",
		"Jan 02 2006",
		"January 02 2006",

		// Numeric formats
		"1/2",
		"01/02",
		"1/2/2006",
		"01/02/2006",
		"1/2/06",
		"01/02/06",

		// ISO-like formats
		"2006-01-02",
		"2006/01/02",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			// If the format doesn't include a year, add the current year
			if !strings.Contains(format, "2006") && !strings.Contains(format, "06") {
				parsed = time.Date(currentYear, parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
			}
			return parsed
		}
	}

	return time.Time{}
}
