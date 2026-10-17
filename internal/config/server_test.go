package config

import (
	"strings"
	"testing"
	"time"

	"butterfly.orx.me/core/mod"
	"gopkg.in/yaml.v3"
)

func srvCfg(httpAddr, grpcAddr, adminAddr string) mod.ServerConfig {
	return mod.ServerConfig{
		HTTP:  mod.ListenConfig{Addr: httpAddr},
		GRPC:  mod.ListenConfig{Addr: grpcAddr},
		Admin: mod.ListenConfig{Addr: adminAddr},
	}
}

func TestResolveServerAddrs(t *testing.T) {
	tests := []struct {
		name        string
		cfg         mod.ServerConfig
		port        string
		includeGRPC bool
		want        ServerAddrs
		wantErr     string
	}{
		{
			name:        "defaults",
			includeGRPC: true,
			want:        ServerAddrs{HTTP: ":8080", GRPC: ":9090", Admin: ":2223"},
		},
		{
			name:        "PORT env sets http",
			port:        "3000",
			includeGRPC: true,
			want:        ServerAddrs{HTTP: ":3000", GRPC: ":9090", Admin: ":2223"},
		},
		{
			name:        "yaml beats PORT",
			cfg:         srvCfg("127.0.0.1:8081", "", ""),
			port:        "3000",
			includeGRPC: true,
			want:        ServerAddrs{HTTP: "127.0.0.1:8081", GRPC: ":9090", Admin: ":2223"},
		},
		{
			name:        "all overridden",
			cfg:         srvCfg(":18080", ":19090", ":12223"),
			includeGRPC: true,
			want:        ServerAddrs{HTTP: ":18080", GRPC: ":19090", Admin: ":12223"},
		},
		{
			name:        "whitespace is trimmed",
			cfg:         srvCfg("  :18080 ", "", ""),
			includeGRPC: true,
			want:        ServerAddrs{HTTP: ":18080", GRPC: ":9090", Admin: ":2223"},
		},
		{
			name: "PORT equal to unused grpc default is fine",
			port: "9090",
			want: ServerAddrs{HTTP: ":9090", GRPC: ":9090", Admin: ":2223"},
		},
		{
			name:        "PORT equal to grpc default conflicts when grpc is used",
			port:        "9090",
			includeGRPC: true,
			wantErr:     "server.http.addr \":9090\" conflicts with server.grpc.addr \":9090\"",
		},
		{
			name:    "http and admin conflict",
			cfg:     srvCfg(":2223", "", ""),
			wantErr: "conflicts with server.admin.addr",
		},
		{
			name:    "wildcard conflicts with specific host",
			cfg:     srvCfg("127.0.0.1:7000", "", "0.0.0.0:7000"),
			wantErr: "conflicts",
		},
		{
			name: "same port on different hosts is fine",
			cfg:  srvCfg("127.0.0.1:7000", "", "127.0.0.2:7000"),
			want: ServerAddrs{HTTP: "127.0.0.1:7000", GRPC: ":9090", Admin: "127.0.0.2:7000"},
		},
		{
			name:        "port zero never conflicts",
			cfg:         srvCfg(":0", ":0", ":0"),
			includeGRPC: true,
			want:        ServerAddrs{HTTP: ":0", GRPC: ":0", Admin: ":0"},
		},
		{
			name:    "missing port",
			cfg:     srvCfg("localhost", "", ""),
			wantErr: "server.http.addr \"localhost\"",
		},
		{
			name:    "non numeric port",
			cfg:     srvCfg("", "", ":http"),
			wantErr: "server.admin.addr",
		},
		{
			name:    "port out of range",
			cfg:     srvCfg(":70000", "", ""),
			wantErr: "invalid port",
		},
		{
			name:    "invalid PORT env",
			port:    "abc",
			wantErr: "server.http.addr \":abc\"",
		},
		{
			name:    "unused grpc addr is still validated",
			cfg:     srvCfg("", "bogus", ""),
			wantErr: "server.grpc.addr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveServerAddrs(tt.cfg, tt.port, tt.includeGRPC)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestServerConfigYAML(t *testing.T) {
	var cfg mod.CoreConfig
	in := "server:\n  http: {addr: \":18080\"}\n  shutdown_delay: 5s\n"
	if err := yaml.Unmarshal([]byte(in), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.HTTP.Addr != ":18080" || cfg.Server.ShutdownDelay != 5*time.Second {
		t.Fatalf("parsed = %+v", cfg.Server)
	}
}
