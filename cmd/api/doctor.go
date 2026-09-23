package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/handlers"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
)

var doctorTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type doctorReport struct {
	Version string        `json:"version"`
	OK      bool          `json:"ok"`
	Checks  []doctorCheck `json:"checks"`
}

// runDoctor validates the runtime environment and data files without
// starting the server. Stores are opened through the real constructors,
// so a clean doctor means the server will boot.
func runDoctor(args []string) error {
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--json", "-json":
			asJSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("doctor: unknown flag %q", arg)
			}
		}
	}

	checks := []doctorCheck{}
	pass := func(name, detail string) {
		checks = append(checks, doctorCheck{Name: name, OK: true, Detail: detail})
	}
	fail := func(name, detail string) {
		checks = append(checks, doctorCheck{Name: name, OK: false, Detail: detail})
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		fail("config", err.Error())
	} else {
		pass("config", "environment is valid")
	}

	token := os.Getenv("API_TOKEN")
	tokenSource := "env"
	if token == "" && os.Getenv("API_TOKEN_FILE") != "" {
		tokenSource = "file"
		if raw, err := os.ReadFile(os.Getenv("API_TOKEN_FILE")); err != nil {
			fail("token-file", err.Error())
		} else {
			token = strings.TrimSpace(string(raw))
		}
	}
	switch {
	case token == "":
		fail("token", "API_TOKEN is required")
	case len(token) < 16 || !doctorTokenPattern.MatchString(token):
		fail("token", "token must hold 16+ letters, numbers, underscores, or hyphens")
	default:
		pass("token", fmt.Sprintf("%d chars, from %s", len(token), tokenSource))
	}

	if _, err := net.ParseMAC(os.Getenv("DEFAULT_MAC")); err != nil {
		fail("default-mac", "DEFAULT_MAC must be a valid MAC address")
	} else {
		pass("default-mac", os.Getenv("DEFAULT_MAC"))
	}

	devicesFile := pathOr("DEVICES_FILE", "devices.json")
	if store, err := storage.New(devicesFile); err != nil {
		fail("devices-file", err.Error())
	} else {
		pass("devices-file", fmt.Sprintf("%s holds %d devices", devicesFile, len(store.GetAll())))
	}
	historyFile := pathOr("HISTORY_FILE", "wake-history.json")
	if hist, err := history.New(historyFile); err != nil {
		fail("history-file", err.Error())
	} else {
		pass("history-file", fmt.Sprintf("%s holds %d entries", historyFile, hist.Stats().Size))
	}
	schedulesFile := pathOr("SCHEDULES_FILE", "schedules.json")
	if sched, err := scheduler.NewStore(schedulesFile); err != nil {
		fail("schedules-file", err.Error())
	} else {
		pass("schedules-file", fmt.Sprintf("%s holds %d schedules", schedulesFile, len(sched.All())))
	}
	ignoreFile := pathOr("IGNORED_FILE", "ignored.json")
	if il, err := discover.LoadIgnoreList(ignoreFile); err != nil {
		fail("ignore-file", err.Error())
	} else {
		pass("ignore-file", fmt.Sprintf("%s holds %d entries", ignoreFile, len(il.List())))
	}

	for _, bin := range []string{"ping", "arp"} {
		if path, err := exec.LookPath(bin); err != nil {
			fail("binary-"+bin, "not in PATH, discovery and ping fall back to TCP")
		} else {
			pass("binary-"+bin, path)
		}
	}

	port := "8080"
	if cfg != nil {
		port = cfg.Port
	} else if v := os.Getenv("PORT"); v != "" {
		port = v
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err != nil {
		pass("port", fmt.Sprintf("port %s is free", port))
	} else {
		_ = conn.Close()
		pass("port", fmt.Sprintf("port %s answers, a server may already run", port))
	}

	webhooks := []string{}
	if os.Getenv("STATUS_WEBHOOK_URL") != "" {
		webhooks = append(webhooks, "status")
	}
	if os.Getenv("WAKE_WEBHOOK_URL") != "" {
		webhooks = append(webhooks, "wake")
	}
	if len(webhooks) == 0 {
		pass("webhooks", "none configured")
	} else {
		pass("webhooks", "configured: "+strings.Join(webhooks, ", "))
	}

	report := doctorReport{Version: handlers.Version, OK: true, Checks: checks}
	for _, c := range checks {
		if !c.OK {
			report.OK = false
			break
		}
	}
	if asJSON {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("doctor: encode: %w", err)
		}
		fmt.Println(string(raw))
	} else {
		fmt.Printf("hello doctor (version %s)\n", report.Version)
		for _, c := range checks {
			mark := "ok"
			if !c.OK {
				mark = "FAIL"
			}
			fmt.Printf("[%s] %s: %s\n", mark, c.Name, c.Detail)
		}
	}
	if !report.OK {
		return fmt.Errorf("doctor: %d failing checks", countFailed(checks))
	}
	return nil
}

func countFailed(checks []doctorCheck) int {
	n := 0
	for _, c := range checks {
		if !c.OK {
			n++
		}
	}
	return n
}

func pathOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
