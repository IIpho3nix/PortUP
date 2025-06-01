# PortUP

🚀 **PortUP** is a simple command-line tool that uses UPnP to expose local ports, making your services accessible from the internet.

---

## 🌐 Features

- ✅ Supports **TCP** and **UDP** port forwarding
- ✅ Add **multiple port mappings** at once
- ✅ Friendly **console output** with colorized formatting

---

## 🛠️ Installation

```bash
go install github.com/IIpho3nix/PortUP@latest
```

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

MIT License. See [LICENSE](LICENSE) for details.
