# DNS Manager & Proxy 🚀

A Go-based DNS server with a Web UI for real-time configuration.

## 📋 Features
- **Local DNS Overrides:** Map custom domains to local IPs.
- **Upstream Proxying:** Forwards unknown queries to Google DNS (8.8.8.8).
- **Web UI:** Manage port, upstream, and blacklist via browser.
- **Auto-Restart:** DNS server restarts automatically when config changes.

## 🛠 Deployment Instructions

### 1. Build the Binary
```bash
go build -o dns-manager main.go
