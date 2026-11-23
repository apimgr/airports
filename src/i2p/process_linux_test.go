//go:build linux

package i2p

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestI2PParentSIGKILLStopsChild(t *testing.T) {
	switch os.Getenv("AIRPORTS_I2P_TEST_PROCESS_ROLE") {
	case "child":
		for {
			time.Sleep(time.Hour)
		}
	case "server":
		child := exec.Command(os.Args[0], "-test.run=^TestI2PParentSIGKILLStopsChild$")
		child.Env = append(os.Environ(), "AIRPORTS_I2P_TEST_PROCESS_ROLE=child")
		setParentDeathSignal(child)
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(os.Stdout, child.Process.Pid)
		for {
			time.Sleep(time.Hour)
		}
	}

	server := exec.Command(os.Args[0], "-test.run=^TestI2PParentSIGKILLStopsChild$")
	server.Env = append(os.Environ(), "AIRPORTS_I2P_TEST_PROCESS_ROLE=server")
	stdout, err := server.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Process.Kill()
		_ = server.Wait()
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read child PID: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || childPID < 2 {
		t.Fatalf("child PID %q is invalid: %v", line, err)
	}
	if err := server.Process.Kill(); err != nil {
		t.Fatalf("SIGKILL server helper: %v", err)
	}
	if err := server.Wait(); err == nil {
		t.Fatal("server helper unexpectedly exited without SIGKILL")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err == nil {
			if data, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/stat", childPID)); readErr == nil {
				fields := strings.Fields(string(data))
				if len(fields) > 2 && fields[2] == "Z" {
					return
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("i2pd child process %d survived its server's SIGKILL", childPID)
}
