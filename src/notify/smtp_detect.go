package notify

import (
	"bufio"
	"net"
	"net/smtp"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// probeTimeout bounds each individual host/port SMTP handshake attempt
// during autodetection, so a firewalled/black-holed host cannot stall
// startup per host for longer than this.
const probeTimeout = 2 * time.Second

// smtpPorts is the fixed port list tried against every candidate host per
// AI.md PART 17 "SMTP Auto-Detection" priority table (25, 465, 587).
var smtpPorts = []int{25, 465, 587}

// Autodetect implements AI.md PART 17 "SMTP Auto-Detection Process": try
// each host/port combination in the documented 7-host priority order,
// confirming a real SMTP listener via a TCP connect + EHLO handshake (not
// just an open port). The first successful combination wins. ok is false
// when no candidate responded, meaning email features stay disabled — not
// an error, per the spec ("just no SMTP available").
func Autodetect(fqdn string) (host string, port int, ok bool) {
	for _, candidate := range candidateHosts(fqdn) {
		if candidate == "" {
			continue
		}
		for _, p := range smtpPorts {
			if probeSMTP(candidate, p, probeTimeout) {
				return candidate, p, true
			}
		}
	}
	return "", 0, false
}

// candidateHosts builds the ordered host list per AI.md PART 17's priority
// table. Hosts that cannot be resolved (gateway IP, global IPv4) are
// omitted rather than probed as empty strings.
func candidateHosts(fqdn string) []string {
	hosts := []string{"127.0.0.1", "172.17.0.1"}

	if gw := defaultGatewayIP(); gw != "" {
		hosts = append(hosts, gw)
	}
	if fqdn != "" {
		hosts = append(hosts, fqdn)
	}
	if ip := globalIPv4(); ip != "" {
		hosts = append(hosts, ip)
	}
	if fqdn != "" {
		hosts = append(hosts, "mail."+fqdn, "smtp."+fqdn)
	}

	return hosts
}

// probeSMTP attempts a TCP connect followed by an SMTP EHLO handshake
// against host:port, confirming a real SMTP listener rather than merely an
// open port. Returns true only on a successful handshake.
func probeSMTP(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return false
	}
	defer client.Close()

	if err := client.Hello(ehloName()); err != nil {
		return false
	}
	_ = client.Quit()
	return true
}

// ehloName returns the local identity used in the EHLO/HELO greeting.
func ehloName() string {
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}
	return "localhost"
}

// defaultGatewayIP returns the machine's default gateway IPv4 address, or
// "" if it cannot be determined (non-Linux platforms, or the routing table
// is unreadable). Best-effort per AI.md PART 17 priority 3 — a failure here
// simply means that candidate is skipped, not an error.
func defaultGatewayIP() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		// Destination field "00000000" marks the default route.
		if fields[1] != "00000000" {
			continue
		}
		gwHex := fields[2]
		if len(gwHex) != 8 {
			continue
		}
		return hexRouteToIP(gwHex)
	}
	return ""
}

// hexRouteToIP converts /proc/net/route's little-endian hex-encoded IPv4
// address into dotted-quad form.
func hexRouteToIP(hexAddr string) string {
	if len(hexAddr) != 8 {
		return ""
	}
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseUint(hexAddr[i*2:i*2+2], 16, 8)
		if err != nil {
			return ""
		}
		// /proc/net/route stores the address in little-endian byte order.
		b[3-i] = byte(v)
	}
	return net.IP(b).String()
}

// globalIPv4 returns the machine's outbound IPv4 address if it appears to
// be globally routable (not RFC1918 private), or "" otherwise. This is a
// best-effort local routing-table lookup — no packets are actually sent
// (UDP Dial only resolves the route), per AI.md PART 17 priority 5.
func globalIPv4() string {
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", probeTimeout)
	if err != nil {
		return ""
	}
	defer conn.Close()

	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr.IP == nil {
		return ""
	}
	ip := udpAddr.IP.To4()
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return ""
	}
	return ip.String()
}
