package security

import (
	"net"
	"net/http"

	"github.com/DongDuong2001/graft/internal/utils"
)

// CIDRAllowlist provides IP-based access control using CIDR notation.
// Supports IPv4 and IPv6 addresses and networks.
type CIDRAllowlist struct {
	allowedNets []*net.IPNet
	allowedIPs  map[string]bool
	trustProxy  bool
}

// NewCIDRAllowlist creates a new allowlist from a list of CIDRs or IPs.
// Supports formats like:
//   - "192.168.1.1" (single IP)
//   - "192.168.1.0/24" (CIDR range)
//   - "10.0.0.0/8" (private network)
//   - "::1/128" (IPv6 localhost)
func NewCIDRAllowlist(cidrs []string, trustForwardedFor bool) (*CIDRAllowlist, error) {
	ca := &CIDRAllowlist{
		allowedIPs: make(map[string]bool),
		trustProxy: trustForwardedFor,
	}

	for _, cidr := range cidrs {
		if err := ca.addCIDR(cidr); err != nil {
			return nil, err
		}
	}

	return ca, nil
}

// addCIDR parses and adds a CIDR or single IP to the allowlist.
func (ca *CIDRAllowlist) addCIDR(cidr string) error {
	// Try parsing as CIDR first
	_, ipNet, err := net.ParseCIDR(cidr)
	if err == nil {
		ca.allowedNets = append(ca.allowedNets, ipNet)
		return nil
	}

	// Try parsing as single IP
	ip := net.ParseIP(cidr)
	if ip == nil {
		return err
	}

	// Normalize to ensure consistent comparison
	ca.allowedIPs[ip.String()] = true
	return nil
}

// IsAllowed checks if the given IP is in the allowlist.
func (ca *CIDRAllowlist) IsAllowed(ipStr string) bool {
	if len(ca.allowedNets) == 0 && len(ca.allowedIPs) == 0 {
		return true // no restrictions
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check exact IP match
	if ca.allowedIPs[ip.String()] {
		return true
	}

	// Check CIDR ranges
	for _, ipNet := range ca.allowedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// Middleware returns an HTTP middleware that enforces the allowlist.
func (ca *CIDRAllowlist) Middleware(next http.Handler) http.Handler {
	// If no restrictions, return pass-through
	if len(ca.allowedNets) == 0 && len(ca.allowedIPs) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.ClientIP(r, ca.trustProxy)

		if !ca.IsAllowed(ip) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Update replaces the current allowlist with new CIDRs.
func (ca *CIDRAllowlist) Update(cidrs []string) error {
	ca.allowedNets = nil
	ca.allowedIPs = make(map[string]bool)

	for _, cidr := range cidrs {
		if err := ca.addCIDR(cidr); err != nil {
			return err
		}
	}
	return nil
}

// IsEmpty returns true if no restrictions are configured.
func (ca *CIDRAllowlist) IsEmpty() bool {
	return len(ca.allowedNets) == 0 && len(ca.allowedIPs) == 0
}
