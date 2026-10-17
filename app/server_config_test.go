package app

import (
	"testing"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/mod"
	"google.golang.org/grpc"
)

func TestInitServerResolvesCoreConfig(t *testing.T) {
	orig := config.CoreConfig().Server
	t.Cleanup(func() { config.CoreConfig().Server = orig })
	config.CoreConfig().Server = mod.ServerConfig{
		HTTP:  mod.ListenConfig{Addr: "127.0.0.1:18080"},
		Admin: mod.ListenConfig{Addr: "127.0.0.1:12223"},
	}
	t.Setenv("PORT", "3000") // yaml wins over $PORT

	a := New(&Config{Service: "server-config-test"})
	if err := a.initServer(); err != nil {
		t.Fatalf("initServer: %v", err)
	}
	want := config.ServerAddrs{HTTP: "127.0.0.1:18080", GRPC: ":9090", Admin: "127.0.0.1:12223"}
	if a.addrs != want {
		t.Fatalf("addrs = %+v, want %+v", a.addrs, want)
	}
}

func TestInitServerUsesPortEnvAndRejectsConflicts(t *testing.T) {
	orig := config.CoreConfig().Server
	t.Cleanup(func() { config.CoreConfig().Server = orig })
	config.CoreConfig().Server = mod.ServerConfig{}

	t.Setenv("PORT", "3000")
	a := New(&Config{Service: "server-config-test"})
	if err := a.initServer(); err != nil {
		t.Fatalf("initServer: %v", err)
	}
	if a.addrs.HTTP != ":3000" {
		t.Fatalf("http addr = %q, want :3000", a.addrs.HTTP)
	}

	// The default gRPC port only matters when a gRPC server is registered.
	t.Setenv("PORT", "9090")
	if err := New(&Config{Service: "s"}).initServer(); err != nil {
		t.Fatalf("PORT=9090 without grpc should be fine: %v", err)
	}
	withGRPC := New(&Config{Service: "s", GRPCRegister: func(_ *grpc.Server) {}})
	if err := withGRPC.initServer(); err == nil {
		t.Fatal("PORT=9090 with grpc registered should conflict")
	}
}
