package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/security/pgp"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
)

func closeTestResource(t *testing.T, name string, closeResource func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closeResource(); err != nil {
			t.Errorf("close %s: %v", name, err)
		}
	})
}

func TestFinalizeBackupPairRestoresPreviousPairWhenIncrementalRenameFails(t *testing.T) {
	backupDir := newMainTestDir(t)
	fullPath := filepath.Join(backupDir, "airports_backup_previous.tar.gz")
	incPath := filepath.Join(backupDir, "airports-daily.tar.gz")
	fullTemp := filepath.Join(backupDir, "full.pending.tar.gz")
	incTemp := filepath.Join(backupDir, "incremental.pending.tar.gz")
	files := map[string]string{
		fullPath: "previous full",
		incPath:  "previous incremental",
		fullTemp: "new full",
		incTemp:  "new incremental",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	rename := func(oldPath, newPath string) error {
		if oldPath == incTemp && newPath == incPath {
			return errors.New("injected incremental rename failure")
		}
		return os.Rename(oldPath, newPath)
	}

	err := finalizeBackupPairWithRename(fullTemp, fullPath, incTemp, incPath, backupDir, rename)
	if err == nil {
		t.Fatal("finalizeBackupPairWithRename() succeeded after the incremental rename failed")
	}
	for path, want := range map[string]string{fullPath: "previous full", incPath: "previous incremental"} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("restored %s = %q, want %q", path, got, want)
		}
	}
}

func TestParseDataMaintenanceArgs(t *testing.T) {
	tests := []struct {
		args    []string
		owner   string
		output  string
		yes     bool
		wantErr bool
	}{
		{args: []string{"--owner", "own_12345678", "--output", "-", "--yes"}, owner: "own_12345678", output: "-", yes: true},
		{args: []string{"--owner"}, wantErr: true},
		{args: []string{"--output"}, wantErr: true},
		{args: []string{"--unknown"}, wantErr: true},
	}
	for _, test := range tests {
		owner, output, yes, err := parseDataMaintenanceArgs(test.args)
		if (err != nil) != test.wantErr {
			t.Errorf("parseDataMaintenanceArgs(%v) error = %v", test.args, err)
			continue
		}
		if err == nil && (owner != test.owner || output != test.output || yes != test.yes) {
			t.Errorf("parseDataMaintenanceArgs(%v) = %q, %q, %t", test.args, owner, output, yes)
		}
	}
}

func TestMaintenanceDispatchersRejectMissingOrUnsupportedActions(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "PGP missing action", run: func() error { return runMaintenancePGP("", "", "") }},
		{name: "secret unsupported action", run: func() error { return runMaintenanceSecret("", "", "unsupported rotate") }},
		{name: "data missing action", run: func() error { return runMaintenanceData("", "", "") }},
		{name: "security missing action", run: func() error { return runMaintenanceSecurity("", "") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("maintenance dispatcher accepted an invalid action")
			}
		})
	}
}

func TestConfirmOwnerDeletionRequiresExactOwnerID(t *testing.T) {
	original := os.Stdin
	t.Cleanup(func() { os.Stdin = original })
	for _, test := range []struct {
		name  string
		input string
		want  bool
	}{
		{name: "matching id", input: "owner-123\n"},
		{name: "mismatched id", input: "owner-456\n", want: true},
		{name: "end of input", input: "", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writer.WriteString(test.input); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			os.Stdin = reader
			err = confirmOwnerDeletion("owner-123")
			_ = reader.Close()
			if (err != nil) != test.want {
				t.Fatalf("confirmOwnerDeletion() error = %v, wantError %t", err, test.want)
			}
		})
	}
}

func TestSecretEncryptionKeyHelpers(t *testing.T) {
	current := bytes.Repeat([]byte{0x42}, 32)
	previous := bytes.Repeat([]byte{0x24}, 32)
	plain := []byte("stored secret payload")
	wrapped, err := wrapPreviousEncryptionKey(current, previous)
	if err != nil {
		t.Fatal(err)
	}
	unwrapped, err := unwrapPreviousEncryptionKey(current, wrapped)
	if err != nil || !bytes.Equal(unwrapped, previous) {
		t.Fatalf("unwrapPreviousEncryptionKey() = %x, %v", unwrapped, err)
	}
	if _, err := unwrapPreviousEncryptionKey(previous, wrapped); err == nil {
		t.Fatal("wrong wrapping key was accepted")
	}
	ciphertext, err := encryptAESGCM(previous, plain)
	if err != nil {
		t.Fatal(err)
	}
	decoded, usedPrevious, err := decryptWithKeyFallback(current, previous, ciphertext, false)
	if err != nil || !usedPrevious || !bytes.Equal(decoded, plain) {
		t.Fatalf("previous-key fallback = %q, %t, %v", decoded, usedPrevious, err)
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(ciphertext))
	decoded, usedPrevious, err = decryptWithKeyFallback(current, previous, encoded, true)
	if err != nil || !usedPrevious || !bytes.Equal(decoded, plain) {
		t.Fatalf("encoded previous-key fallback = %q, %t, %v", decoded, usedPrevious, err)
	}
	if _, _, err := decryptWithKeyFallback(current, previous, []byte("not base64"), true); err == nil {
		t.Fatal("invalid encoded ciphertext was accepted")
	}
	if !equalBytes(current, append([]byte(nil), current...)) || equalBytes(current, previous) {
		t.Fatal("constant-time equality helper returned an incorrect result")
	}
}

func TestRunMaintenanceSecretRotateEncryptionKey(t *testing.T) {
	if !service.IsElevated() {
		t.Skip("secret rotation test requires administrator authorization")
	}
	root := newMainTestDir(t)
	configDir := filepath.Join(root, "config")
	logsDir := filepath.Join(root, "logs")
	databasePath := filepath.Join(root, "server.db")
	oldKey := bytes.Repeat([]byte{0x37}, 32)
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Database.URL = databasePath
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(oldKey)
	cfg.Server.Security.EncryptionKeyVersion = 1
	configPath := filepath.Join(configDir, "server.yml")
	if err := config.SaveApplicationConfig(configPath, cfg); err != nil {
		t.Fatalf("save test configuration: %v", err)
	}
	database := openRecoveryTestDB(t, databasePath, false)
	if _, err := server.ListOwnerTokens(database); err != nil {
		t.Fatalf("initialize token schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	confirm := func(value string) {
		t.Helper()
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.WriteString(value + "\n"); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		original := os.Stdin
		os.Stdin = reader
		t.Cleanup(func() {
			os.Stdin = original
			_ = reader.Close()
		})
	}
	confirm("ROTATE ENCRYPTION_KEY")
	if err := runMaintenanceSecret(configDir, logsDir, "rotate encryption_key"); err != nil {
		t.Fatalf("rotate encryption key: %v", err)
	}
	rotated, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		t.Fatalf("load rotated configuration: %v", err)
	}
	newKey, err := base64.StdEncoding.DecodeString(rotated.Server.Security.EncryptionKey)
	if err != nil || len(newKey) != 32 || bytes.Equal(newKey, oldKey) || rotated.Server.Security.EncryptionKeyVersion != 2 {
		t.Fatalf("rotated key metadata invalid: version=%d key_length=%d err=%v", rotated.Server.Security.EncryptionKeyVersion, len(newKey), err)
	}
	database = openRecoveryTestDB(t, databasePath, false)
	closeTestResource(t, "database", database.Close)
	var wrapped []byte
	if err := database.QueryRow(`SELECT value FROM app_secrets WHERE name = 'encryption_key_previous'`).Scan(&wrapped); err != nil {
		t.Fatal(err)
	}
	wrapped = db.DecodeAppSecret("encryption_key_previous", wrapped)
	previous, err := unwrapPreviousEncryptionKey(newKey, wrapped)
	if err != nil || !bytes.Equal(previous, oldKey) {
		t.Fatalf("wrapped previous key does not match original: %v", err)
	}

	confirm("ROTATE ENCRYPTION_KEY")
	if err := runMaintenanceSecret(configDir, logsDir, "rotate encryption_key"); err == nil || !strings.Contains(err.Error(), "grace period") {
		t.Fatalf("second rotation during grace period error = %v", err)
	}
}

func TestMaintenancePrivateWriters(t *testing.T) {
	root := newMainTestDir(t)
	secretPath := filepath.Join(root, "security", "pgp.priv.asc.enc")
	if err := atomicPGPWrite(secretPath, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(secretPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private file mode = %v, err=%v", info, err)
	}
	if err := writeExclusivePrivateFile(filepath.Join(root, "exclusive"), []byte("once")); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusivePrivateFile(filepath.Join(root, "exclusive"), []byte("twice")); err == nil {
		t.Fatal("exclusive writer replaced an existing file")
	}
	if err := writeOwnerExport(filepath.Join(root, "export", "owner.json"), []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeOwnerExport(filepath.Join(root, "export", "owner.json"), []byte("{}\n")); err == nil {
		t.Fatal("owner export replaced an existing file")
	}
}

func TestPGPKeyserverEndpointValidation(t *testing.T) {
	valid := map[string]string{
		"keys.openpgp.org":            "https://keys.openpgp.org/vks/v1/upload",
		"https://keys.example/upload": "https://keys.example/upload",
	}
	for input, want := range valid {
		got, err := pgpKeyserverEndpoint(input)
		if err != nil || got != want {
			t.Errorf("pgpKeyserverEndpoint(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"http://keys.example", "https://user:pass@keys.example", "https://keys.example/?token=x", "https://keys.example/#part", "not a url"} {
		if _, err := pgpKeyserverEndpoint(input); err == nil {
			t.Errorf("pgpKeyserverEndpoint(%q) accepted an invalid target", input)
		}
	}
}

func TestPGPMetadataPersistence(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "metadata.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	want := pgpMetadata{
		Fingerprint:         "0123456789ABCDEF",
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:           time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		KeyserversPublished: map[string]string{"keys.openpgp.org": "published"},
		KeyserverRetries:    map[string]pgpRetryState{"keys.example": {Attempts: 2, NextAttempt: time.Now().UTC().Format(time.RFC3339), LastError: "unavailable"}},
		Revoked:             true,
		PrivateExportedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := savePGPMetadata(database, want); err != nil {
		t.Fatal(err)
	}
	got, err := readPGPMetadata(database)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != want.Fingerprint || got.CreatedAt != want.CreatedAt || got.ExpiresAt != want.ExpiresAt || !got.Revoked || got.KeyserversPublished["keys.openpgp.org"] != "published" || got.KeyserverRetries["keys.example"].Attempts != 2 {
		t.Fatalf("metadata round-trip = %+v", got)
	}
}

func TestGeneratePGPKeyPersistsEncryptedKeypairAndMetadata(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "pgp.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Branding.Title = "Airports"
	cfg.Server.Contact.Security.Email = "security@example.org"
	keyDir := filepath.Join(root, "security")
	privatePath := filepath.Join(keyDir, "pgp.priv.asc.enc")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	secret := bytes.Repeat([]byte{0x5a}, 32)
	if err := generatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, logs); err != nil {
		t.Fatal(err)
	}
	privateCiphertext, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := pgp.DecryptPrivate(secret, privateCiphertext)
	if err != nil || len(privateKey) == 0 {
		t.Fatalf("decrypt generated private key: %v", err)
	}
	publicKey, err := os.ReadFile(publicPath)
	if err != nil || len(publicKey) == 0 {
		t.Fatalf("read generated public key: %v", err)
	}
	metadata, err := readPGPMetadata(database)
	if err != nil || len(metadata.Fingerprint) != 40 || metadata.ExpiresAt == "" {
		t.Fatalf("generated key metadata = %+v, err=%v", metadata, err)
	}
	if _, err := os.Stat(publicPath); err != nil {
		t.Fatalf("generated PGP public key is not available for publication: %v", err)
	}
	if err := generatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, logs); err == nil {
		t.Fatal("generation replaced an existing private key")
	}
}

func TestRotatePGPKeyReencryptsReportsAndKeepsRecoveryKey(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "pgp-rotation.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE security_reports (tracking_id TEXT PRIMARY KEY, body_enc BLOB NOT NULL, body_alg TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(root, "security")
	privatePath := filepath.Join(keyDir, "pgp.priv.asc.enc")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	secret := bytes.Repeat([]byte{0x53}, 32)
	oldKey, err := pgp.Generate("Airports Security", "security@example.org", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := pgp.WritePrivate(privatePath, secret, oldKey.Private); err != nil {
		t.Fatal(err)
	}
	if err := atomicPGPWrite(publicPath, oldKey.Public, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := savePGPMetadata(database, pgpMetadata{Fingerprint: oldKey.Fingerprint, KeyserversPublished: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	const reportBody = "A retained security report"
	ciphertext, err := pgp.EncryptTo(oldKey.Public, []byte(reportBody))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO security_reports(tracking_id, body_enc, body_alg) VALUES('sec_rotation', ?, 'pgp')`, ciphertext); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Branding.Title = "Airports"
	cfg.Server.Contact.Security.Email = "security@example.org"
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	if err := rotatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, logs); err != nil {
		t.Fatalf("rotate PGP key: %v", err)
	}
	newPrivate, err := pgp.ReadPrivate(privatePath, secret)
	if err != nil {
		t.Fatal(err)
	}
	newEntity, err := pgp.ParsePrivate(newPrivate)
	if err != nil {
		t.Fatal(err)
	}
	var rotatedBody []byte
	if err := database.QueryRow(`SELECT body_enc FROM security_reports WHERE tracking_id = 'sec_rotation'`).Scan(&rotatedBody); err != nil {
		t.Fatal(err)
	}
	message, err := openpgp.ReadMessage(bytes.NewReader(rotatedBody), openpgp.EntityList{newEntity}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	decryptedBody, err := io.ReadAll(message.UnverifiedBody)
	if err != nil || string(decryptedBody) != reportBody {
		t.Fatalf("rotated report body = %q, err=%v", decryptedBody, err)
	}
	if _, err := os.Stat(filepath.Join(keyDir, "pgp.priv.old.asc.enc")); err != nil {
		t.Fatalf("retired recovery key was not retained: %v", err)
	}
	metadata, err := readPGPMetadata(database)
	if err != nil || metadata.Fingerprint == oldKey.Fingerprint || metadata.LastRotatedAt == "" {
		t.Fatalf("rotated metadata = %+v, err=%v", metadata, err)
	}
	var journalCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM pgp_rotation_journal`).Scan(&journalCount); err != nil || journalCount != 0 {
		t.Fatalf("rotation journal entries = %d, err=%v", journalCount, err)
	}
}

func TestPGPPrivateExportRateLimitAndDelete(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "pgp-export.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(root, "security")
	privatePath := filepath.Join(keyDir, "pgp.priv.asc.enc")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	secret := bytes.Repeat([]byte{0x62}, 32)
	key, err := pgp.Generate("Airports Security", "security@example.org", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := pgp.WritePrivate(privatePath, secret, key.Private); err != nil {
		t.Fatal(err)
	}
	if err := atomicPGPWrite(publicPath, key.Public, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := savePGPMetadata(database, pgpMetadata{Fingerprint: key.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	if err := os.MkdirAll(filepath.Join(root, "export"), 0o700); err != nil {
		t.Fatal(err)
	}
	exportedPath := filepath.Join(root, "export", "pgp.priv.asc")
	if err := exportPrivatePGPKey(database, secret, privatePath, exportedPath, logs, "scheduled recovery check"); err != nil {
		t.Fatalf("export private key: %v", err)
	}
	info, err := os.Stat(exportedPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("exported private key mode = %v, err=%v", info, err)
	}
	exported, err := os.ReadFile(exportedPath)
	if err != nil || !bytes.Equal(exported, key.Private) {
		t.Fatalf("exported private key mismatch, err=%v", err)
	}
	if err := exportPrivatePGPKey(database, secret, privatePath, filepath.Join(root, "second-export"), logs, "repeat export"); err == nil || !strings.Contains(err.Error(), "once per hour") {
		t.Fatalf("second private export error = %v", err)
	}
	for _, name := range []string{"pgp.priv.old.asc.enc", "pgp.pub.asc.sig", "keyservers.state"} {
		if err := atomicPGPWrite(filepath.Join(keyDir, name), []byte("recovery data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := deletePGPKey(database, keyDir, logs); err != nil {
		t.Fatalf("delete PGP key: %v", err)
	}
	metadata, err := readPGPMetadata(database)
	if err != nil || !metadata.Revoked || metadata.Fingerprint != key.Fingerprint {
		t.Fatalf("revoked key metadata = %+v, err=%v", metadata, err)
	}
	for _, name := range []string{"pgp.priv.asc.enc", "pgp.priv.old.asc.enc", "pgp.pub.asc", "pgp.pub.asc.sig", "keyservers.state"} {
		if _, err := os.Lstat(filepath.Join(keyDir, name)); !os.IsNotExist(err) {
			t.Errorf("deleted key file %s still exists (err=%v)", name, err)
		}
	}
}

func TestImportPGPPrivateKeyPersistsIdentityAndPublicKey(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "pgp-import.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	key, err := pgp.Generate("Airports Security", "security@example.org", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "import.asc")
	if err := os.WriteFile(inputPath, key.Private, 0o600); err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(root, "security")
	privatePath := filepath.Join(keyDir, "pgp.priv.asc.enc")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Branding.Title = "Airports"
	cfg.Server.Contact.Security.Email = "security@example.org"
	secret := bytes.Repeat([]byte{0x19}, 32)
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	if err := importPrivatePGPKey(database, cfg, keyDir, privatePath, publicPath, secret, inputPath, logs); err != nil {
		t.Fatalf("import private key: %v", err)
	}
	storedPrivate, err := pgp.ReadPrivate(privatePath, secret)
	if err != nil || !bytes.Equal(storedPrivate, key.Private) {
		t.Fatalf("imported private key mismatch, err=%v", err)
	}
	storedPublic, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := pgp.Fingerprint(storedPublic)
	if err != nil || fingerprint != key.Fingerprint {
		t.Fatalf("imported public fingerprint = %q, err=%v", fingerprint, err)
	}
	metadata, err := readPGPMetadata(database)
	if err != nil || metadata.Fingerprint != key.Fingerprint {
		t.Fatalf("imported metadata = %+v, err=%v", metadata, err)
	}
	if _, err := os.Stat(publicPath); err != nil {
		t.Fatalf("imported PGP public key is not available for publication: %v", err)
	}
}

func TestPGPKeyserverFailureIsRecordedAndRemovedTargetCleared(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "pgp-publish.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(root, "security")
	publicPath := filepath.Join(keyDir, "pgp.pub.asc")
	if err := atomicPGPWrite(publicPath, []byte("public key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := savePGPMetadata(database, pgpMetadata{Fingerprint: "ABCDEF", KeyserversPublished: map[string]string{}, KeyserverRetries: map[string]pgpRetryState{}}); err != nil {
		t.Fatal(err)
	}
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	badTarget := "http://keys.example.org"
	if err := publishPGPKey(database, keyDir, publicPath, []string{badTarget}, logs); err == nil {
		t.Fatal("publishing to a non-HTTPS keyserver did not fail")
	}
	metadata, err := readPGPMetadata(database)
	if err != nil || metadata.KeyserverRetries[badTarget].Attempts != 3 || metadata.KeyserverRetries[badTarget].LastError == "" {
		t.Fatalf("failed publish retry metadata = %+v, err=%v", metadata.KeyserverRetries[badTarget], err)
	}
	if err := retryPendingPGPKeyservers(database, root, nil, logs, time.Now().UTC()); err != nil {
		t.Fatalf("clear retry for removed keyserver: %v", err)
	}
	metadata, err = readPGPMetadata(database)
	if err != nil || len(metadata.KeyserverRetries) != 0 {
		t.Fatalf("removed keyserver retry state = %+v, err=%v", metadata.KeyserverRetries, err)
	}
	if _, err := os.Stat(filepath.Join(keyDir, "keyservers.state")); err != nil {
		t.Fatalf("keyserver state file was not persisted: %v", err)
	}
}

func TestRunMaintenancePGPRejectsUnknownActionAfterAuthorization(t *testing.T) {
	if !service.IsElevated() {
		t.Skip("maintenance command test requires administrator authorization")
	}
	root := newMainTestDir(t)
	configDir := filepath.Join(root, "config")
	logsDir := filepath.Join(root, "logs")
	databasePath := filepath.Join(root, "server.db")
	database := openRecoveryTestDB(t, databasePath, false)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Database.URL = databasePath
	if err := config.SaveApplicationConfig(filepath.Join(configDir, "server.yml"), cfg); err != nil {
		t.Fatal(err)
	}
	if err := runMaintenancePGP(configDir, logsDir, "unsupported"); err == nil || !strings.Contains(err.Error(), "unknown PGP action") {
		t.Fatalf("unknown PGP action error = %v", err)
	}
}

func TestResumeInstallationSecretRotationReencryptsPrivateKey(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "secret-recovery.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensureAppSecretsSchema(database); err != nil {
		t.Fatal(err)
	}
	current := bytes.Repeat([]byte{0x31}, 32)
	previous := bytes.Repeat([]byte{0x73}, 32)
	for name, value := range map[string][]byte{"installation_secret": current, "installation_secret_previous": previous} {
		if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, name, value); err != nil {
			t.Fatal(err)
		}
	}
	configDir := filepath.Join(root, "config")
	privatePath := filepath.Join(configDir, "security", "pgp.priv.asc.enc")
	const privateArmor = "recovery-test-private-material"
	if err := pgp.WritePrivate(privatePath, previous, []byte(privateArmor)); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x44}, 32))
	if err := resumeSecretRotations(database, cfg, configDir); err != nil {
		t.Fatalf("resume secret rotation: %v", err)
	}
	recovered, err := pgp.ReadPrivate(privatePath, current)
	if err != nil || string(recovered) != privateArmor {
		t.Fatalf("recovered PGP private key = %q, err=%v", recovered, err)
	}
	if _, err := pgp.ReadPrivate(privatePath, previous); err == nil {
		t.Fatal("recovered PGP private key still uses the previous installation secret")
	}
}

func TestResumeSecretRotationClearsUncommittedEncryptionKeyJournal(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "uncommitted-rotation.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensureAppSecretsSchema(database); err != nil {
		t.Fatal(err)
	}
	installationSecret := bytes.Repeat([]byte{0x29}, 32)
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES('installation_secret', ?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, installationSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES('encryption_key_previous', ?)`, []byte("invalid previous-key envelope")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES('encryption_key_previous_until', ?)`, []byte(time.Now().UTC().Add(30*24*time.Hour).Format(time.RFC3339))); err != nil {
		t.Fatal(err)
	}
	oldKey := bytes.Repeat([]byte{0x58}, 32)
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(oldKey)
	if err := resumeSecretRotations(database, cfg, root); err != nil {
		t.Fatalf("resume uncommitted rotation: %v", err)
	}
	var remaining int
	if err := database.QueryRow(`SELECT COUNT(*) FROM app_secrets WHERE name IN ('encryption_key_previous', 'encryption_key_previous_until')`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("uncommitted rotation left %d previous-key journal rows", remaining)
	}
}

func TestPreviousSecretGracePeriodAndCleanup(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "secrets.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensureAppSecretsSchema(database); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?)`, "token_previous_until", []byte(now.Add(time.Hour).Format(time.RFC3339))); err != nil {
		t.Fatal(err)
	}
	if err := refuseActivePreviousSecret(database, "token", now); err == nil {
		t.Fatal("active grace period did not block another rotation")
	}
	if err := refuseActivePreviousSecret(database, "absent", now); err != nil {
		t.Fatalf("missing previous secret: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES(?, ?)`, "expired_previous_until", []byte(now.Add(-time.Hour).Format(time.RFC3339))); err != nil {
		t.Fatal(err)
	}
	if err := cleanupPreviousSecrets(database, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM app_secrets WHERE name = 'expired_previous_until'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired secret marker count = %d, err=%v", count, err)
	}
}

func TestSecretRotationsPreserveRecoveryMaterial(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "rotation.db"), false)
	closeTestResource(t, "database", database.Close)
	if err := ensureAppSecretsSchema(database); err != nil {
		t.Fatal(err)
	}
	logs := logging.NewLogManager(filepath.Join(root, "logs"), logging.LogManagerConfig{})
	closeTestResource(t, "log manager", logs.Close)
	oldInstallation, err := loadInstallationSecret(database)
	if err != nil {
		t.Fatal(err)
	}
	if err := rotateInstallationSecret(database, root, "installation_secret", logs); err != nil {
		t.Fatalf("rotate installation secret: %v", err)
	}
	newInstallation, err := loadInstallationSecret(database)
	if err != nil || bytes.Equal(oldInstallation, newInstallation) {
		t.Fatalf("installation secret did not rotate: equal=%t err=%v", bytes.Equal(oldInstallation, newInstallation), err)
	}
	var previous []byte
	if err := database.QueryRow(`SELECT value FROM app_secrets WHERE name='installation_secret_previous'`).Scan(&previous); err != nil || !bytes.Equal(db.DecodeAppSecret("installation_secret_previous", previous), oldInstallation) {
		t.Fatalf("previous installation secret = %x, err=%v", previous, err)
	}

	oldKey := bytes.Repeat([]byte{0x37}, 32)
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(oldKey)
	configPath := filepath.Join(root, "server.yml")
	if err := config.SaveApplicationConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadApplicationConfig(configPath)
	if err != nil {
		t.Fatalf("load encryption-key config: %v", err)
	}
	if err := rotateEncryptionKey(database, cfg, configPath, root, logs); err != nil {
		t.Fatalf("rotate encryption key: %v", err)
	}
	newKey, err := base64.StdEncoding.DecodeString(cfg.Server.Security.EncryptionKey)
	if err != nil || bytes.Equal(oldKey, newKey) || cfg.Server.Security.EncryptionKeyVersion != 2 {
		t.Fatalf("encryption key was not rotated: version=%d err=%v", cfg.Server.Security.EncryptionKeyVersion, err)
	}
	var wrapped []byte
	if err := database.QueryRow(`SELECT value FROM app_secrets WHERE name='encryption_key_previous'`).Scan(&wrapped); err != nil {
		t.Fatal(err)
	}
	wrapped = db.DecodeAppSecret("encryption_key_previous", wrapped)
	unwrapped, err := unwrapPreviousEncryptionKey(newKey, wrapped)
	if err != nil || !bytes.Equal(unwrapped, oldKey) {
		t.Fatalf("previous encryption key recovery = %x, err=%v", unwrapped, err)
	}
}

func TestCleanupSecurityRetentionRemovesExpiredState(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "retention.db"), false)
	closeTestResource(t, "database", database.Close)
	now := time.Now().UTC()
	if err := ensurePGPMetadataSchema(database); err != nil {
		t.Fatal(err)
	}
	if err := savePGPMetadata(database, pgpMetadata{LastRotatedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(root, "security")
	oldPrivateKey := filepath.Join(keyDir, "pgp.priv.old.asc.enc")
	if err := atomicPGPWrite(oldPrivateKey, []byte("retired key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE security_reports (tracking_id TEXT PRIMARY KEY, closed_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	_, err := database.Exec(`INSERT INTO security_reports(tracking_id, closed_at) VALUES (?, ?), (?, ?)`,
		"expired", now.Add(-400*24*time.Hour).Format(time.RFC3339), "recent", now.Add(-10*24*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureAppSecretsSchema(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO app_secrets(name, value) VALUES (?, ?), (?, ?)`,
		"token_previous", []byte("old"), "token_previous_until", []byte(now.Add(-time.Hour).Format(time.RFC3339))); err != nil {
		t.Fatal(err)
	}
	if err := CleanupSecurityReports(database, root, now, 365); err != nil {
		t.Fatal(err)
	}
	var reports, expiredSecrets int
	if err := database.QueryRow(`SELECT COUNT(*) FROM security_reports`).Scan(&reports); err != nil || reports != 1 {
		t.Fatalf("retained report count = %d, err=%v", reports, err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM app_secrets WHERE name LIKE 'token_previous%'`).Scan(&expiredSecrets); err != nil || expiredSecrets != 0 {
		t.Fatalf("expired secret marker count = %d, err=%v", expiredSecrets, err)
	}
	if _, err := os.Stat(oldPrivateKey); !os.IsNotExist(err) {
		t.Fatalf("retired PGP key still exists: %v", err)
	}
}

func TestComplianceCountsAndAuditEvents(t *testing.T) {
	root := newMainTestDir(t)
	databasePath := filepath.Join(root, "compliance.db")
	database := openRecoveryTestDB(t, databasePath, false)
	closeTestResource(t, "database", database.Close)
	if _, err := server.ListOwnerTokens(database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO owners(id) VALUES('owner-a'),('owner-b')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO api_tokens(owner_id,kind,token_hash,token_prefix,token_last4,token_enc) VALUES('owner-a','primary','hash-a','own_12345678','ABCD','encrypted')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE owned_resources(id TEXT PRIMARY KEY, owner_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO owned_resources(id,owner_id) VALUES('resource-a','owner-a'),('resource-b','owner-b')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE security_reports(tracking_id TEXT PRIMARY KEY, status TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO security_reports(tracking_id,status) VALUES('report-open','triaged'),('report-closed','disclosed')`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := ensureComplianceEventSchema(database); err != nil {
		t.Fatal(err)
	}
	if err := recordComplianceEvent(database, "compliance.report_generated", "", "source=test"); err != nil {
		t.Fatal(err)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := recordComplianceEventTx(tx, "compliance.data_export_requested", "owner-a", "request_id=test"); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(root, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(backupDir, "snapshot.zip")
	if err := os.WriteFile(backupPath, []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	owners, resources, tokens, reports, events, lastBackup, err := complianceCounts(database, backupDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if owners != 3 || resources != 2 || tokens != 1 || reports != 1 || events != 2 || lastBackup == "none" {
		t.Fatalf("complianceCounts() = %d, %d, %d, %d, %d, %q", owners, resources, tokens, reports, events, lastBackup)
	}
	owner, err := resolveOwnerByPrefix(database, "own_12345678")
	if err != nil || owner != "owner-a" {
		t.Fatalf("resolveOwnerByPrefix() = %q, %v", owner, err)
	}
	if _, err := resolveOwnerByPrefix(database, "bad"); err == nil {
		t.Fatal("invalid owner token prefix was accepted")
	}
	exportPath := filepath.Join(root, "owner-export.json")
	logsDir := filepath.Join(root, "logs")
	if err := exportOwnerData(database, logsDir, logging.LogManagerConfig{}, "owner-a", exportPath); err != nil {
		t.Fatal(err)
	}
	exportInfo, err := os.Stat(exportPath)
	if err != nil || exportInfo.Mode().Perm() != 0o600 {
		t.Fatalf("owner export mode = %v, err=%v", exportInfo, err)
	}
	export, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(export), "hash-a") || strings.Contains(string(export), "encrypted") || !strings.Contains(string(export), `"prefix": "own_12345678"`) {
		t.Fatalf("owner export exposes secret material or omits token metadata: %s", export)
	}
	if service.IsElevated() {
		configDir := filepath.Join(root, "config")
		cfg := config.DefaultApplicationConfig()
		cfg.Server.Database.URL = databasePath
		if err := config.SaveApplicationConfig(filepath.Join(configDir, "server.yml"), cfg); err != nil {
			t.Fatal(err)
		}
		commandExport := filepath.Join(root, "maintenance-owner-export.json")
		if err := runMaintenanceData(configDir, logsDir, "export --owner own_12345678 --output "+commandExport); err != nil {
			t.Fatalf("runMaintenanceData(export): %v", err)
		}
		if _, err := os.Stat(commandExport); err != nil {
			t.Fatalf("maintenance export file: %v", err)
		}
	}
	if err := deleteOwnerData(database, logsDir, logging.LogManagerConfig{}, "owner-a", "request-test"); err != nil {
		t.Fatal(err)
	}
	var remainingOwners, remainingTokens int
	if err := database.QueryRow(`SELECT COUNT(*) FROM owners`).Scan(&remainingOwners); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM api_tokens`).Scan(&remainingTokens); err != nil {
		t.Fatal(err)
	}
	if remainingOwners != 2 || remainingTokens != 0 {
		t.Fatalf("owner deletion left %d owner(s) and %d token(s)", remainingOwners, remainingTokens)
	}
	var ownerAResources, ownerBResources int
	if err := database.QueryRow(`SELECT COUNT(*) FROM owned_resources WHERE owner_id = 'owner-a'`).Scan(&ownerAResources); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM owned_resources WHERE owner_id = 'owner-b'`).Scan(&ownerBResources); err != nil {
		t.Fatal(err)
	}
	if ownerAResources != 0 || ownerBResources != 1 {
		t.Fatalf("owner resource deletion left owner-a=%d owner-b=%d", ownerAResources, ownerBResources)
	}
	requestID, err := newComplianceRequestID()
	if err != nil || len(requestID) != 32 || strings.Trim(requestID, "0123456789abcdef") != "" {
		t.Fatalf("newComplianceRequestID() = %q, %v", requestID, err)
	}
}
