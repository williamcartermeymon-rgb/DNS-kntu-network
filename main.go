package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"gopkg.in/ini.v1"
)

// Config structure for JSON communication with UI
type AppConfig struct {
	Port        int               `json:"port"`
	Upstream    string            `json:"upstream"`
	LocalDB     map[string]string `json:"local_db"`
	Blacklist   []string          `json:"blacklist"`
}

var (
	currentConfig AppConfig
	dnsServer     *dns.Server
	configMutex   sync.Mutex
	cancelDNS     context.CancelFunc
)

func loadConfig() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Printf("Ini not found, using defaults")
		return
	}
	currentConfig.Port = cfg.Section("server").Key("port").MustInt(5454)
	currentConfig.Upstream = cfg.Section("server").Key("upstream").MustString("8.8.8.8:53")
	
	currentConfig.LocalDB = make(map[string]string)
	for _, k := range cfg.Section("local_db").Keys() {
		currentConfig.LocalDB[k.Name()] = k.String()
	}

	blStr := cfg.Section("blacklist").Key("domains").String()
	currentConfig.Blacklist = strings.Split(blStr, ",")
}

func saveConfigToIni(c AppConfig) {
	cfg := ini.Empty()
	s, _ := cfg.NewSection("server")
	s.NewKey("port", fmt.Sprint(c.Port))
	s.NewKey("upstream", c.Upstream)

	l, _ := cfg.NewSection("local_db")
	for k, v := range c.LocalDB {
		l.NewKey(k, v)
	}

	b, _ := cfg.NewSection("blacklist")
	b.NewKey("domains", strings.Join(c.Blacklist, ","))
	cfg.SaveTo("config.ini")
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	if len(r.Question) == 0 { return }
	q := r.Question[0]

	configMutex.Lock()
	defer configMutex.Unlock()

	// 1. Blacklist
	for _, d := range currentConfig.Blacklist {
		if strings.TrimSpace(d) == q.Name {
			m.SetRcode(r, dns.RcodeRefused)
			w.WriteMsg(m)
			return
		}
	}

	// 2. Local DB
	if ip, ok := currentConfig.LocalDB[q.Name]; ok && q.Qtype == dns.TypeA {
		rr, _ := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", q.Name, ip))
		m.Answer = append(m.Answer, rr)
		w.WriteMsg(m)
		return
	}

	// 3. Forward
	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	res, _, err := c.Exchange(r, currentConfig.Upstream)
	if err == nil { w.WriteMsg(res) }
}

func startDNSServer() {
	configMutex.Lock()
	addr := fmt.Sprintf(":%d", currentConfig.Port)
	dnsServer = &dns.Server{Addr: addr, Net: "udp"}
	configMutex.Unlock()

	log.Printf("DNS Server starting on %s", addr)
	if err := dnsServer.ListenAndServe(); err != nil {
		log.Printf("DNS Server stopped: %s", err)
	}
}

func main() {
	loadConfig()

	// API Handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(currentConfig)
		} else if r.Method == "POST" {
			var newCfg AppConfig
			json.NewDecoder(r.Body).Decode(&newCfg)
			
			configMutex.Lock()
			currentConfig = newCfg
			saveConfigToIni(newCfg)
			if dnsServer != nil { dnsServer.Shutdown() } // Restart DNS
			configMutex.Unlock()

			go startDNSServer()
			w.Write([]byte("Config Applied and DNS Restarted"))
		}
	})

	// Start DNS in background
	dns.HandleFunc(".", handleDNSRequest)
	go startDNSServer()

	fmt.Println("UI available at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
