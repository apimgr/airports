package i2p

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paths "github.com/apimgr/airports/src/path"
)

func TestSAMEepsitePersistsKeyAndUsesSeparateSilentForwardSocket(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "airports-i2p-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	privateBytes := make([]byte, 400)
	binary.BigEndian.PutUint16(privateBytes[385:387], 0)
	privateBytes[390], privateBytes[391] = 0xfb, 0xff
	private := i2pBase64.EncodeToString(privateBytes)
	if !strings.Contains(private, "-") || !strings.Contains(private, "~") {
		t.Fatal("test destination does not exercise the I2P base64 alphabet")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close SAM test listener: %v", err)
		}
	}()
	commands := make(chan []string, 3)
	serverErr := make(chan error, 1)
	keysPath := filepath.Join(dir, "site", "site-keys.dat")
	sessionCalls := 0
	go func() {
		var accepted int
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				serverErr <- acceptErr
				return
			}
			accepted++
			isForward := accepted == 2
			go func() {
				defer func() {
					if err := conn.Close(); err != nil {
						t.Errorf("close mock SAM connection: %v", err)
					}
				}()
				reader := bufio.NewReader(conn)
				lines := []string{}
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil {
						if isForward {
							serverErr <- readErr
						}
						return
					}
					line = strings.TrimSpace(line)
					lines = append(lines, line)
					switch {
					case strings.HasPrefix(line, "HELLO "):
						_, _ = fmt.Fprintln(conn, "HELLO REPLY RESULT=OK VERSION=3.3")
					case strings.HasPrefix(line, "DEST GENERATE "):
						_, _ = fmt.Fprintf(conn, "DEST REPLY RESULT=OK PUB=%s PRIV=%s\n", private, private)
					case strings.HasPrefix(line, "SESSION CREATE "):
						if _, statErr := os.Stat(keysPath); statErr != nil {
							serverErr <- fmt.Errorf("SESSION CREATE arrived before the destination key was persisted: %w", statErr)
						}
						sessionCalls++
						if sessionCalls == 1 {
							_, _ = fmt.Fprintln(conn, "SESSION STATUS RESULT=DUPLICATED_ID")
						} else {
							_, _ = fmt.Fprintln(conn, "SESSION STATUS RESULT=OK DESTINATION=known")
						}
						commands <- []string{line}
					case strings.HasPrefix(line, "STREAM FORWARD "):
						_, _ = fmt.Fprintln(conn, "STREAM STATUS RESULT=OK")
						commands <- lines
						return
					default:
						serverErr <- fmt.Errorf("unexpected SAM command")
						return
					}
				}
			}()
		}
	}()

	cfg := DefaultI2PConfig()
	cfg.BootstrapTimeout = 30
	svc := &I2PProviderService{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	address, err := startSAMEepsite(ctx, cfg, listener.Addr().String(), keysPath, 64500, svc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("close I2P test service: %v", err)
		}
	}()
	if !strings.HasSuffix(address, ".b32.i2p") {
		t.Fatalf("unexpected address %q", address)
	}
	stored, err := os.ReadFile(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stored)) != private {
		t.Fatal("destination key was not persisted as a single PRIV value")
	}
	first, second, third := <-commands, <-commands, <-commands
	if len(first) != 1 || !strings.Contains(first[0], " ID=site-") || !strings.HasPrefix(first[0], "SESSION CREATE ") {
		t.Fatalf("first control socket command = %v", first)
	}
	if len(second) != 1 || !strings.HasPrefix(second[0], "SESSION CREATE ") {
		t.Fatalf("retried control socket command = %v", second)
	}
	firstID := strings.Fields(first[0])[3]
	secondID := strings.Fields(second[0])[3]
	if firstID == secondID {
		t.Fatalf("DUPLICATED_ID retry reused session ID %q", firstID)
	}
	if len(third) != 2 || !strings.Contains(third[1], "SILENT=true") {
		t.Fatalf("forward socket commands = %v", third)
	}
	select {
	case err := <-serverErr:
		t.Fatal(err)
	default:
	}
}

func TestSAMEepsiteAcceptsSlowSessionStatusWithinConfiguredBootstrapTimeout(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "airports-i2p-slow-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	privateBytes := make([]byte, 400)
	private := i2pBase64.EncodeToString(privateBytes)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	serverErr := make(chan error, 2)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				reader := bufio.NewReader(conn)
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "HELLO "):
						_, _ = fmt.Fprintln(conn, "HELLO REPLY RESULT=OK VERSION=3.3")
					case strings.HasPrefix(line, "DEST GENERATE "):
						_, _ = fmt.Fprintf(conn, "DEST REPLY RESULT=OK PUB=%s PRIV=%s\n", private, private)
					case strings.HasPrefix(line, "SESSION CREATE "):
						time.Sleep(11 * time.Second)
						if _, writeErr := fmt.Fprintln(conn, "SESSION STATUS RESULT=OK DESTINATION=known"); writeErr != nil {
							serverErr <- writeErr
						}
					case strings.HasPrefix(line, "STREAM FORWARD "):
						_, _ = fmt.Fprintln(conn, "STREAM STATUS RESULT=OK")
						return
					default:
						serverErr <- fmt.Errorf("unexpected SAM command")
						return
					}
				}
			}()
		}
	}()

	cfg := DefaultI2PConfig()
	cfg.BootstrapTimeout = 60
	svc := &I2PProviderService{}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	started := time.Now()
	address, err := startSAMEepsite(ctx, cfg, listener.Addr().String(), filepath.Join(dir, "site", "site-keys.dat"), 64500, svc)
	if err != nil {
		t.Fatalf("complete slow SESSION CREATE within the configured 60-second timeout: %v", err)
	}
	defer func() { _ = svc.Close() }()
	if elapsed := time.Since(started); elapsed < 10*time.Second {
		t.Fatalf("mock SESSION CREATE was not slow enough to exceed the former 10-second bound: %s", elapsed)
	}
	if !strings.HasSuffix(address, ".b32.i2p") {
		t.Fatalf("slow SESSION CREATE returned invalid address %q", address)
	}
	select {
	case err := <-serverErr:
		t.Fatal(err)
	default:
	}
}

func TestI2PPrivateDestinationNeverAppearsInStartupLogs(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "airports-i2p-secret-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	privateBytes := make([]byte, 400)
	privateBytes[0], privateBytes[1], privateBytes[2] = 0xde, 0xad, 0xbe
	private := i2pBase64.EncodeToString(privateBytes)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	sessionCreate := make(chan struct{}, 1)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				reader := bufio.NewReader(conn)
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "HELLO "):
						_, _ = fmt.Fprintln(conn, "HELLO REPLY RESULT=OK VERSION=3.3")
					case strings.HasPrefix(line, "DEST GENERATE "):
						_, _ = fmt.Fprintf(conn, "DEST REPLY RESULT=OK PUB=%s PRIV=%s\n", private, private)
					case strings.HasPrefix(line, "SESSION CREATE "):
						_, _ = fmt.Fprintln(conn, "SESSION STATUS RESULT=INVALID_KEY")
						sessionCreate <- struct{}{}
						return
					case strings.HasPrefix(line, "QUIT"):
						return
					}
				}
			}()
		}
	}()

	cfg := DefaultI2PConfig()
	cfg.Enabled = true
	cfg.Binary = filepath.Join(dir, "missing-i2pd")
	cfg.SAMAddress = listener.Addr().String()
	manager := NewI2PManager(cfg, I2PDirs{
		ConfigDir: filepath.Join(dir, "config"),
		DataDir:   filepath.Join(dir, "data"),
		LogDir:    filepath.Join(dir, "logs"),
	})
	var logs bytes.Buffer
	previousLogOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousLogOutput)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.StartI2PService(ctx, http.NotFoundHandler()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sessionCreate:
	case <-time.After(15 * time.Second):
		t.Fatalf("mock SAM did not receive SESSION CREATE; startup logs: %s", logs.String())
	}
	cancel()
	deadline := time.Now().Add(15 * time.Second)
	for manager.GetI2PStatus() != "error" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.GetI2PStatus() != "error" {
		t.Fatal("mock SAM INVALID_KEY response did not fail I2P startup")
	}
	if err := manager.StopI2PService(); err != nil {
		t.Fatalf("stop I2P manager: %v", err)
	}
	if strings.Contains(logs.String(), private) {
		t.Fatalf("private destination appeared in startup logs: %s", logs.String())
	}
}

func TestI2PBase64AndAddressKnownVector(t *testing.T) {
	privateBytes := make([]byte, 400)
	binary.BigEndian.PutUint16(privateBytes[385:387], 0)
	privateBytes[390], privateBytes[391] = 0xfb, 0xff
	private := i2pBase64.EncodeToString(privateBytes)
	if !strings.Contains(private, "-") || !strings.Contains(private, "~") {
		t.Fatal("fixture does not exercise both I2P base64 alphabet characters")
	}
	decoded, err := i2pBase64.DecodeString(private)
	if err != nil {
		t.Fatalf("decode I2P base64: %v", err)
	}
	if got := i2pBase64.EncodeToString(decoded); got != private {
		t.Fatalf("I2P base64 round trip changed the key: got %q", got)
	}
	destination, err := destinationBytes(private)
	if err != nil {
		t.Fatalf("parse destination vector: %v", err)
	}
	const wantAddress = "gem7z2yovuoqqbg3sd5qzb5dhaiit6osezfdo3cbuonanzjsuzaq.b32.i2p"
	if got := b32Address(destination); got != wantAddress {
		t.Fatalf("destination address = %q, want %q", got, wantAddress)
	}
}

func TestI2PDConfigurationDoesNotEnableForbiddenListenerPorts(t *testing.T) {
	configuration := getI2PDConf(7656, "/tmp/i2pd.log")
	for _, port := range []string{"7070", "4444", "4447", "7654"} {
		if strings.Contains(configuration, port) {
			t.Errorf("i2pd configuration contains forbidden listener port %s", port)
		}
	}
}

func TestDestinationFileRejectsInvalidKeyWithoutReplacingIt(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "airports-i2p-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "site-keys.dat")
	if err := os.WriteFile(path, []byte("not-a-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateDestination(nil, nil, path, 7); err == nil {
		t.Fatal("invalid persisted key was accepted")
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != "not-a-key\n" {
		t.Fatal("invalid key file was overwritten")
	}
}
