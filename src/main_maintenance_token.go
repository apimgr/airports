package main

import (
	"bufio"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	paths "github.com/apimgr/airports/src/path"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

func runMaintenanceToken(configDir, logsDir, rawArgs string) (resultErr error) {
	args := strings.Fields(rawArgs)
	if len(args) == 0 {
		return errors.New("usage: --maintenance token {list|revoke <prefix> [--yes]}")
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
			return errors.New("token maintenance denied: invalid operator token")
		}
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close token maintenance database: %w", err))
		}
	}()
	items, err := server.ListOwnerTokens(database)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: --maintenance token list")
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(writer, "PREFIX\tKIND\tOWNER\tNAME\tCREATED\tLAST USED\tSTATUS"); err != nil {
			return err
		}
		for _, item := range items {
			lastUsed := "never"
			if item.LastUsed != nil {
				lastUsed = time.Unix(*item.LastUsed, 0).UTC().Format(time.RFC3339)
			}
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", item.Prefix, item.Kind, item.OwnerID, item.Name, time.Unix(item.CreatedAt, 0).UTC().Format(time.RFC3339), lastUsed, item.Status); err != nil {
				return err
			}
		}
		return writer.Flush()
	case "revoke":
		if len(args) < 2 || len(args) > 3 || (len(args) == 3 && args[2] != "--yes") {
			return errors.New("usage: --maintenance token revoke <prefix> [--yes]")
		}
		prefix := args[1]
		var selected server.OwnerTokenSummary
		found := false
		for _, item := range items {
			if item.Prefix == prefix {
				selected = item
				found = true
				break
			}
		}
		if !found {
			return errors.New("owner token prefix was not found")
		}
		if selected.Kind == "primary" {
			tokenCount := 0
			for _, item := range items {
				if item.OwnerID == selected.OwnerID {
					tokenCount++
				}
			}
			resourceCount, err := server.CountOwnerResources(database, selected.OwnerID)
			if err != nil {
				return fmt.Errorf("count owner resources: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Revoke primary token for owner %s (%d tokens, %d resources).\n", selected.OwnerID, tokenCount, resourceCount)
			if len(args) == 2 {
				if err := confirmOwnerDeletion(selected.OwnerID); err != nil {
					return err
				}
			}
		} else if len(args) == 2 {
			if err := confirmTokenRevocation(prefix); err != nil {
				return err
			}
		}
		manager := logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
		defer func() {
			if err := manager.Close(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("close token maintenance log manager: %w", err))
			}
		}()
		if err := server.RevokeOwnerTokenByPrefixWithAudit(database, prefix, manager, "cli", maintenanceOperatorIP()); err != nil {
			return err
		}
		if selected.Kind == "primary" {
			fmt.Printf("Deleted owner %s and all associated tokens and resources.\n", selected.OwnerID)
		} else {
			fmt.Printf("Revoked token prefix %s for owner %s.\n", prefix, selected.OwnerID)
		}
		return nil
	default:
		return fmt.Errorf("unknown token action %q; use list or revoke", args[0])
	}
}

func confirmTokenRevocation(prefix string) error {
	fmt.Fprintf(os.Stderr, "Type token prefix %s to revoke it: ", prefix)
	input := bufio.NewScanner(os.Stdin)
	if !input.Scan() {
		return errors.New("token revocation cancelled")
	}
	if input.Text() != prefix {
		return errors.New("token prefix did not match; revocation cancelled")
	}
	return nil
}
