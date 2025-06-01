# ![PortUP Logo](./assets/Logo.png "PortUP Logo")

[![GitHub release](https://img.shields.io/github/release/IIpho3nix/PortUP?include_prereleases=&sort=semver&color=brightgreen)](https://github.com/IIpho3nix/PortUP/releases/)
[![License](https://img.shields.io/badge/License-MIT-brightgreen)](#license)
[![issues - PortUP](https://img.shields.io/github/issues/IIpho3nix/PortUP)](https://github.com/IIpho3nix/PortUP/issues)
[![stars - PortUP](https://img.shields.io/github/stars/IIpho3nix/PortUP?style=social)](https://github.com/IIpho3nix/PortUP)
[![forks - PortUP](https://img.shields.io/github/forks/IIpho3nix/PortUP?style=social)](https://github.com/IIpho3nix/PortUP)

![PortUP Showcase](./assets/PortUP.gif "PortUP Showcase")

🚀 **PortUP** is a simple command-line tool that uses UPnP to expose local ports, making your services accessible from the internet.

---

## 🌐 Features

- ✅ Supports **TCP** and **UDP** port forwarding
- ✅ Add **multiple port mappings** at once
- ✅ Supports **IGDv2** and **IGDv1**
- ✅ Friendly **console output** with colorized formatting

---

## 🛠️ Installation

```bash
go install github.com/IIpho3nix/PortUP@latest
```

or download the binaries from [Releases](https://github.com/IIpho3nix/PortUP/releases)

---

## ⚙️ Usage

```bash
PortUP <tcp|udp> <port mapping> [<port mapping> ...]
```

### 🧾 Port Mapping Formats

- `<port>`  
  Forward local port to the same remote port.

- `<local>~<remote>`  
  Forward local port to a different remote port.

- `<ip:port>`  
  Forward from a specific local IP and port to same remote port.

- `<ip:port>~<remote>`  
  Forward from specific local IP and port to remote port.

### 📌 Examples

```bash
PortUP tcp 8080
```

> Forwards local TCP port 8080 to external port 8080.

```bash
PortUP udp 5000~6000
```

> Forwards local UDP port 5000 to external port 6000.

```bash
PortUP tcp 8080~12345 9090 7070~7071
```

> Forwards multiple TCP ports with custom mappings.

```bash
PortUP tcp 192.168.1.50:8080~80
```

> Forwards external TCP port 80 to 192.168.1.50's port 8080.

```bash
PortUP cleanup
```

> Cleans up previous port mappings left behind after a improper shutdown.

## ⚠️ Requirements

- A router that supports **UPnP** and has it enabled.

---

## 🙌 Acknowledgments

- [charmbracelet/log](https://github.com/charmbracelet/log) — for beautiful logging
- [huin/goupnp](https://github.com/huin/goupnp) — for UPnP

---

## 📝 License

Released under [MIT](/LICENSE) by [IIpho3nix](https://github.com/IIpho3nix).
