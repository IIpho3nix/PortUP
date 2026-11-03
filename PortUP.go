package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Masterminds/semver"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const VERSION = "1.7.0"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

func startMDNS(localIP, name string) error {
	ip := net.ParseIP(localIP)
	if ip == nil {
		return fmt.Errorf("invalid IP: %s", localIP)
	}

	addr4, err := net.ResolveUDPAddr("udp4", mdns.DefaultAddressIPv4)
	if err != nil {
		return err
	}
	addr6, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6)
	if err != nil {
		return err
	}

	l4, err := net.ListenUDP("udp4", addr4)
	if err != nil {
		return err
	}
	l6, err := net.ListenUDP("udp6", addr6)
	if err != nil {
		return err
	}

	_, err = mdns.Server(ipv4.NewPacketConn(l4), ipv6.NewPacketConn(l6), &mdns.Config{
		LocalNames: []string{name + ".local"},
	})
	if err != nil {
		return err
	}

	logger.Infof("mDNS broadcast started: %s.local [%s]", name, localIP)
	return nil
}

func getLatestVersion() (string, error) {
	url := "https://api.github.com/repos/IIpho3nix/PortUP/releases/latest"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-http-client")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status code %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

const (
	green  = "\033[38;5;46m"
	cyan   = "\033[38;5;86m"
	purple = "\033[38;5;134m"
	reset  = "\033[0m"
)

type Mapping struct {
	LocalPort  int
	RemotePort int
	localIP    string
	Protocol   string
}

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
})

func parseArgs(args []string, protocol string) ([]Mapping, error) {
	var mappings []Mapping
	for _, arg := range args {
		var localPort, remotePort int
		var localIP string
		parts := strings.Split(arg, "~")
		switch len(parts) {
		case 1:
			if strings.Contains(parts[0], ":") {
				var parts2 = strings.Split(parts[0], ":")
				localIP = parts2[0]
				p, err := strconv.Atoi(parts2[1])
				if err != nil {
					return nil, fmt.Errorf("invalid port: %s", arg)
				}
				localPort = p
				remotePort = p
			} else {
				p, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid port: %s", arg)
				}
				localIP = getLocalIP()
				localPort = p
				remotePort = p
			}
		case 2:
			if strings.Contains(parts[0], ":") {
				var parts2 = strings.Split(parts[0], ":")
				localIP = parts2[0]
				lp, err := strconv.Atoi(parts2[1])
				if err != nil {
					return nil, fmt.Errorf("invalid port mapping: %s", arg)
				}
				localPort = lp
			} else {
				lp, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid port mapping: %s", arg)
				}
				localIP = getLocalIP()
				localPort = lp
			}

			rp, err2 := strconv.Atoi(parts[1])
			if err2 != nil {
				return nil, fmt.Errorf("invalid port mapping: %s", arg)
			}
			remotePort = rp
		default:
			return nil, fmt.Errorf("invalid format: %s", arg)
		}

		if !isValidPort(localPort) {
			return nil, fmt.Errorf("invalid local port: %d", localPort)
		}
		if !isValidPort(remotePort) {
			return nil, fmt.Errorf("invalid remote port: %d", remotePort)
		}

		if !isValidLocalIP(localIP) {
			return nil, fmt.Errorf("invalid local IP: %s", localIP)
		} else {
			if !isInSameRange(localIP, getLocalIP()) {
				logger.Warnf("Local IP %s is not in the same network range as the local IP of current machine %s, UPnP may fail.", localIP, getLocalIP())
				logger.Warn(("Are you sure you entered the correct IP address?"))
			}
		}

		mappings = append(mappings, Mapping{
			LocalPort:  localPort,
			RemotePort: remotePort,
			localIP:    localIP,
			Protocol:   strings.ToUpper(protocol),
		})
	}
	return mappings, nil
}

var dnsLabelRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func isValidDNSLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}
	return dnsLabelRegex.MatchString(label)
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func isValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func isValidLocalIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateCIDRs {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func isInSameRange(ipStr1, ipStr2 string) bool {
	ip1 := net.ParseIP(ipStr1)
	ip2 := net.ParseIP(ipStr2)
	if ip1 == nil || ip2 == nil {
		return false
	}

	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
	}

	for _, cidr := range privateCIDRs {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip1) && subnet.Contains(ip2) {
			return true
		}
	}
	return false
}

func isNewerVersion(v1, v2 string) (bool, error) {
	version1, err := semver.NewVersion(v1)
	if err != nil {
		return false, err
	}

	version2, err := semver.NewVersion(v2)
	if err != nil {
		return false, err
	}

	return version1.GreaterThan(version2), nil
}

func printLogo() {
	logo := `
 ____    ___   ____   ______  __ __  ____
|    \  /   \ |    \ |      T|  T  T|    \
|  o  )Y     Y|  D  )|      ||  |  ||  o  )
|   _/ |  O  ||    / l_j  l_j|  |  ||   _/
|  |   |     ||    \   |  |  |  :  ||  |
|  |   l     !|  .  Y  |  |  l     ||  |
l__j    \___/ l__j\_j  l__j   \__,_jl__j
`
	fmt.Print(cyan + logo + reset)
	fmt.Println()
}

func printUsage() {
	if jsonOut {
		return
	}
	fmt.Fprintln(os.Stderr, `Usage:
  PortUP <tcp|udp|cleanup> <port mapping> [<port mapping> ...] [--json]
  PortUP <tcp|udp> auto [--json]

Flags:
  --json                     Machine-readable JSON events on stdout, JSON logs on stderr

Description:
  Forward local ports to remote ports over TCP or UDP.

Port Mapping Formats:
  <port>                     Forward local port to the same remote port
  <local>~<remote>           Forward local port to a different remote port
  <ip:port>                  Forward from a specific local IP and port to same remote port
  <ip:port>~<remote>         Forward from specific local IP and port to remote port

Examples:
  PortUP tcp 8080~12345
  PortUP udp 192.168.1.101:5000
  PortUP tcp 192.168.1.101:8080~80
  PortUP udp 8080 192.168.1.50:1234~5678
  PortUP mdns machine
  PortUP mdns 192.168.1.101~service	
  PortUP cleanup
  PortUP tcp auto
  PortUP udp auto --json
  `)
}

type Client interface {
	GetExternalIPAddress() (string, error)
	GetGenericPortMappingEntry(uint16) (string, uint16, string, uint16, string, bool, string, uint32, error)
	DeletePortMapping(string, uint16, string) error
	AddPortMapping(string, uint16, string, uint16, string, bool, string, uint32) error
}

type wrappedClient struct {
	raw any
}

func (w *wrappedClient) GetExternalIPAddress() (string, error) {
	switch c := w.raw.(type) {
	case *internetgateway2.WANIPConnection2:
		return c.GetExternalIPAddress()
	case *internetgateway2.WANIPConnection1:
		return c.GetExternalIPAddress()
	case *internetgateway1.WANIPConnection1:
		return c.GetExternalIPAddress()
	default:
		return "", fmt.Errorf("unsupported client type")
	}
}

func (w *wrappedClient) GetGenericPortMappingEntry(port uint16) (string, uint16, string, uint16, string, bool, string, uint32, error) {
	switch c := w.raw.(type) {
	case *internetgateway2.WANIPConnection2:
		return c.GetGenericPortMappingEntry(port)
	case *internetgateway2.WANIPConnection1:
		return c.GetGenericPortMappingEntry(port)
	case *internetgateway1.WANIPConnection1:
		return c.GetGenericPortMappingEntry(port)
	default:
		return "", 0, "", 0, "", false, "", 0, fmt.Errorf("unsupported client type")
	}
}

func (w *wrappedClient) DeletePortMapping(externalIPAddress string, externalPort uint16, protocol string) error {
	switch c := w.raw.(type) {
	case *internetgateway2.WANIPConnection2:
		return c.DeletePortMapping(externalIPAddress, externalPort, protocol)
	case *internetgateway2.WANIPConnection1:
		return c.DeletePortMapping(externalIPAddress, externalPort, protocol)
	case *internetgateway1.WANIPConnection1:
		return c.DeletePortMapping(externalIPAddress, externalPort, protocol)
	default:
		return fmt.Errorf("unsupported client type")
	}
}

func (w *wrappedClient) AddPortMapping(NewRemoteHost string, NewExternalPort uint16, NewProtocol string, NewInternalPort uint16, NewInternalClient string, NewEnabled bool, NewPortMappingDescription string, NewLeaseDuration uint32) error {
	switch c := w.raw.(type) {
	case *internetgateway2.WANIPConnection2:
		return c.AddPortMapping(NewRemoteHost, NewExternalPort, NewProtocol, NewInternalPort, NewInternalClient, NewEnabled, NewPortMappingDescription, NewLeaseDuration)
	case *internetgateway2.WANIPConnection1:
		return c.AddPortMapping(NewRemoteHost, NewExternalPort, NewProtocol, NewInternalPort, NewInternalClient, NewEnabled, NewPortMappingDescription, NewLeaseDuration)
	case *internetgateway1.WANIPConnection1:
		return c.AddPortMapping(NewRemoteHost, NewExternalPort, NewProtocol, NewInternalPort, NewInternalClient, NewEnabled, NewPortMappingDescription, NewLeaseDuration)
	default:
		return fmt.Errorf("unsupported client type")
	}
}

func getPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get("https://www.cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "ip=") {
			ip := strings.TrimPrefix(line, "ip=")
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

var jsonOut bool

type mappingOut struct {
	Protocol   string `json:"protocol"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
}

type readyOut struct {
	Event    string       `json:"event"`
	PublicIP string       `json:"public_ip,omitempty"`
	Mappings []mappingOut `json:"mappings,omitempty"`
	MDNS     []mdnsOut    `json:"mdns,omitempty"`
}

type errorOut struct {
	Event string `json:"event"`
	Error string `json:"error"`
}

type cleanupEntry struct {
	Protocol   string `json:"protocol"`
	RemotePort int    `json:"remote_port"`
	Error      string `json:"error,omitempty"`
}

type cleanupOut struct {
	Event   string         `json:"event"`
	Removed []cleanupEntry `json:"removed"`
	Failed  []cleanupEntry `json:"failed"`
}

type stoppedOut struct {
	Event string `json:"event"`
}

type mdnsOut struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

func parseFlags(args []string) []string {
	rest := []string{args[0]}
	for _, a := range args[1:] {
		if a == "--json" {
			jsonOut = true
		} else {
			rest = append(rest, a)
		}
	}
	return rest
}

func fail(format string, a ...any) {
	if jsonOut {
		printJSON(errorOut{Event: "error", Error: fmt.Sprintf(format, a...)})
		os.Exit(1)
	}
	logger.Fatalf(format, a...)
}

func printJSON(v any) {
	out, _ := json.Marshal(v)
	fmt.Println(string(out))
}

func isLocalPortFree(protocol string, port int) bool {
	addr := fmt.Sprintf(":%d", port)
	if protocol == "udp" {
		c, err := net.ListenPacket("udp", addr)
		if err != nil {
			return false
		}
		c.Close()
		return true
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func pickFreePort(client Client, protocol string) (int, error) {
	used := map[int]bool{}
	for i := 0; i < 64; i++ {
		_, extport, proto, _, _, _, _, _, err := client.GetGenericPortMappingEntry(uint16(i))
		if err != nil {
			break
		}
		if strings.EqualFold(proto, protocol) {
			used[int(extport)] = true
		}
	}

	for i := 0; i < 500; i++ {
		port := 49152 + rand.Intn(65536-49152)
		if used[port] || !isLocalPortFree(protocol, port) {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("no free port found")
}

func removeMappings(client Client, mappings []Mapping) {
	for _, m := range mappings {
		err := client.DeletePortMapping("", uint16(m.RemotePort), m.Protocol)
		if err != nil {
			logger.Warnf("Failed to remove port mapping %d (%s): %v", m.RemotePort, m.Protocol, err)
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func checkForUpdate() {
	latestVer, verErr := getLatestVersion()
	if verErr != nil {
		logger.Warnf("Failed to fetch latest version: %v", verErr)
		latestVer = VERSION
	}

	logger.Infof("PortUP v%s", VERSION)

	update, errllatestVer := isNewerVersion(latestVer, VERSION)
	if errllatestVer != nil {
		logger.Warnf("Failed to check for updates: %v", errllatestVer)
		update = false
	}

	if update {
		logger.Infof("A new version (%s) is available. it is recommended to update to the latest version.", latestVer)
	}
}

func main() {
	args := parseFlags(os.Args)

	styles := log.DefaultStyles()
	styles.Timestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	logger.SetStyles(styles)

	if jsonOut {
		logger.SetReportTimestamp(false)
		logger.SetLevel(log.WarnLevel)
		logger.SetFormatter(log.JSONFormatter)
	} else {
		checkForUpdate()
	}

	if len(args) < 2 {
		printUsage()
		fail("Missing command.")
	}

	if strings.ToLower(args[1]) == "mdns" {
		if len(args) < 3 {
			printUsage()
			fail("Missing mDNS name or mappings.")
		}

		type mdnsEntry struct {
			IP   string
			Name string
		}

		var entries []mdnsEntry

		for _, arg := range args[2:] {
			var ip, name string

			if strings.Contains(arg, "~") {
				parts := strings.Split(arg, "~")
				if len(parts) != 2 {
					fail("Invalid mDNS mapping: %s", arg)
				}
				ip = parts[0]
				name = parts[1]
			} else {
				ip = getLocalIP()
				name = arg
			}

			if !isValidLocalIP(ip) {
				fail("Invalid local IP: %s", ip)
			}

			if !isValidDNSLabel(name) {
				logger.Warnf("'%s' does not match standard DNS label requirements (letters, numbers, hyphens only). While this is allowed under RFC 6762, some operating systems and programs may fail to resolve these.", name)
			}

			entries = append(entries, mdnsEntry{IP: ip, Name: name})
		}

		for _, entry := range entries {
			err := startMDNS(entry.IP, entry.Name)
			if err != nil {
				logger.Warnf("Failed to start mDNS for %s.local (%s): %v", entry.Name, entry.IP, err)
			} else {
				logger.Infof("Started mDNS for %s.local [%s]", entry.Name, entry.IP)
			}
		}

		if jsonOut {
			out := readyOut{Event: "mdns_ready"}
			for _, entry := range entries {
				out.MDNS = append(out.MDNS, mdnsOut{Name: entry.Name + ".local", IP: entry.IP})
			}
			printJSON(out)
		} else {
			printLogo()
			fmt.Println("mDNS registrations:")
			for _, entry := range entries {
				fmt.Printf("  %s%s.local%s -> %s%s%s\n", purple, entry.Name, reset, cyan, entry.IP, reset)
			}
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL)
		<-sigs

		logger.Info("Caught shutdown signal. Exiting mDNS mode.")
		if jsonOut {
			printJSON(stoppedOut{Event: "stopped"})
		}
		os.Exit(0)
	}

	if len(args) < 3 {
		if strings.ToLower(args[1]) != "cleanup" {
			printUsage()
			fail("Missing port mappings.")
		}
	}

	protocol := strings.ToLower(args[1])
	if protocol != "cleanup" && protocol != "tcp" && protocol != "udp" {
		printUsage()
		fail("Invalid protocol: %s. Must be tcp or udp.", protocol)
	}

	var mappings []Mapping
	var err error

	isAuto := protocol != "cleanup" && len(args) == 3 && strings.ToLower(args[2]) == "auto"

	if protocol != "cleanup" && !isAuto {
		mappings, err = parseArgs(args[2:], protocol)
	}

	if err != nil {
		printUsage()
		fail("%v", err)
	}

	var TempClient any

	logger.Info("Discovering UPnP gateway...")
	devices2, errs, err := internetgateway2.NewWANIPConnection2Clients()
	if len(devices2) == 0 {
		for _, err := range errs {
			logger.Infof("Discovery error: %v", err)
		}
		devices, errs, _ := internetgateway2.NewWANIPConnection1Clients()
		if len(devices) == 0 {
			for _, err := range errs {
				logger.Infof("Discovery error: %v", err)
			}
			devicesIG1, errs, _ := internetgateway1.NewWANIPConnection1Clients()
			if len(devicesIG1) == 0 {
				for _, err := range errs {
					logger.Infof("Discovery error: %v", err)
				}
				fail("No UPnP gateway found. Please make sure UPnP is enabled on your router.")
			} else {
				logger.Info("UPnP gateway found. IGDv1")
				TempClient = devicesIG1[0]
			}
		} else {
			logger.Info("UPnP gateway found. IGDv2, WANIPConnection1")
			TempClient = devices[0]
		}
	} else {
		logger.Info("UPnP gateway found. IGDv2, WANIPConnection2")
		TempClient = devices2[0]
	}

	client := &wrappedClient{raw: TempClient}

	addedMappings := []Mapping{}
	publicIP, _ := client.GetExternalIPAddress()

	if getPublicIP() != publicIP {
		logger.Warnf("Public IP mismatch: %s != %s, Likely Double NAT, PortUP may not work.", getPublicIP(), publicIP)
	}

	if publicIP == "0.0.0.0" {
		logger.Warn("Public IP is 0.0.0.0, UPnP may be disabled/misconfigured, PortUP may not work.")
	}

	if protocol == "cleanup" {
		//UPnP has a max of 64 port mappings on most consumer hardware
		for i := 0; i < 64; i++ {
			_, extport, proto, _, _, _, desc, _, err := client.GetGenericPortMappingEntry(uint16(i))
			time.Sleep(time.Millisecond * 50)
			if err == nil && strings.Contains(desc, "PortUP") {
				addedMappings = append(addedMappings, Mapping{RemotePort: int(extport), Protocol: proto})
			}
		}

		result := cleanupOut{Event: "cleanup", Removed: []cleanupEntry{}, Failed: []cleanupEntry{}}
		for _, m := range addedMappings {
			err := client.DeletePortMapping("", uint16(m.RemotePort), m.Protocol)
			if err != nil {
				logger.Warnf("Failed to remove port mapping %d (%s): %v", m.RemotePort, m.Protocol, err)
				result.Failed = append(result.Failed, cleanupEntry{Protocol: strings.ToLower(m.Protocol), RemotePort: m.RemotePort, Error: err.Error()})
			} else {
				logger.Infof("Removed port mapping %d (%s)", m.RemotePort, m.Protocol)
				result.Removed = append(result.Removed, cleanupEntry{Protocol: strings.ToLower(m.Protocol), RemotePort: m.RemotePort})
			}
		}

		if jsonOut {
			printJSON(result)
		} else {
			logger.Infof("PortUP cleanup complete: %d removed, %d failed.", len(result.Removed), len(result.Failed))
		}
		if len(result.Failed) > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if isAuto {
		localIP := getLocalIP()
		if !isValidLocalIP(localIP) {
			fail("Invalid local IP: %s", localIP)
		}
		port, err := pickFreePort(client, protocol)
		if err != nil {
			fail("%v", err)
		}
		mappings = []Mapping{{LocalPort: port, RemotePort: port, localIP: localIP, Protocol: strings.ToUpper(protocol)}}
	}

	for _, m := range mappings {
		desc := fmt.Sprintf("PortUP %s %d", strings.ToUpper(protocol), m.LocalPort)
		err := client.AddPortMapping("", uint16(m.RemotePort), m.Protocol, uint16(m.LocalPort), m.localIP, true, desc, 0)
		if err != nil {
			logger.Warnf("Failed to add port mapping %d -> %d (%s): %v", m.RemotePort, m.LocalPort, m.Protocol, err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		addedMappings = append(addedMappings, m)
	}

	if len(addedMappings) == 0 || (jsonOut && len(addedMappings) != len(mappings)) {
		removeMappings(client, addedMappings)
		fail("Failed to add port mappings.")
	}

	if jsonOut {
		out := readyOut{Event: "ready", PublicIP: publicIP}
		for _, m := range addedMappings {
			out.Mappings = append(out.Mappings, mappingOut{Protocol: strings.ToLower(m.Protocol), LocalIP: m.localIP, LocalPort: m.LocalPort, RemotePort: m.RemotePort})
		}
		printJSON(out)
	} else {
		printLogo()
		fmt.Println("Currently Forwarding Ports:")

		for _, m := range addedMappings {
			fmt.Printf(" %s%s%s:%s%d%s %s->%s %s%s%s:%s%d%s\n",
				purple, publicIP, reset, cyan, m.RemotePort, reset,
				green, reset,
				purple, m.localIP, reset, cyan, m.LocalPort, reset,
			)
		}
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL)
	<-sigs

	logger.Info("Caught shutdown signal. Cleaning up port mappings...")
	removeMappings(client, addedMappings)
	logger.Info("Shutdown complete.")
	if jsonOut {
		printJSON(stoppedOut{Event: "stopped"})
	}
}
