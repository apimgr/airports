package app

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/notify"
	paths "github.com/apimgr/airports/src/path"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

type securityMaintenanceOptions struct {
	Status         *string `json:"status,omitempty"`
	Comment        *string `json:"comment,omitempty"`
	DisclosureDate *string `json:"disclosure_date,omitempty"`
	Severity       *string `json:"severity,omitempty"`
}

func validSecurityReportStatus(status string) bool {
	switch status {
	case "received", "triaged", "confirmed", "patching", "disclosed", "wont_fix":
		return true
	default:
		return false
	}
}

func canAdvanceSecurityReport(oldStatus, newStatus string) bool {
	if !validSecurityReportStatus(oldStatus) || !validSecurityReportStatus(newStatus) || oldStatus == newStatus {
		return false
	}
	if oldStatus == "disclosed" || oldStatus == "wont_fix" {
		return false
	}
	if newStatus == "wont_fix" {
		return true
	}
	order := map[string]int{"received": 0, "triaged": 1, "confirmed": 2, "patching": 3, "disclosed": 4}
	return order[newStatus] > order[oldStatus]
}

func runMaintenanceSecurity(configDir, rawArgs string) error {
	args, err := parseSecurityMaintenanceArgs(rawArgs)
	if err != nil {
		return err
	}
	if strings.TrimSpace(configDir) == "" {
		defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
		configDir = firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	}
	cfg, err := config.LoadApplicationConfig(filepath.Join(configDir, "server.yml"))
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	if !service.IsElevated() {
		token, err := promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
		if cfg.Server.Token == "" || len(token) != len(cfg.Server.Token) || subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Server.Token)) != 1 {
			return errors.New("security maintenance denied: invalid operator token")
		}
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() { _ = database.Close() }()

	switch args[0] {
	case "list":
		return listSecurityReports(database, args[1])
	case "show":
		if len(args) != 2 {
			return errors.New("usage: --maintenance security show TRACKING_ID")
		}
		return showSecurityReport(database, args[1])
	case "update":
		if len(args) < 2 {
			return errors.New("usage: --maintenance security update TRACKING_ID [--status STATUS] [--comment TEXT] [--disclosure-date YYYY-MM-DD] [--severity LEVEL]")
		}
		return updateSecurityReport(database, args[1], args[2], cfg, configDir)
	default:
		return errors.New("usage: --maintenance security {list [--status STATUS]|show TRACKING_ID|update TRACKING_ID [options]}")
	}
}

func parseSecurityMaintenanceArgs(raw string) ([]string, error) {
	fields := strings.Split(raw, "\x00")
	if len(fields) == 1 {
		fields = strings.Fields(raw)
	}
	fields = expandSecurityMaintenanceEquals(fields)
	if len(fields) == 0 {
		return nil, errors.New("usage: --maintenance security {list [--status STATUS]|show TRACKING_ID|update TRACKING_ID [options]}")
	}
	command := fields[0]
	if command != "list" && command != "show" && command != "update" {
		return nil, fmt.Errorf("unknown security maintenance action %q", command)
	}
	if command == "list" {
		if len(fields) == 1 {
			return []string{command, ""}, nil
		}
		if len(fields) != 3 || fields[1] != "--status" || !validSecurityReportStatus(fields[2]) {
			return nil, errors.New("usage: --maintenance security list [--status STATUS]")
		}
		return []string{command, fields[2]}, nil
	}
	if len(fields) < 2 {
		return nil, fmt.Errorf("usage: --maintenance security %s TRACKING_ID", command)
	}
	options := securityMaintenanceOptions{}
	for i := 2; i < len(fields); i++ {
		option := fields[i]
		if !strings.HasPrefix(option, "--") || i+1 >= len(fields) {
			return nil, fmt.Errorf("invalid security maintenance option %q", option)
		}
		i++
		value := fields[i]
		switch option {
		case "--status":
			options.Status = &value
		case "--comment":
			options.Comment = &value
		case "--disclosure-date":
			options.DisclosureDate = &value
		case "--severity":
			options.Severity = &value
		default:
			return nil, fmt.Errorf("unknown security maintenance option %q", option)
		}
	}
	if command == "show" && len(fields) != 2 {
		return nil, errors.New("usage: --maintenance security show TRACKING_ID")
	}
	if command == "show" {
		return []string{command, fields[1]}, nil
	}
	return []string{command, fields[1], encodeSecurityMaintenanceOptions(options)}, nil
}

func expandSecurityMaintenanceEquals(fields []string) []string {
	expanded := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field, "--") {
			if index := strings.IndexByte(field, '='); index >= 0 {
				expanded = append(expanded, field[:index], field[index+1:])
				continue
			}
		}
		expanded = append(expanded, field)
	}
	return expanded
}

func encodeSecurityMaintenanceOptions(options securityMaintenanceOptions) string {
	encoded, _ := json.Marshal(options)
	return string(encoded)
}

func decodeSecurityMaintenanceOptions(encoded string) securityMaintenanceOptions {
	var options securityMaintenanceOptions
	_ = json.Unmarshal([]byte(encoded), &options)
	return options
}

func listSecurityReports(database db.Handle, status string) error {
	query := `SELECT tracking_id, status, severity, component, created_at, updated_at FROM security_reports`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := database.Query(query, args...)
	if err != nil {
		return fmt.Errorf("list security reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var trackingID, reportStatus, severity, component, createdAt, updatedAt string
		if err := rows.Scan(&trackingID, &reportStatus, &severity, &component, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("read security report: %w", err)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", trackingID, reportStatus, severity, component, createdAt, updatedAt)
	}
	return rows.Err()
}

func showSecurityReport(database db.Handle, trackingID string) error {
	var status, severity, component, comment, disclosureDate, createdAt, updatedAt string
	var creditPreference string
	err := database.QueryRow(`SELECT status, severity, component, researcher_comment, COALESCE(disclosure_date, ''), created_at, updated_at, credit_pref FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&status, &severity, &component, &comment, &disclosureDate, &createdAt, &updatedAt, &creditPreference)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("security report not found")
	}
	if err != nil {
		return fmt.Errorf("show security report: %w", err)
	}
	fmt.Printf("tracking_id: %s\nstatus: %s\nseverity: %s\ncomponent: %s\ncreated_at: %s\nupdated_at: %s\nresearcher_comment: %s\ndisclosure_date: %s\ncredit_preference: %s\n", trackingID, status, severity, component, createdAt, updatedAt, comment, disclosureDate, creditPreference)
	return nil
}

func updateSecurityReport(database db.Handle, trackingID, encodedOptions string, cfg *config.ApplicationConfig, configDir string) error {
	options := decodeSecurityMaintenanceOptions(encodedOptions)
	var oldStatus string
	if err := database.QueryRow(`SELECT status FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&oldStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("security report not found")
		}
		return fmt.Errorf("read security report status: %w", err)
	}
	if err := server.ApplySecurityReportUpdate(database, trackingID, server.SecurityReportUpdate{
		Status: options.Status, Comment: options.Comment, DisclosureDate: options.DisclosureDate, Severity: options.Severity,
	}, time.Now().UTC()); err != nil {
		return fmt.Errorf("update security report: %w", err)
	}
	newStatus := oldStatus
	if options.Status != nil {
		newStatus = *options.Status
	}
	_, _, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	logsDir := firstNonEmpty(os.Getenv("LOG_DIR"), defaultLogsDir)
	logManager := logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
	defer func() { _ = logManager.Close() }()
	if err := logManager.Security().Write(fmt.Sprintf("event=security.report_updated tracking_id=%q old_status=%q new_status=%q", trackingID, oldStatus, newStatus)); err != nil {
		return fmt.Errorf("write security report audit event: %w", err)
	}
	var contactCiphertext []byte
	if err := database.QueryRow(`SELECT contact_enc FROM security_reports WHERE tracking_id = ?`, trackingID).Scan(&contactCiphertext); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read researcher contact: %w", err)
	}
	if oldStatus == newStatus || len(contactCiphertext) == 0 || !notify.CanSend(cfg) {
		return nil
	}
	email, pubkey, err := server.DecryptSecurityContact(cfg.Server.Security.EncryptionKey, contactCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt researcher contact: %w", err)
	}
	if pubkey != "" {
		log.Printf("security report status notice skipped for %s because PGP notification encryption is not available", trackingID)
		return nil
	}
	if err := notify.Send(cfg, configDir, "security_report_status", email, map[string]string{
		"tracking_id": trackingID,
		"status":      newStatus,
	}); err != nil {
		log.Printf("security report status notice failed for %s: %v", trackingID, err)
	}
	return nil
}
