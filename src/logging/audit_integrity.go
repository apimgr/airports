package logging

import (
	"bufio"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const auditChainGenesis = "0000000000000000000000000000000000000000000000000000000000000000"

type auditEntry struct {
	ID       string         `json:"id"`
	Time     string         `json:"time"`
	Event    string         `json:"event"`
	Category string         `json:"category"`
	Severity string         `json:"severity"`
	Actor    map[string]any `json:"actor"`
	Target   map[string]any `json:"target,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Source   string         `json:"source,omitempty"`
	Result   string         `json:"result"`
	Prev     string         `json:"prev"`
}

// AuditIntegrityResult describes a verification of the active and retained audit files.
type AuditIntegrityResult struct {
	Intact   bool
	Entries  int
	File     string
	Line     int
	LastHash string
}

// VerifyAuditLog verifies the active audit file against its persisted chain anchor.
func VerifyAuditLog(dir, filename string) AuditIntegrityResult {
	if filename == "" {
		filename = "audit.log"
	}
	active := filepath.Join(dir, filename)
	anchor, hasAnchor, err := readAuditAnchor(dir, filename)
	if err != nil {
		return AuditIntegrityResult{File: filepath.Base(active), Line: 1}
	}
	if !hasAnchor {
		anchor = auditChainGenesis
	}
	info, err := os.Stat(active)
	if errors.Is(err, os.ErrNotExist) {
		return AuditIntegrityResult{Intact: true, LastHash: anchor}
	}
	if err != nil {
		return AuditIntegrityResult{File: filepath.Base(active), Line: 1}
	}
	if info.Size() == 0 {
		return AuditIntegrityResult{Intact: true, LastHash: anchor}
	}
	count, last, badLine, err := verifyAuditFile(active, anchor)
	result := AuditIntegrityResult{Intact: err == nil, Entries: count, LastHash: last}
	if err != nil {
		result.File = filepath.Base(active)
		result.Line = badLine
	}
	return result
}

func auditAnchorPath(dir, filename string) string {
	if filename == "" {
		filename = "audit.log"
	}
	return filepath.Join(dir, filename+".chain")
}

func readAuditAnchor(dir, filename string) (string, bool, error) {
	data, err := os.ReadFile(auditAnchorPath(dir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var anchor struct {
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(data, &anchor); err != nil || len(anchor.Previous) != 64 {
		return "", true, errors.New("invalid audit chain anchor")
	}
	if _, err := hex.DecodeString(anchor.Previous); err != nil || strings.ToLower(anchor.Previous) != anchor.Previous {
		return "", true, errors.New("invalid audit chain anchor")
	}
	return anchor.Previous, true, nil
}

func writeAuditAnchor(dir, filename, previous string) error {
	path := auditAnchorPath(dir, filename)
	data, err := json.Marshal(struct {
		Previous string `json:"previous"`
	}{Previous: previous})
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

func auditRotationJournalPath(dir, filename string) string {
	return auditAnchorPath(dir, filename) + ".rotate"
}

func writeAuditRotationJournal(dir, filename, previous string) error {
	path := auditRotationJournalPath(dir, filename)
	data, err := json.Marshal(struct {
		Previous string `json:"previous"`
	}{Previous: previous})
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return syncAuditDirectory(dir)
}

// recoverAuditRotation completes or cancels an interrupted rotation while
// the caller holds the cross-process audit lock.
func recoverAuditRotation(dir, filename string) error {
	if filename == "" {
		filename = "audit.log"
	}
	path := auditRotationJournalPath(dir, filename)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal struct {
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(data, &journal); err != nil || len(journal.Previous) != 64 {
		return errors.New("invalid audit rotation journal")
	}
	if _, err := hex.DecodeString(journal.Previous); err != nil || strings.ToLower(journal.Previous) != journal.Previous {
		return errors.New("invalid audit rotation journal")
	}
	active := filepath.Join(dir, filename)
	info, err := os.Stat(active)
	if errors.Is(err, os.ErrNotExist) || (err == nil && info.Size() == 0) {
		if err := writeAuditAnchor(dir, filename, journal.Previous); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		integrity := VerifyAuditLog(dir, filename)
		if integrity.LastHash != journal.Previous {
			return errors.New("audit rotation journal does not match active log")
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncAuditDirectory(dir)
}

func syncAuditDirectory(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

func verifyAuditFile(path, previous string) (int, string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", 1, err
	}
	defer func() { _ = file.Close() }()

	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return 0, "", 1, err
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}

	result := 0
	lastHash := ""
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		hash := sha256.Sum256(line)
		lineHash := hex.EncodeToString(hash[:])
		var entry struct {
			Prev string `json:"prev"`
		}
		if err := json.Unmarshal(line, &entry); err != nil || len(entry.Prev) != 64 {
			return result, lineHash, lineNumber, errors.New("invalid audit JSON or hash")
		}
		if _, err := hex.DecodeString(entry.Prev); err != nil || strings.ToLower(entry.Prev) != entry.Prev {
			return result, lineHash, lineNumber, errors.New("invalid audit hash encoding")
		}
		lastHash = lineHash
		if entry.Prev != previous {
			return result, lastHash, lineNumber, errors.New("audit hash chain mismatch")
		}
		previous = lastHash
		result++
	}
	if err := scanner.Err(); err != nil {
		return result, lastHash, lineNumber + 1, err
	}
	return result, lastHash, 0, nil
}

func newAuditID(now time.Time) (string, error) {
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	var raw [16]byte
	ms := uint64(now.UnixMilli())
	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)
	copy(raw[6:], entropy[:])
	var value [17]byte
	copy(value[1:], raw[:])
	value[0] = 0
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	encoded := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		encoded[i] = alphabet[value[16]&31]
		for j := 16; j > 0; j-- {
			value[j] = (value[j] >> 5) | (value[j-1] << 3)
		}
		value[0] >>= 5
	}
	return "audit_" + string(encoded), nil
}

func auditEntryJSON(entry auditEntry) ([]byte, error) {
	return json.Marshal(entry)
}

var auditFieldPattern = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)=("(?:\\.|[^"])*"|[^\s]+)`)

func makeAuditEntry(category AuditCategory, line, previous string, now time.Time) (auditEntry, error) {
	line = strings.TrimSpace(line)
	fields := make(map[string]string)
	for _, match := range auditFieldPattern.FindAllStringSubmatch(line, -1) {
		value := match[2]
		if strings.HasPrefix(value, `"`) {
			if decoded, err := strconv.Unquote(value); err == nil {
				value = decoded
			}
		}
		fields[match[1]] = value
	}
	event := fields["event"]
	if event == "" {
		for _, part := range strings.Fields(line) {
			if strings.Contains(part, ".") && !strings.Contains(part, "=") {
				event = part
				break
			}
		}
	}
	if event == "" {
		event = "server.event"
	}
	categoryName := string(category)
	switch strings.SplitN(event, ".", 2)[0] {
	case "config":
		categoryName = "configuration"
	case "security":
		categoryName = "security"
	case "backup":
		categoryName = "backup"
	case "scheduler", "server":
		categoryName = "server"
	case "compliance":
		categoryName = "compliance"
	}
	severity := "info"
	if strings.HasSuffix(event, ".failed") || strings.HasSuffix(event, ".failure") || strings.Contains(event, "backup.failed") || strings.Contains(event, "scheduler.task_failed") {
		severity = "error"
	} else if strings.Contains(event, "invalid") || strings.Contains(event, "blocked") || strings.Contains(event, "csrf_failure") || strings.Contains(event, "rate_limit_exceeded") {
		severity = "warn"
	}
	if strings.Contains(event, "audit_chain_broken") || strings.Contains(event, "suspicious_activity") || event == "server.maintenance_entered" {
		severity = "critical"
	}
	result := fields["result"]
	if result != "success" && result != "failure" {
		result = "success"
		if strings.HasSuffix(event, ".failed") || strings.HasSuffix(event, ".failure") || strings.Contains(event, "_failed") || strings.Contains(event, "invalid") || strings.Contains(event, "blocked") || strings.Contains(event, "csrf_failure") || strings.Contains(event, "audit_chain_broken") {
			result = "failure"
		}
	}
	id, err := newAuditID(now)
	if err != nil {
		return auditEntry{}, err
	}
	actorIP := fields["ip"]
	if actorIP == "" {
		actorIP = fields["operator_ip"]
	}
	if actorIP == "" {
		actorIP = fields["actor_ip"]
	}
	actor := map[string]any{}
	if actorType := fields["actor_type"]; actorType != "" {
		actor["type"] = actorType
	}
	if actorIP != "" {
		actor["ip"] = actorIP
	}
	if actorID := fields["actor"]; actorID != "" {
		actor["id"] = actorID
	}
	details := make(map[string]any)
	for key, value := range fields {
		switch key {
		case "event", "ip", "operator_ip", "result", "source", "reason", "actor", "actor_type", "actor_ip", "target_type", "target_id":
		case "details_source":
			details["source"] = maskSensitiveAuditValue("source", value)
		default:
			details[key] = maskSensitiveAuditValue(key, value)
		}
	}
	entry := auditEntry{
		ID:       id,
		Time:     now.UTC().Format("2006-01-02T15:04:05.000Z"),
		Event:    event,
		Category: categoryName,
		Severity: severity,
		Actor:    actor,
		Result:   result,
		Prev:     previous,
		Source:   fields["source"],
	}
	if reason := fields["reason"]; reason != "" {
		entry.Reason = reason
	}
	if targetType, targetID := fields["target_type"], fields["target_id"]; targetType != "" || targetID != "" {
		entry.Target = map[string]any{"type": targetType, "id": targetID}
	}
	if len(details) > 0 {
		entry.Details = details
	}
	return entry, nil
}

func maskSensitiveAuditValue(key, value string) string {
	normalized := strings.ToLower(key)
	if strings.Contains(normalized, "token_id") || strings.Contains(normalized, "token_prefix") {
		return value
	}
	for _, sensitive := range []string{"password", "credential", "secret", "private_key", "api_key", "access_token", "refresh_token"} {
		if strings.Contains(normalized, sensitive) {
			return "********"
		}
	}
	if normalized == "token" || strings.HasSuffix(normalized, "_token") {
		if len(value) > 8 {
			return value[:8] + "..."
		}
		return "********"
	}
	return value
}
