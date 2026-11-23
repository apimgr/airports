package i2p

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var i2pBase64 = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-~")

type samReply struct {
	topic  string
	action string
	kv     map[string]string
}

func getI2PDConf(samPort int, logPath string) string {
	return fmt.Sprintf(`daemon = false
ipv4 = true
ipv6 = false
notransit = true
floodfill = false
log = file
logfile = %s
loglevel = warn

[http]
enabled = false
[httpproxy]
enabled = false
[socksproxy]
enabled = false
[sam]
enabled = true
address = 127.0.0.1
port = %d
[bob]
enabled = false
[i2cp]
enabled = false
[i2pcontrol]
enabled = false
[upnp]
enabled = false
`, logPath, samPort)
}

func startI2PD(ctx context.Context, binary string, dirs I2PDirs, svc *I2PProviderService) (string, error) {
	samPort, err := allocateRandomLoopbackPort()
	if err != nil {
		return "", err
	}
	confDir, dataDir := dirs.i2pConfigDir(), dirs.i2pDataDir()
	confPath := filepath.Join(confDir, "i2pd.conf")
	tunnelsPath := filepath.Join(confDir, "tunnels.conf")
	if err := writePrivateFile(confPath, []byte(getI2PDConf(samPort, filepath.Join(dirs.LogDir, "i2pd.log")))); err != nil {
		return "", fmt.Errorf("write i2pd.conf: %w", err)
	}
	if err := writePrivateFile(tunnelsPath, nil); err != nil {
		return "", fmt.Errorf("write empty tunnels.conf: %w", err)
	}
	tunnelsDir := filepath.Join(confDir, "tunnels.d")
	if err := os.MkdirAll(tunnelsDir, 0700); err != nil {
		return "", fmt.Errorf("create empty tunnels.d: %w", err)
	}
	if err := os.Chmod(tunnelsDir, 0700); err != nil {
		return "", fmt.Errorf("secure tunnels.d: %w", err)
	}
	if err := os.MkdirAll(dirs.LogDir, 0700); err != nil {
		return "", fmt.Errorf("create i2pd log directory: %w", err)
	}
	pidPath := filepath.Join(dataDir, "i2pd.pid")
	reapStaleI2PD(pidPath, binary)
	cmd := exec.Command(binary, "--conf", confPath, "--tunconf", tunnelsPath, "--tunnelsdir", tunnelsDir, "--datadir", dataDir, "--pidfile", pidPath)
	setParentDeathSignal(cmd)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start i2pd: %w", err)
	}
	svc.i2pd = cmd
	svc.i2pdDone = make(chan struct{})
	svc.pidPath = pidPath
	go func() {
		svc.i2pdErr = cmd.Wait()
		close(svc.i2pdDone)
	}()
	if err := writePrivateFile(pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid))); err != nil {
		_ = svc.Close()
		return "", err
	}
	samAddress := fmt.Sprintf("127.0.0.1:%d", samPort)
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if conn, _, err := samDial(ctx, samAddress); err == nil {
			_ = conn.Close()
			return samAddress, nil
		}
		select {
		case <-ctx.Done():
			_ = svc.Close()
			return "", ctx.Err()
		case <-svc.i2pdDone:
			return "", errors.New("i2pd exited before its SAM port became ready")
		case <-deadline.C:
			_ = svc.Close()
			return "", errors.New("i2pd SAM port did not become ready within 60 seconds")
		case <-ticker.C:
		}
	}
}

func allocateRandomLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

func reapStaleI2PD(pidPath, binary string) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid < 2 || pid == os.Getpid() {
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if runtime.GOOS == "windows" {
		return
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || !strings.Contains(string(cmdline), binary) {
		return
	}
	_ = process.Signal(os.Interrupt)
}

func parseSAMReply(line string) *samReply {
	reply := &samReply{kv: make(map[string]string)}
	line = strings.TrimSpace(line)
	if index := strings.Index(line, ` MESSAGE=`); index >= 0 {
		message := strings.TrimSpace(line[index+len(` MESSAGE=`):])
		reply.kv["MESSAGE"] = strings.Trim(message, `"`)
		line = strings.TrimSpace(line[:index])
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return reply
	}
	reply.topic, reply.action = fields[0], fields[1]
	for _, field := range fields[2:] {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			reply.kv[key] = strings.Trim(value, `"' `)
		}
	}
	return reply
}

func samCmd(conn net.Conn, reader *bufio.Reader, timeout time.Duration, command string, args ...any) (reply *samReply, resultErr error) {
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set SAM command deadline: %w", err)
	}
	defer func() {
		if err := conn.SetDeadline(time.Time{}); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("reset SAM command deadline: %w", err))
		}
	}()
	if _, err := fmt.Fprintf(conn, command+"\n", args...); err != nil {
		return nil, errors.New("SAM command write failed")
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, errors.New("SAM command reply failed")
	}
	return parseSAMReply(line), nil
}

func samDial(ctx context.Context, address string) (net.Conn, *bufio.Reader, error) {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	reply, err := samCmd(conn, reader, 10*time.Second, "HELLO VERSION MIN=3.1 MAX=3.3")
	if err != nil || reply.topic != "HELLO" || reply.action != "REPLY" || reply.kv["RESULT"] != "OK" {
		_ = conn.Close()
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("SAM HELLO was rejected")
	}
	return conn, reader, nil
}

func destinationBytes(private string) ([]byte, error) {
	decoded, err := i2pBase64.DecodeString(private)
	if err != nil || len(decoded) < 387 {
		return nil, errors.New("invalid destination key")
	}
	destinationLength := 387 + int(binary.BigEndian.Uint16(decoded[385:387]))
	if len(decoded) < destinationLength {
		return nil, errors.New("truncated destination key")
	}
	return decoded[:destinationLength], nil
}

func loadOrCreateDestination(conn net.Conn, reader *bufio.Reader, keysPath string, signatureType int) (string, error) {
	data, err := os.ReadFile(keysPath)
	if err == nil {
		private := strings.TrimSpace(string(data))
		if _, err := destinationBytes(private); err != nil {
			return "", fmt.Errorf("corrupt destination key file %s: %w", keysPath, err)
		}
		return private, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	reply, err := samCmd(conn, reader, 10*time.Second, "DEST GENERATE SIGNATURE_TYPE=%d", signatureType)
	if err != nil || reply.kv["RESULT"] != "OK" {
		return "", errors.New("SAM destination generation failed")
	}
	private := reply.kv["PRIV"]
	if _, err := destinationBytes(private); err != nil {
		return "", errors.New("SAM generated an invalid destination key")
	}
	if err := writeDestinationFile(keysPath, private); err != nil {
		return "", err
	}
	return private, nil
}

func writeDestinationFile(path, private string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tmp, private); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}

func startSAMEepsite(ctx context.Context, cfg I2PConfig, samAddress, keysPath string, backendPort int, svc *I2PProviderService) (string, error) {
	control, reader, err := samDial(ctx, samAddress)
	if err != nil {
		return "", fmt.Errorf("failed to reach SAM: %w", err)
	}
	private, err := loadOrCreateDestination(control, reader, keysPath, cfg.SignatureType)
	if err != nil {
		_ = control.Close()
		return "", err
	}
	destination, err := destinationBytes(private)
	if err != nil {
		_ = control.Close()
		return "", err
	}
	id := fmt.Sprintf("site-%d", os.Getpid())
	create := "SESSION CREATE STYLE=STREAM ID=%s DESTINATION=%s i2cp.leaseSetEncType=4 inbound.length=%d outbound.length=%d inbound.quantity=%d outbound.quantity=%d"
	var reply *samReply
	for attempt := 0; attempt < 2; attempt++ {
		reply, err = samCmd(control, reader, time.Duration(cfg.BootstrapTimeout)*time.Second, create,
			id, private, cfg.InboundLength, cfg.OutboundLength, cfg.InboundQuantity, cfg.OutboundQuantity)
		if err != nil {
			_ = control.Close()
			return "", errors.New("SAM session creation failed")
		}
		if reply.kv["RESULT"] != "DUPLICATED_ID" || attempt == 1 {
			break
		}
		id = fmt.Sprintf("site-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	if reply.kv["RESULT"] != "OK" {
		_ = control.Close()
		return "", fmt.Errorf("SAM session creation failed: %s", reply.kv["RESULT"])
	}
	forward, forwardReader, err := samDial(ctx, samAddress)
	if err != nil {
		_ = control.Close()
		return "", errors.New("failed to open SAM forwarding socket")
	}
	reply, err = samCmd(forward, forwardReader, 10*time.Second, "STREAM FORWARD ID=%s PORT=%d HOST=127.0.0.1 SILENT=true", id, backendPort)
	if err != nil || reply.kv["RESULT"] != "OK" {
		_ = forward.Close()
		_ = control.Close()
		return "", errors.New("SAM stream forwarding failed")
	}
	svc.samConn, svc.forwardConn = control, forward
	svc.sessionLost = watchSAMSocket(control)
	svc.forwardLost = watchSAMSocket(forward)
	return b32Address(destination), nil
}

func watchSAMSocket(conn net.Conn) <-chan struct{} {
	lost := make(chan struct{})
	go func() {
		var data [1]byte
		_, _ = conn.Read(data[:])
		close(lost)
	}()
	return lost
}

func b32Address(destination []byte) string {
	hash := sha256.Sum256(destination)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])
	return strings.ToLower(encoded) + ".b32.i2p"
}

func probeSAM(ctx context.Context, address string) error {
	conn, _, err := samDial(ctx, address)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(conn, "QUIT")
	return conn.Close()
}
