package logging

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxRecentLogEntries = 10000
const maxRecentLogLineRunes = 512

var (
	credentialFieldPattern = regexp.MustCompile(`(?i)(["']?[a-z0-9_.-]*(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|private[_-]?key|authorization)[a-z0-9_.-]*["']?\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;&]+)`)
	authorizationPattern   = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)[^\r\n]+`)
	cookieHeaderPattern    = regexp.MustCompile(`(?i)((?:set-)?cookie\s*[:=]\s*)[^\r\n]+`)
	urlUserInfoPattern     = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^:/\s]+:)[^@/\s]+@`)
)

// RecentLogEntry is a sanitized log line with the time it entered the recent-log buffer.
type RecentLogEntry struct {
	Timestamp time.Time
	Line      string
}

// RecentLogBuffer is a bounded, concurrency-safe sink for recent application log lines.
type RecentLogBuffer struct {
	mu           sync.RWMutex
	entries      []RecentLogEntry
	start        int
	count        int
	redactingPEM bool
}

// NewRecentLogBuffer creates a bounded recent-log buffer. Capacities outside
// the supported range use the safe default or the maximum capacity.
func NewRecentLogBuffer(capacity int) *RecentLogBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	if capacity > maxRecentLogEntries {
		capacity = maxRecentLogEntries
	}
	return &RecentLogBuffer{entries: make([]RecentLogEntry, capacity)}
}

// Write stores sanitized lines and implements io.Writer for log.Logger teeing.
func (b *RecentLogBuffer) Write(p []byte) (int, error) {
	if b == nil || len(p) == 0 {
		return len(p), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) == 0 {
		b.entries = make([]RecentLogEntry, 1000)
	}

	now := time.Now().UTC()
	for _, raw := range strings.Split(string(p), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		line = b.sanitize(line)
		runes := []rune(line)
		if len(runes) > maxRecentLogLineRunes {
			line = string(runes[:maxRecentLogLineRunes])
		}
		entry := RecentLogEntry{Timestamp: now, Line: line}
		if b.count < len(b.entries) {
			index := (b.start + b.count) % len(b.entries)
			b.entries[index] = entry
			b.count++
			continue
		}
		b.entries[b.start] = entry
		b.start = (b.start + 1) % len(b.entries)
	}
	return len(p), nil
}

// Recent returns the newest entries within maxAge, ordered oldest to newest.
func (b *RecentLogBuffer) Recent(maxEntries int, maxAge time.Duration, now time.Time) []RecentLogEntry {
	if b == nil || maxEntries <= 0 {
		return []RecentLogEntry{}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	cutoff := now.Add(-maxAge)
	entries := make([]RecentLogEntry, 0, min(maxEntries, b.count))
	for i := 0; i < b.count; i++ {
		entry := b.entries[(b.start+i)%len(b.entries)]
		if !entry.Timestamp.Before(cutoff) {
			entries = append(entries, entry)
		}
	}
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	return entries
}

func (b *RecentLogBuffer) sanitize(line string) string {
	if b.redactingPEM {
		if strings.Contains(line, "-----END ") {
			b.redactingPEM = false
		}
		return "[redacted credential block]"
	}
	if strings.Contains(line, "-----BEGIN ") && (strings.Contains(strings.ToUpper(line), "PRIVATE KEY") || strings.Contains(strings.ToUpper(line), "SECRET")) {
		b.redactingPEM = !strings.Contains(line, "-----END ")
		return "[redacted credential block]"
	}
	line = authorizationPattern.ReplaceAllString(line, "${1}xxxxx")
	line = cookieHeaderPattern.ReplaceAllString(line, "${1}xxxxx")
	line = urlUserInfoPattern.ReplaceAllString(line, "${1}xxxxx@")
	return credentialFieldPattern.ReplaceAllString(line, "${1}xxxxx")
}
