package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/apimgr/airports/src/backup"
	"github.com/apimgr/airports/src/config"
	dbmetrics "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/notify"
	"github.com/apimgr/airports/src/server"
)

const recoveryProbeTimeout = 5 * time.Second

type maintenanceRecovery struct {
	db         *sql.DB
	dataDir    string
	configDir  string
	logsDir    string
	backupDir  string
	logManager *logging.Manager
	manual     bool
	mu         sync.RWMutex
	cfg        *config.Config
	cancel     context.CancelFunc
	done       chan struct{}
}

func startMaintenanceRecovery(cfg *config.Config, dbHandle *sql.DB, dataDir, configDir, logsDir, backupDir string, logManager *logging.Manager, manual bool) *maintenanceRecovery {
	ctx, cancel := context.WithCancel(context.Background())
	recovery := &maintenanceRecovery{
		db:         dbHandle,
		dataDir:    dataDir,
		configDir:  configDir,
		logsDir:    logsDir,
		backupDir:  backupDir,
		logManager: logManager,
		manual:     manual,
		cfg:        cfg,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	go recovery.run(ctx)
	return recovery
}

func (r *maintenanceRecovery) stop() {
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
}

func (r *maintenanceRecovery) updateConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	r.mu.Lock()
	r.cfg = cfg
	r.mu.Unlock()
}

func (r *maintenanceRecovery) currentConfig() *config.Config {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

func (r *maintenanceRecovery) run(ctx context.Context) {
	defer close(r.done)
	var active *server.MaintenanceHealthInfo
	var attempts int
	var entered time.Time

	for {
		cfg := r.currentConfig()
		if cfg == nil {
			return
		}
		interval := maintenanceRetryInterval(cfg.Server.Maintenance.SelfHealing.RetryInterval)
		if active == nil {
			reason, probeErr := r.detectCriticalFailure(cfg)
			if reason != "" {
				if r.manual {
					if !waitForRecoveryTick(ctx, interval) {
						return
					}
					continue
				}
				entered = time.Now().UTC()
				active = &server.MaintenanceHealthInfo{
					Reason: reason,
					Since:  entered,
					SelfHealing: server.SelfHealingHealthStatus{
						Enabled: cfg.Server.Maintenance.SelfHealing.Enabled,
					},
				}
				server.SetHealthMaintenance(active)
				log.Printf("Critical service failure detected (%s): %v; entering read-only maintenance mode", reason, probeErr)
				logMaintenanceGuidance(reason)
				r.sendNotification(cfg, "maintenance_entered", cfg.Server.Maintenance.Notify.OnEnter, reason, time.Duration(0))
			}
		}

		if active != nil && cfg.Server.Maintenance.SelfHealing.Enabled {
			maxAttempts := cfg.Server.Maintenance.SelfHealing.MaxAttempts
			if maxAttempts == 0 || attempts < maxAttempts {
				now := time.Now().UTC()
				lastAttempt := now
				attempts++
				next := now.Add(interval)
				active.SelfHealing = server.SelfHealingHealthStatus{
					Enabled:     true,
					Attempts:    attempts,
					LastAttempt: &lastAttempt,
					NextAttempt: &next,
				}
				server.SetHealthMaintenance(active)
				if err := r.recoverOnce(ctx, cfg); err != nil {
					log.Printf("Self-healing attempt %d failed (%s): %v; maintenance retry in %s", attempts, active.Reason, err, interval)
					if !waitForRecoveryTick(ctx, interval) {
						return
					}
					continue
				}
				if reason, err := r.detectCriticalFailure(cfg); reason != "" {
					log.Printf("Self-healing attempt %d did not pass recovery checks (%s): %v", attempts, reason, err)
					if !waitForRecoveryTick(ctx, interval) {
						return
					}
					continue
				}
				log.Printf("Issue resolved, exiting maintenance mode")
				server.SetHealthMaintenance(nil)
				r.sendNotification(cfg, "maintenance_recovered", cfg.Server.Maintenance.Notify.OnExit, active.Reason, time.Since(entered))
				active = nil
				attempts = 0
			} else if maxAttempts > 0 && attempts >= maxAttempts {
				active.SelfHealing.NextAttempt = nil
				server.SetHealthMaintenance(active)
			}
		}

		if !waitForRecoveryTick(ctx, interval) {
			return
		}
	}
}

func logMaintenanceGuidance(reason string) {
	switch reason {
	case "database_connection":
		log.Printf("Maintenance guidance: verify the database file is readable and writable, check configured database credentials/connectivity, then inspect server logs")
	case "database_write":
		log.Printf("Maintenance guidance: check database write permissions and available disk space, then inspect server logs")
	case "disk_full":
		log.Printf("Maintenance guidance: automatic cleanup is pruning expired logs, project temp files older than 24 hours, and backups beyond retention; inspect filesystem usage if space remains low")
	case "file_permission":
		log.Printf("Maintenance guidance: check directory ownership and write permissions, SELinux/AppArmor policy, and whether the filesystem is mounted read-only")
	}
}

func maintenanceRetryInterval(value string) time.Duration {
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return 30 * time.Second
	}
	return interval
}

func waitForRecoveryTick(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *maintenanceRecovery) detectCriticalFailure(cfg *config.Config) (string, error) {
	if err := probeDatabaseConnection(r.db); err != nil {
		return "database_connection", err
	}
	if err := probeDatabaseReadWrite(r.db); err != nil {
		return "database_write", err
	}
	total, free, err := backup.DiskUsage(r.dataDir)
	if err != nil || total == 0 {
		if err == nil {
			err = errors.New("disk usage provider returned no total capacity")
		}
		return "file_permission", err
	}
	threshold := cfg.Server.Maintenance.Cleanup.DiskThreshold
	if threshold <= 0 || threshold > 100 {
		threshold = 90
	}
	usedPercent := 100 * (1 - float64(free)/float64(total))
	if usedPercent >= float64(threshold) {
		return "disk_full", fmt.Errorf("disk usage is %.1f%%", usedPercent)
	}
	for _, dir := range []string{r.configDir, r.dataDir, r.logsDir} {
		if err := probeWritableDirectory(dir); err != nil {
			return "file_permission", err
		}
	}
	return "", nil
}

func probeDatabaseReadWrite(db *sql.DB) error {
	if db == nil {
		return errors.New("database handle is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), recoveryProbeTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	query := `UPDATE scheduler_tasks SET enabled = enabled WHERE task_id = (SELECT task_id FROM scheduler_tasks LIMIT 1)`
	started := time.Now()
	result, err := tx.ExecContext(ctx, query)
	rows := int64(0)
	if result != nil {
		if affected, resultErr := result.RowsAffected(); resultErr == nil {
			rows = affected
		}
	}
	dbmetrics.LogQuery(query, 0, started, rows, err)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}
	if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return rollbackErr
	}
	return nil
}

func probeDatabaseConnection(db *sql.DB) error {
	if db == nil {
		return errors.New("database handle is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), recoveryProbeTimeout)
	defer cancel()
	return db.PingContext(ctx)
}

func probeWritableDirectory(dir string) error {
	if dir == "" {
		return errors.New("required directory is not configured")
	}
	file, err := os.CreateTemp(dir, ".airports-recovery-probe-")
	if err != nil {
		return fmt.Errorf("directory write probe failed for %s: %w", dir, err)
	}
	name := file.Name()
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(name)
		return fmt.Errorf("directory write probe sync failed for %s: %w", dir, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("directory write probe close failed for %s: %w", dir, err)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("directory write probe cleanup failed for %s: %w", dir, err)
	}
	return nil
}

func (r *maintenanceRecovery) recoverOnce(ctx context.Context, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var cleanupErr error
	if cfg.Server.Maintenance.Cleanup.DiskThreshold > 0 {
		_, cleanupErr = runDiskPressureCleanup(
			cfg.Server.Maintenance.Cleanup,
			r.dataDir,
			r.backupDir,
			r.logManager,
			time.Now(),
			backup.DiskUsage,
		)
	}
	if err := r.recoverDatabase(ctx); err != nil {
		return errors.Join(cleanupErr, err)
	}
	if _, _, err := backup.DiskUsage(r.dataDir); err != nil {
		return errors.Join(cleanupErr, err)
	}
	for _, dir := range []string{r.configDir, r.dataDir, r.logsDir} {
		if err := probeWritableDirectory(dir); err != nil {
			return errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (r *maintenanceRecovery) recoverDatabase(ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := probeDatabaseConnection(r.db); err != nil {
			lastErr = err
		} else if err := probeDatabaseReadWrite(r.db); err != nil {
			lastErr = err
		} else {
			return nil
		}
		if attempt < 2 && !waitForRecoveryTick(ctx, time.Second<<attempt) {
			return ctx.Err()
		}
	}
	return lastErr
}

func (r *maintenanceRecovery) sendNotification(cfg *config.Config, name string, enabled bool, reason string, duration time.Duration) {
	if !enabled || !notify.CanSend(cfg) {
		return
	}
	vars := map[string]string{
		"reason":   reason,
		"duration": duration.Round(time.Second).String(),
	}
	if err := notify.Send(cfg, r.configDir, name, notifyRecipient(cfg), vars); err != nil {
		log.Printf("Warning: failed to send %s notification: %v", name, err)
	}
}
