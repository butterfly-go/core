package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"butterfly.orx.me/core/mod"
)

// Default listen addresses of the built-in servers.
const (
	DefaultHTTPAddr  = ":8080"
	DefaultGRPCAddr  = ":9090"
	DefaultAdminAddr = ":2223"
)

// ServerAddrs are the resolved, validated listen addresses.
type ServerAddrs struct {
	HTTP  string
	GRPC  string
	Admin string
}

// DefaultServerAddrs returns the built-in defaults, ignoring $PORT.
func DefaultServerAddrs() ServerAddrs {
	return ServerAddrs{HTTP: DefaultHTTPAddr, GRPC: DefaultGRPCAddr, Admin: DefaultAdminAddr}
}

// ResolveServerAddrs applies defaults to cfg and validates the result.
//
// HTTP precedence is: server.http.addr, then port (the value of $PORT, which
// PaaS platforms and Kubernetes manifests commonly set), then ":8080".
// When includeGRPC is false the gRPC address is still validated for syntax but
// ignored when checking for conflicts, because no gRPC server will bind it.
func ResolveServerAddrs(cfg mod.ServerConfig, port string, includeGRPC bool) (ServerAddrs, error) {
	addrs := DefaultServerAddrs()
	if port = strings.TrimSpace(port); port != "" {
		addrs.HTTP = ":" + port
	}
	if v := strings.TrimSpace(cfg.HTTP.Addr); v != "" {
		addrs.HTTP = v
	}
	if v := strings.TrimSpace(cfg.GRPC.Addr); v != "" {
		addrs.GRPC = v
	}
	if v := strings.TrimSpace(cfg.Admin.Addr); v != "" {
		addrs.Admin = v
	}

	type named struct{ name, addr string }
	all := []named{{"http", addrs.HTTP}, {"grpc", addrs.GRPC}, {"admin", addrs.Admin}}
	for _, n := range all {
		if err := validateListenAddr(n.addr); err != nil {
			return ServerAddrs{}, fmt.Errorf("server.%s.addr %q: %w", n.name, n.addr, err)
		}
	}

	active := []named{all[0], all[2]}
	if includeGRPC {
		active = []named{all[0], all[1], all[2]}
	}
	for i := range active {
		for j := i + 1; j < len(active); j++ {
			if listenAddrsConflict(active[i].addr, active[j].addr) {
				return ServerAddrs{}, fmt.Errorf("server.%s.addr %q conflicts with server.%s.addr %q",
					active[i].name, active[i].addr, active[j].name, active[j].addr)
			}
		}
	}
	return addrs, nil
}

func validateListenAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("must be host:port, such as \":8080\": %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid port %q", port)
	}
	return nil
}

// listenAddrsConflict reports whether two valid addresses would fight over
// the same port. Port 0 never conflicts: each bind gets its own free port.
func listenAddrsConflict(a, b string) bool {
	hostA, portA, _ := net.SplitHostPort(a)
	hostB, portB, _ := net.SplitHostPort(b)
	nA, _ := strconv.Atoi(portA)
	nB, _ := strconv.Atoi(portB)
	if nA == 0 || nA != nB {
		return false
	}
	return hostA == hostB || isWildcardHost(hostA) || isWildcardHost(hostB)
}

func isWildcardHost(h string) bool {
	if h == "" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsUnspecified()
}
