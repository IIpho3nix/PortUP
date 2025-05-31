package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/huin/goupnp/dcps/internetgateway1"
)

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
  PortUP <tcp|udp> <port mapping> [<port mapping> ...]

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
  `)
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	protocol := strings.ToLower(os.Args[1])
	if protocol != "tcp" && protocol != "udp" {
		printUsage()
		logger.Fatalf("Invalid protocol: %s. Must be tcp or udp.", protocol)
	}

	mappings, err := parseArgs(os.Args[2:], protocol)
	if err != nil {
		printUsage()
		logger.Fatal(err)
	}

	logger.Info("Discovering UPnP gateway...")
	devices, errs, err := internetgateway1.NewWANIPConnection1Clients()
	if len(devices) == 0 {
		for _, err := range errs {
			logger.Info("Discovery error:", err)
		}
		logger.Fatal("No UPnP gateway found.")
	}
	client := devices[0]
	logger.Info("UPnP gateway found.")

	addedMappings := []Mapping{}
	publicIP, _ := client.GetExternalIPAddress()

	printLogo()
	fmt.Println("Currently Forwarding Ports:")

	for _, m := range mappings {
		desc := fmt.Sprintf("PortUP %s %d", strings.ToUpper(protocol), m.LocalPort)
		err := client.AddPortMapping("", uint16(m.RemotePort), m.Protocol, uint16(m.LocalPort), m.localIP, true, desc, 0)
		if err != nil {
			logger.Fatalf("Failed to add port mapping %d -> %d (%s): %v", m.RemotePort, m.LocalPort, m.Protocol, err)
		}

		fmt.Printf(" %s%s%s:%s%d%s %s->%s %s%s%s:%s%d%s\n",
			purple, publicIP, reset, cyan, m.RemotePort, reset,
			green, reset,
			purple, m.localIP, reset, cyan, m.LocalPort, reset,
		)

		addedMappings = append(addedMappings, m)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL)
	<-sigs

	logger.Info("Caught shutdown signal. Cleaning up port mappings...")
	for _, m := range addedMappings {
		err := client.DeletePortMapping("", uint16(m.RemotePort), m.Protocol)
		if err != nil {
			logger.Printf("Failed to remove port mapping %d (%s): %v", m.RemotePort, m.Protocol, err)
		}
	}
	logger.Info("Shutdown complete.")
}
