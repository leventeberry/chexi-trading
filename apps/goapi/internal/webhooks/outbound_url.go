// Outbound URL validation mitigates SSRF by blocking private/link-local/metadata-style
// targets and by resolving hostnames at validation time. DNS TTL can still change
// addresses before delivery; we re-validate at delivery time to reduce (not eliminate)
// rebinding risk. HTTP redirects are disabled on the webhook client so a "safe" first
// hop cannot redirect into an internal network. True IP pinning would require a custom
// Dial (future hardening).
package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"goapi/config"
)

// LookupHostFunc resolves a hostname to IP address strings (typically net.DefaultResolver.LookupHost).
type LookupHostFunc func(ctx context.Context, host string) ([]string, error)

// ValidateOutboundWebhookURL enforces scheme, rejects embedded credentials, resolves DNS when needed,
// and blocks non–globally-routable addresses except the narrow development/test HTTP+loopback case.
func ValidateOutboundWebhookURL(ctx context.Context, cfg *config.Config, raw string, lookup LookupHostFunc) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if lookup == nil {
		return errors.New("lookup is nil")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("missing scheme or host")
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", scheme)
	}
	if hasURLUserinfo(u) {
		return errors.New("url must not contain embedded credentials")
	}

	env := config.NormalizeEnvironment(cfg.Environment)
	isStagingProd := config.IsStagingOrProductionEnvironment(env)
	isDevTest := env == config.EnvironmentDevelopment || env == config.EnvironmentTest

	if isStagingProd && scheme != "https" {
		return errors.New("staging and production require https webhook URLs")
	}
	if isDevTest && scheme == "http" {
		if !allowedDevHTTPLoopbackHost(u.Hostname()) {
			return errors.New("http webhook URLs are limited to localhost or loopback IPs in development/test")
		}
	}
	if !isDevTest && scheme != "https" {
		return errors.New("https required for webhook URL")
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("missing host")
	}

	addrs, err := resolvedAddrs(ctx, host, lookup)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return errors.New("host resolved to no addresses")
	}

	allowLoopback := isDevTest && scheme == "http" && allowedDevHTTPLoopbackHost(host)
	for _, a := range addrs {
		if isBlockedOutboundIP(a, allowLoopback) {
			return fmt.Errorf("address %s is not an allowed webhook target", a)
		}
	}
	return nil
}

func hasURLUserinfo(u *url.URL) bool {
	if u == nil || u.User == nil {
		return false
	}
	username := u.User.Username()
	_, hasPassword := u.User.Password()
	return username != "" || hasPassword
}

// allowedDevHTTPLoopbackHost is true for localhost or a literal loopback IP in the host field.
func allowedDevHTTPLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" {
		return true
	}
	addr, ok := parseHostAsIP(h)
	if !ok {
		return false
	}
	return addr.IsLoopback()
}

func parseHostAsIP(host string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

// resolvedAddrs returns IP addresses for host: literal IP, or DNS A/AAAA via lookup.
func resolvedAddrs(ctx context.Context, host string, lookup LookupHostFunc) ([]netip.Addr, error) {
	if addr, ok := parseHostAsIP(host); ok {
		return []netip.Addr{addr}, nil
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, s := range ips {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("parse resolved ip %q: %w", s, err)
		}
		out = append(out, addr)
	}
	return out, nil
}

func isBlockedOutboundIP(addr netip.Addr, allowLoopback bool) bool {
	if !addr.IsValid() {
		return true
	}
	if addr.IsUnspecified() {
		return true
	}
	if addr.IsMulticast() {
		return true
	}
	if addr.IsLoopback() {
		return !allowLoopback
	}
	if addr.IsPrivate() {
		return true
	}
	if addr.IsLinkLocalUnicast() {
		return true
	}
	if !addr.IsGlobalUnicast() {
		return true
	}
	return false
}

// DefaultLookupHost wraps net.DefaultResolver.LookupHost as LookupHostFunc.
func DefaultLookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}
