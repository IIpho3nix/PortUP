package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Masterminds/semver"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/huin/goupnp/dcps/internetgateway1"
)

const VERSION = "1.3.2"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
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
	fmt.Println(`Usage:
  PortUP <tcp|udp|cleanup> <port mapping> [<port mapping> ...]

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
  PortUP cleanup
  `)
}

func main() {
	styles := log.DefaultStyles()
	styles.Timestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("235"))
	logger.SetStyles(styles)

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

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		if strings.ToLower(os.Args[1]) != "cleanup" {
			printUsage()
			os.Exit(1)
		}
	}

	protocol := strings.ToLower(os.Args[1])
	if protocol != "cleanup" && protocol != "tcp" && protocol != "udp" {
		printUsage()
		logger.Fatalf("Invalid protocol: %s. Must be tcp or udp.", protocol)
	}

	var mappings []Mapping
	var err error

	if protocol != "cleanup" {
		mappings, err = parseArgs(os.Args[2:], protocol)
	}

	if err != nil {
		printUsage()
		logger.Fatal(err)
	}

	logger.Info("Discovering UPnP gateway...")
	devices, errs, err := internetgateway1.NewWANIPConnection1Clients()
	if len(devices) == 0 {
		for _, err := range errs {
			logger.Infof("Discovery error: %v", err)
		}
		logger.Fatal("No UPnP gateway found.")
	}
	client := devices[0]
	logger.Info("UPnP gateway found.")

	addedMappings := []Mapping{}
	publicIP, _ := client.GetExternalIPAddress()

	if protocol == "cleanup" {
		//UPnP has a max of 64 port mappings on most consumer hardware
		for i := 0; i < 64; i++ {
			_, extport, proto, _, _, _, desc, _, err := client.GetGenericPortMappingEntry(uint16(i))
			time.Sleep(time.Millisecond * 50)
			if err == nil && strings.Contains(desc, "PortUP") {
				addedMappings = append(addedMappings, Mapping{RemotePort: int(extport), Protocol: proto})
			}
		}

		for _, m := range addedMappings {
			err := client.DeletePortMapping("", uint16(m.RemotePort), m.Protocol)
			if err != nil {
				logger.Warnf("Failed to remove port mapping %d (%s): %v", m.RemotePort, m.Protocol, err)
			} else {
				logger.Infof("Removed port mapping %d (%s)", m.RemotePort, m.Protocol)
			}
		}

		logger.Info("PortUP cleanup complete.")
		os.Exit(0)
	}

	for _, m := range mappings {
		desc := fmt.Sprintf("PortUP %s %d", strings.ToUpper(protocol), m.LocalPort)
		err := client.AddPortMapping("", uint16(m.RemotePort), m.Protocol, uint16(m.LocalPort), m.localIP, true, desc, 0)
		if err != nil {
			logger.Warnf("Failed to add port mapping %d -> %d (%s): %v", m.RemotePort, m.LocalPort, m.Protocol, err)
			time.Sleep(50 * time.Millisecond)
			client = devices[0]
			continue
		}

		addedMappings = append(addedMappings, m)
	}

	printLogo()
	fmt.Println("Currently Forwarding Ports:")

	for _, m := range addedMappings {
		fmt.Printf(" %s%s%s:%s%d%s %s->%s %s%s%s:%s%d%s\n",
			purple, publicIP, reset, cyan, m.RemotePort, reset,
			green, reset,
			purple, m.localIP, reset, cyan, m.LocalPort, reset,
		)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL)
	<-sigs

	logger.Info("Caught shutdown signal. Cleaning up port mappings...")
	for _, m := range addedMappings {
		err := client.DeletePortMapping("", uint16(m.RemotePort), m.Protocol)
		if err != nil {
			logger.Warnf("Failed to remove port mapping %d (%s): %v", m.RemotePort, m.Protocol, err)
			time.Sleep(50 * time.Millisecond)
			client = devices[0]
		}
	}
	logger.Info("Shutdown complete.")
}
