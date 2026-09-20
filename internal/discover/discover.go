package discover

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrScanBusy     = errors.New("a scan is already running")
	ErrCoolingDown  = errors.New("scan cooling down, try again later")
	ErrSubnetLarge  = errors.New("subnet too large to scan, PIN Subnet to a /24 or smaller")
	defaultCooldown = 30 * time.Second
	dialTimeout     = 300 * time.Millisecond
	maxHosts        = 1024
)

// Host is one discovered box. Empty MAC means the box answered a probe but
// left no ARP trace, so it cannot be adopted as a device yet.
type Host struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname,omitempty"`
	Known    bool   `json:"known"`
}

// Scanner probes the local subnet and reads back the OS ARP table. Zero
// value is useless, build one with New. Subnet pins the range, nil means
// auto-detect the first non-loopback IPv4 interface.
type Scanner struct {
	Subnet          *net.IPNet
	ProbePorts      []int
	ResolveHostname func(ip string) string

	cooldown time.Duration
	sweep    func(ctx context.Context, subnet *net.IPNet) map[string]struct{}
	readARP  func(ctx context.Context) map[string]string

	mu       sync.Mutex
	scanning bool
	lastScan time.Time
}

// New returns a Scanner probing common ports with a 30s cooldown.
func New() *Scanner {
	s := &Scanner{
		ProbePorts:      []int{22, 80, 443, 445, 8080},
		ResolveHostname: reverseDNS,
		cooldown:        defaultCooldown,
	}
	s.sweep = s.tcpSweep
	s.readARP = runARP
	return s
}

// Scan probes the subnet then maps live IPs to MACs via the ARP table. It
// refuses concurrent runs and enforces the cooldown, so callers can map both
// failures to 429.
func (s *Scanner) Scan(ctx context.Context) ([]Host, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrScanBusy
	}
	if since := time.Since(s.lastScan); since < s.cooldown {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: retry in %ds", ErrCoolingDown, int((s.cooldown-since).Seconds())+1)
	}
	s.scanning = true
	s.lastScan = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	subnet := s.Subnet
	if subnet == nil {
		var err error
		subnet, err = detectSubnet()
		if err != nil {
			return nil, err
		}
	}
	if hosts := countHosts(subnet); hosts > maxHosts {
		return nil, fmt.Errorf("%w (%d hosts)", ErrSubnetLarge, hosts)
	}

	open := s.sweep(ctx, subnet)
	byMAC := s.readARP(ctx)

	merged := make(map[string]*Host, len(open)+len(byMAC))
	for ip := range open {
		merged[ip] = &Host{IP: ip, MAC: byMAC[ip]}
	}
	for ip, mac := range byMAC {
		if h, ok := merged[ip]; ok {
			h.MAC = mac
		} else {
			merged[ip] = &Host{IP: ip, MAC: mac}
		}
	}
	if local := localEntry(subnet); local != nil {
		if h, ok := merged[local.IP]; ok {
			if h.MAC == "" {
				h.MAC = local.MAC
			}
		} else {
			_, probed := open[local.IP]
			if local.MAC != "" || probed {
				merged[local.IP] = local
			}
		}
	}

	hosts := make([]Host, 0, len(merged))
	for _, h := range merged {
		if name := s.ResolveHostname(h.IP); name != "" {
			h.Hostname = name
		}
		hosts = append(hosts, *h)
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].IP < hosts[j].IP })
	return hosts, nil
}

// RetryIn reports how long before another scan is accepted, zero when idle.
func (s *Scanner) RetryIn() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scanning {
		return 0
	}
	if left := s.cooldown - time.Since(s.lastScan); left > 0 {
		return left
	}
	return 0
}

// tcpSweep dials every subnet IP on every probe port, returning the IPs with
// at least one open port. Refused dials fail fast, filtered ones burn the
// timeout, so callers should keep subnets small.
func (s *Scanner) tcpSweep(ctx context.Context, subnet *net.IPNet) map[string]struct{} {
	open := make(map[string]struct{})
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 128)
	for _, ip := range subnetIPs(subnet) {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			for _, port := range s.ProbePorts {
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				d := net.Dialer{Timeout: dialTimeout}
				conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
				<-sem
				if err == nil {
					_ = conn.Close()
					mu.Lock()
					open[ip] = struct{}{}
					mu.Unlock()
					return
				}
			}
		}(ip)
	}
	wg.Wait()
	return open
}

var arpLine = regexp.MustCompile(`\(([0-9.]+)\) at ([0-9a-fA-F:-]{17})`)

// parseARPTable maps IP to MAC from `arp -a` output, Darwin or Linux shape.
// Entries without a parseable MAC, like <incomplete>, are dropped.
func parseARPTable(out string) map[string]string {
	found := make(map[string]string)
	for _, m := range arpLine.FindAllStringSubmatch(out, -1) {
		mac, err := net.ParseMAC(strings.ReplaceAll(m[2], "-", ":"))
		if err != nil {
			continue
		}
		found[m[1]] = mac.String()
	}
	return found
}

// runARP dumps the OS ARP table, best effort: an empty map on any failure,
// since a bare sweep still yields open IPs.
func runARP(ctx context.Context) map[string]string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "arp", "-a").Output()
	if err != nil {
		return map[string]string{}
	}
	return parseARPTable(string(out))
}

// detectSubnet returns the /24 around the first non-loopback IPv4 interface
// address, keeping scans inside one broadcast domain by design.
func detectSubnet() (*net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		_, small, err := net.ParseCIDR(ipnet.IP.String() + "/24")
		if err != nil {
			continue
		}
		return small, nil
	}
	_, loop, _ := net.ParseCIDR("127.0.0.1/24")
	return loop, nil
}

// localEntry returns the interface address inside the subnet with its
// hardware address, so the scanning box lists itself even when silent.
func localEntry(subnet *net.IPNet) *Host {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil || !subnet.Contains(ipnet.IP) {
				continue
			}
			return &Host{IP: ipnet.IP.String(), MAC: iface.HardwareAddr.String()}
		}
	}
	return nil
}

func reverseDNS(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

func countHosts(subnet *net.IPNet) int {
	ones, bits := subnet.Mask.Size()
	if bits-ones >= 31 {
		return 1 << (uint(bits) - uint(ones))
	}
	return (1 << (uint(bits) - uint(ones))) - 2
}

// subnetIPs lists scannable addresses, skipping network and broadcast except
// on tiny subnets where every address counts.
func subnetIPs(subnet *net.IPNet) []string {
	ones, bits := subnet.Mask.Size()
	base := subnet.IP.To4()
	if base == nil {
		return nil
	}
	total := 1 << (uint(bits) - uint(ones))
	ips := make([]string, 0, total)
	start, end := 1, total-1
	if ones >= 31 {
		start, end = 0, total
	}
	for i := start; i < end; i++ {
		ip := make(net.IP, 4)
		copy(ip, base)
		carry := i
		for k := 3; k >= 0 && carry > 0; k-- {
			sum := int(ip[k]) + carry
			ip[k] = byte(sum & 0xff)
			carry = sum >> 8
		}
		ips = append(ips, ip.String())
	}
	return ips
}
