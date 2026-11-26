package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	paths "github.com/apimgr/airports/src/paths"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

type ownerExport struct {
	Format    int                 `json:"format"`
	Owner     ownerExportIdentity `json:"owner"`
	Tokens    []ownerExportToken  `json:"tokens"`
	Resources []json.RawMessage   `json:"resources"`
}

type ownerExportIdentity struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

type ownerExportToken struct {
	Prefix    string `json:"prefix"`
	Kind      string `json:"kind"`
	Name      string `json:"name,omitempty"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used_at,omitempty"`
	Expires   string `json:"expires_at,omitempty"`
	Status    string `json:"status"`
}

func runMaintenanceData(configDir, logsDir, rawArgs string) error {
	args := strings.Fields(rawArgs)
	if len(args) == 0 {
		return errors.New("usage: --maintenance data {export|delete} --owner <prefix> [--output FILE|--yes]")
	}
	defaultConfigDir, _, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	configDir = firstNonEmpty(configDir, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	logsDir = firstNonEmpty(logsDir, os.Getenv("LOG_DIR"), defaultLogsDir)
	cfg, err := config.LoadApplicationConfig(filepath.Join(configDir, "server.yml"))
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	if !service.IsElevated() {
		provided, err := promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
		if cfg.Server.Token == "" || len(provided) != len(cfg.Server.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.Server.Token)) != 1 {
			return errors.New("data maintenance denied: invalid operator token")
		}
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() { _ = database.Close() }()
	if _, err := server.ListOwnerTokens(database); err != nil {
		return err
	}
	if err := ensureComplianceEventSchema(database); err != nil {
		return err
	}
	prefix, output, yes, err := parseDataMaintenanceArgs(args[1:])
	if err != nil {
		return err
	}
	if prefix == "" {
		return errors.New("--owner <prefix> is required")
	}
	owner, err := resolveOwnerByPrefix(database, prefix)
	if err != nil {
		return err
	}
	switch args[0] {
	case "export":
		if output == "" {
			output = fmt.Sprintf("%s-export-%s-%s.json", ProjectName, owner, time.Now().UTC().Format("20060102"))
		}
		return exportOwnerData(database, logsDir, toLoggingConfig(cfg.Server.Logging), owner, output)
	case "delete":
		if output != "" {
			return errors.New("--output is only valid with data export")
		}
		if !yes {
			if err := confirmOwnerDeletion(owner); err != nil {
				return err
			}
		}
		requestID, err := newComplianceRequestID()
		if err != nil {
			return err
		}
		requestDetails := "request_id=" + requestID
		if err := recordComplianceEvent(database, "compliance.data_deletion_requested", owner, requestDetails); err != nil {
			return err
		}
		if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "compliance.data_deletion_requested", owner, requestDetails); err != nil {
			return err
		}
		return deleteOwnerData(database, logsDir, toLoggingConfig(cfg.Server.Logging), owner, requestID)
	default:
		return fmt.Errorf("unknown data action %q; use export or delete", args[0])
	}
}

func parseDataMaintenanceArgs(args []string) (owner, output string, yes bool, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--owner":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", "", false, errors.New("--owner requires a token prefix")
			}
			i++
			owner = args[i]
		case "--output":
			if i+1 >= len(args) {
				return "", "", false, errors.New("--output requires a file path or -")
			}
			i++
			output = args[i]
		case "--yes":
			yes = true
		default:
			return "", "", false, fmt.Errorf("unknown data option %q", args[i])
		}
	}
	return owner, output, yes, nil
}

func resolveOwnerByPrefix(database db.Handle, prefix string) (string, error) {
	if len(prefix) != 12 || !strings.HasPrefix(prefix, "own_") {
		return "", errors.New("owner token prefix must be the 12-character prefix shown by --maintenance token list")
	}
	var owner string
	err := database.QueryRow(`SELECT owner_id FROM api_tokens WHERE token_prefix = ?`, prefix).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("owner token prefix was not found")
	}
	if err != nil {
		return "", err
	}
	return owner, nil
}

func exportOwnerData(database db.Handle, logsDir string, logConfig logging.LogManagerConfig, owner, output string) error {
	requestID, err := newComplianceRequestID()
	if err != nil {
		return err
	}
	if err := recordComplianceEvent(database, "compliance.data_export_requested", owner, "request_id="+requestID); err != nil {
		return err
	}
	if err := writeDataAudit(logsDir, logConfig, "compliance.data_export_requested", owner, "request_id="+requestID); err != nil {
		return err
	}
	var createdAt, lastSeen int64
	if err := database.QueryRow(`SELECT created_at, last_seen_at FROM owners WHERE id = ?`, owner).Scan(&createdAt, &lastSeen); err != nil {
		return err
	}
	items, err := server.ListOwnerTokens(database)
	if err != nil {
		return err
	}
	result := ownerExport{
		Format:    1,
		Owner:     ownerExportIdentity{ID: owner, CreatedAt: time.Unix(createdAt, 0).UTC().Format(time.RFC3339), LastSeenAt: time.Unix(lastSeen, 0).UTC().Format(time.RFC3339)},
		Tokens:    make([]ownerExportToken, 0),
		Resources: []json.RawMessage{},
	}
	for _, item := range items {
		if item.OwnerID != owner {
			continue
		}
		token := ownerExportToken{Prefix: item.Prefix, Kind: item.Kind, Name: item.Name, CreatedAt: time.Unix(item.CreatedAt, 0).UTC().Format(time.RFC3339), Status: item.Status}
		if item.LastUsed != nil {
			token.LastUsed = time.Unix(*item.LastUsed, 0).UTC().Format(time.RFC3339)
		}
		var expires sql.NullInt64
		if err := database.QueryRow(`SELECT expires_at FROM api_tokens WHERE token_prefix = ?`, item.Prefix).Scan(&expires); err != nil {
			return err
		}
		if expires.Valid {
			token.Expires = time.Unix(expires.Int64, 0).UTC().Format(time.RFC3339)
		}
		result.Tokens = append(result.Tokens, token)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if output == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
	} else if err := writeOwnerExport(output, data); err != nil {
		return err
	}
	if err := recordComplianceEvent(database, "compliance.data_export_completed", owner, fmt.Sprintf("request_id=%s file_size=%d", requestID, len(data))); err != nil {
		return err
	}
	return writeDataAudit(logsDir, logConfig, "compliance.data_export_completed", owner, fmt.Sprintf("request_id=%s file_size=%d", requestID, len(data)))
}

func writeOwnerExport(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func confirmOwnerDeletion(owner string) error {
	fmt.Fprintf(os.Stderr, "Type owner id %s to permanently delete its data: ", owner)
	input := bufio.NewScanner(os.Stdin)
	if !input.Scan() {
		return errors.New("owner deletion cancelled")
	}
	if input.Text() != owner {
		return errors.New("owner id did not match; deletion cancelled")
	}
	return nil
}

func deleteOwnerData(database db.Handle, logsDir string, logConfig logging.LogManagerConfig, owner, requestID string) error {
	var anonBytes [16]byte
	if _, err := rand.Read(anonBytes[:]); err != nil {
		return err
	}
	anonymizedID := hex.EncodeToString(anonBytes[:])
	var deletedCount int64
	if err := db.Transaction(context.Background(), database, func(tx *db.TransactionHandle) error {
		deletedResources, err := db.DeleteOwnerResources(tx, owner)
		if err != nil {
			return err
		}
		var tokenCount int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE owner_id = ?`, owner).Scan(&tokenCount); err != nil {
			return err
		}
		result, err := tx.Exec(`DELETE FROM api_tokens WHERE owner_id = ?`, owner)
		if err != nil {
			return err
		}
		deletedTokens, err := result.RowsAffected()
		if err != nil {
			return err
		}
		ownerResult, err := tx.Exec(`DELETE FROM owners WHERE id = ?`, owner)
		if err != nil {
			return err
		}
		deletedOwners, err := ownerResult.RowsAffected()
		if err != nil {
			return err
		}
		if deletedOwners != 1 {
			return errors.New("owner record disappeared during deletion")
		}
		deletedCount = deletedTokens + deletedOwners + deletedResources
		return recordComplianceEventTx(tx, "compliance.data_deletion_completed", anonymizedID, fmt.Sprintf("request_id=%s deleted_count=%d anonymized_count=0", requestID, deletedCount))
	}); err != nil {
		return err
	}
	fmt.Printf("Deleted %d items; anonymized 0 retained records.\n", deletedCount)
	return writeDataAudit(logsDir, logConfig, "compliance.data_deletion_completed", anonymizedID, fmt.Sprintf("request_id=%s deleted_count=%d anonymized_count=0", requestID, deletedCount))
}

func newComplianceRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func ensureComplianceEventSchema(database db.Handle) error {
	_, err := database.Exec(`CREATE TABLE IF NOT EXISTS compliance_events (id INTEGER PRIMARY KEY AUTOINCREMENT, event TEXT NOT NULL, owner_ref TEXT NOT NULL DEFAULT '', details TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')))`)
	return err
}

func recordComplianceEvent(database db.Handle, event, owner, details string) error {
	_, err := database.Exec(`INSERT INTO compliance_events(event, owner_ref, details) VALUES(?,?,?)`, event, owner, details)
	return err
}

func recordComplianceEventTx(tx interface {
	Exec(string, ...any) (sql.Result, error)
}, event, owner, details string,
) error {
	_, err := tx.Exec(`INSERT INTO compliance_events(event, owner_ref, details) VALUES(?,?,?)`, event, owner, details)
	return err
}

func complianceCounts(database db.Handle, backupDir string, now time.Time) (owners, resources, activeTokens, openReports, recentEvents int64, lastBackup string, err error) {
	if err = ensureComplianceEventSchema(database); err != nil {
		return
	}
	items, listErr := server.ListOwnerTokens(database)
	if listErr != nil {
		err = listErr
		return
	}
	for _, item := range items {
		if item.Status == "active" {
			activeTokens++
		}
	}
	if err = database.QueryRow(`SELECT COUNT(*) FROM owners`).Scan(&owners); err != nil {
		return
	}
	tables, queryErr := database.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('owners','api_tokens')`)
	if queryErr != nil {
		err = queryErr
		return
	}
	var resourceTables []string
	for tables.Next() {
		var name string
		if scanErr := tables.Scan(&name); scanErr != nil {
			_ = tables.Close()
			err = scanErr
			return
		}
		columns, columnErr := database.Query(`PRAGMA table_info("` + strings.ReplaceAll(name, `"`, `""`) + `")`)
		if columnErr != nil {
			_ = tables.Close()
			err = columnErr
			return
		}
		ownerColumn := false
		for columns.Next() {
			var cid, notNull, pk int
			var column, kind string
			var defaultValue any
			if scanErr := columns.Scan(&cid, &column, &kind, &notNull, &defaultValue, &pk); scanErr != nil {
				_ = columns.Close()
				_ = tables.Close()
				err = scanErr
				return
			}
			if column == "owner_id" {
				ownerColumn = true
			}
		}
		if columnErr = columns.Err(); columnErr != nil {
			_ = columns.Close()
			_ = tables.Close()
			err = columnErr
			return
		}
		_ = columns.Close()
		if ownerColumn {
			resourceTables = append(resourceTables, name)
		}
	}
	if err = tables.Err(); err != nil {
		_ = tables.Close()
		return
	}
	if err = tables.Close(); err != nil {
		return
	}
	for _, name := range resourceTables {
		var count int64
		if err = database.QueryRow(`SELECT COUNT(*) FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`).Scan(&count); err != nil {
			return
		}
		resources += count
	}
	var reportTable int
	if err = database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='security_reports'`).Scan(&reportTable); err != nil {
		return
	}
	if reportTable > 0 {
		if err = database.QueryRow(`SELECT COUNT(*) FROM security_reports WHERE status NOT IN ('disclosed','wont_fix')`).Scan(&openReports); err != nil {
			return
		}
	}
	if err = database.QueryRow(`SELECT COUNT(*) FROM compliance_events WHERE created_at >= ?`, now.Add(-30*24*time.Hour).Unix()).Scan(&recentEvents); err != nil {
		return
	}
	var newest time.Time
	err = filepath.WalkDir(backupDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return
	}
	if !newest.IsZero() {
		lastBackup = newest.UTC().Format(time.RFC3339)
	} else {
		lastBackup = "none"
	}
	return
}

func writeDataAudit(logsDir string, config logging.LogManagerConfig, event, owner, details string) error {
	line := fmt.Sprintf("event=%s owner_id=%s %s", event, strconv.Quote(owner), details)
	manager := logging.NewLogManager(logsDir, config)
	defer func() { _ = manager.Close() }()
	return manager.WriteAudit(logging.AuditCategoryServer, line)
}
