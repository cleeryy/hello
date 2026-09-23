package discover

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IgnoreList is the persisted set of MACs and IPs the scanner hides (F-47).
// Entries are matched case-insensitively for MACs (canonical form) and
// exactly for IPs. The file stays 0600 and saves are atomic.
type IgnoreList struct {
	mu   sync.RWMutex
	file string
	macs map[string]struct{}
	ips  map[string]struct{}
}

// LoadIgnoreList opens path, starting empty when the file is missing and
// failing fast on corrupt content.
func LoadIgnoreList(path string) (*IgnoreList, error) {
	il := &IgnoreList{file: path, macs: map[string]struct{}{}, ips: map[string]struct{}{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return il, nil
		}
		return nil, fmt.Errorf("discover: read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return il, nil
	}
	var saved struct {
		MACs []string `json:"macs"`
		IPs  []string `json:"ips"`
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, fmt.Errorf("discover: decode %s: %w", path, err)
	}
	for _, rawMAC := range saved.MACs {
		mac, err := net.ParseMAC(strings.TrimSpace(rawMAC))
		if err != nil {
			return nil, fmt.Errorf("discover: decode %s: invalid mac %q", path, rawMAC)
		}
		il.macs[mac.String()] = struct{}{}
	}
	for _, rawIP := range saved.IPs {
		ip := strings.TrimSpace(rawIP)
		if net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("discover: decode %s: invalid ip %q", path, rawIP)
		}
		il.ips[ip] = struct{}{}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("discover: chmod %s: %w", path, err)
	}
	return il, nil
}

// Entry is one ignored address.
type Entry struct {
	MAC string `json:"mac,omitempty"`
	IP  string `json:"ip,omitempty"`
}

// List returns every ignored address, MACs first then IPs, both sorted.
func (il *IgnoreList) List() []Entry {
	il.mu.RLock()
	defer il.mu.RUnlock()
	out := make([]Entry, 0, len(il.macs)+len(il.ips))
	for mac := range il.macs {
		out = append(out, Entry{MAC: mac})
	}
	for ip := range il.ips {
		out = append(out, Entry{IP: ip})
	}
	return out
}

// Contains reports whether a MAC or IP is ignored.
func (il *IgnoreList) Contains(mac, ip string) bool {
	il.mu.RLock()
	defer il.mu.RUnlock()
	if mac != "" {
		if parsed, err := net.ParseMAC(strings.TrimSpace(mac)); err == nil {
			if _, ok := il.macs[parsed.String()]; ok {
				return true
			}
		}
	}
	if ip != "" {
		if _, ok := il.ips[strings.TrimSpace(ip)]; ok {
			return true
		}
	}
	return false
}

// Add stores one entry; at least one of mac or ip is required. It reports
// whether the entry is new.
func (il *IgnoreList) Add(mac, ip string) (bool, error) {
	var normMAC, normIP string
	if trimmed := strings.TrimSpace(mac); trimmed != "" {
		parsed, err := net.ParseMAC(trimmed)
		if err != nil {
			return false, fmt.Errorf("discover: invalid mac %q", mac)
		}
		normMAC = parsed.String()
	}
	if trimmed := strings.TrimSpace(ip); trimmed != "" {
		if net.ParseIP(trimmed) == nil {
			return false, fmt.Errorf("discover: invalid ip %q", ip)
		}
		normIP = trimmed
	}
	if normMAC == "" && normIP == "" {
		return false, fmt.Errorf("discover: mac or ip is required")
	}
	il.mu.Lock()
	defer il.mu.Unlock()
	created := false
	if normMAC != "" {
		if _, ok := il.macs[normMAC]; !ok {
			il.macs[normMAC] = struct{}{}
			created = true
		}
	}
	if normIP != "" {
		if _, ok := il.ips[normIP]; !ok {
			il.ips[normIP] = struct{}{}
			created = true
		}
	}
	if !created {
		return false, nil
	}
	if err := il.saveLocked(); err != nil {
		if normMAC != "" {
			delete(il.macs, normMAC)
		}
		if normIP != "" {
			delete(il.ips, normIP)
		}
		return false, err
	}
	return true, nil
}

// Remove drops one entry; it reports whether something was removed.
func (il *IgnoreList) Remove(mac, ip string) (bool, error) {
	il.mu.Lock()
	defer il.mu.Unlock()
	removed := false
	if trimmed := strings.TrimSpace(mac); trimmed != "" {
		if parsed, err := net.ParseMAC(trimmed); err == nil {
			if _, ok := il.macs[parsed.String()]; ok {
				delete(il.macs, parsed.String())
				removed = true
			}
		}
	}
	if trimmed := strings.TrimSpace(ip); trimmed != "" {
		if _, ok := il.ips[trimmed]; ok {
			delete(il.ips, trimmed)
			removed = true
		}
	}
	if !removed {
		return false, nil
	}
	if err := il.saveLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (il *IgnoreList) saveLocked() error {
	macs := make([]string, 0, len(il.macs))
	for mac := range il.macs {
		macs = append(macs, mac)
	}
	ips := make([]string, 0, len(il.ips))
	for ip := range il.ips {
		ips = append(ips, ip)
	}
	data, err := json.MarshalIndent(struct {
		MACs []string `json:"macs"`
		IPs  []string `json:"ips"`
	}{MACs: macs, IPs: ips}, "", "  ")
	if err != nil {
		return fmt.Errorf("discover: encode ignore list: %w", err)
	}
	dir := filepath.Dir(il.file)
	tmp, err := os.CreateTemp(dir, ".ignore-*.tmp")
	if err != nil {
		return fmt.Errorf("discover: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("discover: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("discover: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("discover: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("discover: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, il.file); err != nil {
		return fmt.Errorf("discover: replace %s: %w", il.file, err)
	}
	return nil
}
