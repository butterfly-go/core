package store

// These tests start real Redis, PostgreSQL, MySQL and MongoDB servers with
// testcontainers, so the health checks are exercised on their success path as
// well as their failure path. They need a running Docker daemon and are
// skipped when there is none, or with -short.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"butterfly.orx.me/core/internal/health"
	"butterfly.orx.me/core/internal/tlstest"
	"butterfly.orx.me/core/mod"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	redisImage    = "redis:7-alpine"
	postgresImage = "postgres:16-alpine"
	mysqlImage    = "mysql:8.4"
	mongoImage    = "mongo:7"

	// A password full of URL-reserved characters: it must survive being
	// embedded in a DSN.
	trickyPassword = "p@ss/w:rd#1?&x"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping container tests in -short mode")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)
}

func startContainer(t *testing.T, run func(context.Context) (testcontainers.Container, error)) testcontainers.Container {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := run(ctx)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	return c
}

func stopContainer(t *testing.T, c testcontainers.Container) {
	t.Helper()
	timeout := 5 * time.Second
	if err := c.Stop(context.Background(), &timeout); err != nil {
		t.Fatalf("stop container: %v", err)
	}
}

func hostPort(t *testing.T, c testcontainers.Container, port string) (string, int) {
	t.Helper()
	ctx := context.Background()
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.MappedPort(ctx, port)
	if err != nil {
		t.Fatal(err)
	}
	return host, int(p.Num())
}

func findCheck(t *testing.T, name string) health.Check {
	t.Helper()
	for _, c := range HealthChecks() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no health check named %q in %v", name, checkNames())
	return health.Check{}
}

func checkNames() []string {
	var names []string
	for _, c := range HealthChecks() {
		names = append(names, c.Name)
	}
	return names
}

func runCheck(c health.Check) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Check(ctx)
}

func requireHealthy(t *testing.T, name string) {
	t.Helper()
	if err := runCheck(findCheck(t, name)); err != nil {
		t.Fatalf("%s should be healthy: %v", name, err)
	}
}

func requireUnhealthy(t *testing.T, name string) {
	t.Helper()
	if err := runCheck(findCheck(t, name)); err == nil {
		t.Fatalf("%s should be unhealthy after its server stopped", name)
	}
}

func TestHealthCheckRedisSuccess(t *testing.T) {
	requireDocker(t)
	c := startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return tcredis.Run(ctx, redisImage)
	})
	host, port := hostPort(t, c, "6379/tcp")
	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"cache": {
			Addr:         fmt.Sprintf("%s:%d", host, port),
			PoolSize:     7,
			MinIdleConns: 2,
			DialTimeout:  2 * time.Second,
		},
	}})
	if err := InitRedis(); err != nil {
		t.Fatalf("InitRedis: %v", err)
	}

	requireHealthy(t, "redis/cache")
	client := GetRedisClient("cache")
	if client == nil {
		t.Fatal("GetRedisClient returned nil")
	}
	if got := client.Options().PoolSize; got != 7 {
		t.Fatalf("PoolSize = %d, want 7", got)
	}
	ctx := context.Background()
	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Get(ctx, "k").Result(); err != nil || got != "v" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	stopContainer(t, c)
	requireUnhealthy(t, "redis/cache")
}

func TestRedisPassword(t *testing.T) {
	requireDocker(t)
	const password = "redis-secret-pw"
	logs := captureLogs(t)
	c := startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        redisImage,
				Cmd:          []string{"redis-server", "--requirepass", password},
				ExposedPorts: []string{"6379/tcp"},
				WaitingFor:   wait.ForLog("Ready to accept connections"),
			},
			Started: true,
		})
	})
	host, port := hostPort(t, c, "6379/tcp")
	addr := fmt.Sprintf("%s:%d", host, port)

	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"cache": {Addr: addr, Password: password},
	}})
	if err := InitRedis(); err != nil {
		t.Fatalf("InitRedis with the right password: %v", err)
	}
	requireHealthy(t, "redis/cache")
	_ = Close(context.Background())

	const wrong = "not-the-password"
	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"cache": {Addr: addr, Password: wrong},
	}})
	err := InitRedis()
	if err == nil {
		t.Fatal("InitRedis with the wrong password should fail")
	}
	for _, secret := range []string{password, wrong} {
		if strings.Contains(err.Error(), secret) || strings.Contains(logs.String(), secret) {
			t.Fatalf("password %q leaked:\nerror: %v\nlogs:\n%s", secret, err, logs.String())
		}
	}
}

// redisTLSContainer runs Redis with TLS only, demanding client certificates
// when mutual is true.
func redisTLSContainer(t *testing.T, b tlstest.Bundle, mutual bool) testcontainers.Container {
	t.Helper()
	cmd := []string{
		"redis-server", "--port", "0", "--tls-port", "6379",
		"--tls-cert-file", "/tls/server.pem", "--tls-key-file", "/tls/server-key.pem", "--tls-ca-cert-file", "/tls/ca.pem",
		"--tls-auth-clients", "no",
	}
	if mutual {
		cmd[len(cmd)-1] = "yes"
	}
	file := func(host, name string) testcontainers.ContainerFile {
		return testcontainers.ContainerFile{HostFilePath: host, ContainerFilePath: "/tls/" + name, FileMode: 0o644}
	}
	return startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        redisImage,
				Cmd:          cmd,
				ExposedPorts: []string{"6379/tcp"},
				Files: []testcontainers.ContainerFile{
					file(b.ServerCertFile, "server.pem"), file(b.ServerKeyFile, "server-key.pem"), file(b.CAFile, "ca.pem"),
				},
				WaitingFor: wait.ForLog("Ready to accept connections"),
			},
			Started: true,
		})
	})
}

func TestRedisTLS(t *testing.T) {
	requireDocker(t)
	b := tlstest.New(t)
	c := redisTLSContainer(t, b, false)
	host, port := hostPort(t, c, "6379/tcp")
	addr := fmt.Sprintf("127.0.0.1:%d", port) // the certificate covers 127.0.0.1 and localhost
	_ = host

	// Plaintext against a TLS-only port must not work.
	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"plain": {Addr: addr, DialTimeout: time.Second},
	}})
	if err := InitRedis(); err == nil {
		t.Fatal("plaintext connection to a TLS-only server should fail")
	}

	// Trusting the CA works.
	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"secure": {Addr: addr, TLS: mod.TLSConfig{Enabled: true, CAFile: b.CAFile}},
	}})
	if err := InitRedis(); err != nil {
		t.Fatalf("InitRedis over TLS: %v", err)
	}
	requireHealthy(t, "redis/secure")

	// Without the CA the server certificate is rejected.
	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"untrusted": {Addr: addr, DialTimeout: time.Second, TLS: mod.TLSConfig{Enabled: true}},
	}})
	if err := InitRedis(); err == nil {
		t.Fatal("an untrusted server certificate should fail verification")
	}
}

func TestRedisMutualTLS(t *testing.T) {
	requireDocker(t)
	b := tlstest.New(t)
	c := redisTLSContainer(t, b, true)
	_, port := hostPort(t, c, "6379/tcp")
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"nocert": {Addr: addr, DialTimeout: time.Second, TLS: mod.TLSConfig{Enabled: true, CAFile: b.CAFile}},
	}})
	if err := InitRedis(); err == nil {
		t.Fatal("a client without a certificate should be rejected")
	}

	withCoreStore(t, mod.StoreConfig{Redis: map[string]mod.RedisConfig{
		"mtls": {Addr: addr, TLS: mod.TLSConfig{Enabled: true, CAFile: b.CAFile, CertFile: b.ClientCertFile, KeyFile: b.ClientKeyFile}},
	}})
	if err := InitRedis(); err != nil {
		t.Fatalf("InitRedis with a client certificate: %v", err)
	}
	requireHealthy(t, "redis/mtls")
}

func TestHealthCheckSQLSuccess(t *testing.T) {
	requireDocker(t)

	pg := startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return tcpostgres.Run(ctx, postgresImage,
			tcpostgres.WithDatabase("app"), tcpostgres.WithUsername("app"), tcpostgres.WithPassword(trickyPassword),
			tcpostgres.BasicWaitStrategies())
	})
	my := startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return tcmysql.Run(ctx, mysqlImage,
			tcmysql.WithDatabase("app"), tcmysql.WithUsername("app"), tcmysql.WithPassword(trickyPassword))
	})
	pgHost, pgPort := hostPort(t, pg, "5432/tcp")
	myHost, myPort := hostPort(t, my, "3306/tcp")

	withCoreStore(t, mod.StoreConfig{DB: map[string]mod.DBConfig{
		"pg":    {Driver: "postgres", Host: pgHost, Port: pgPort, User: "app", Password: trickyPassword, DBName: "app"},
		"mysql": {Driver: "mysql", Host: myHost, Port: myPort, User: "app", Password: trickyPassword, DBName: "app"},
	}})
	if err := InitSQLDB(); err != nil {
		t.Fatalf("InitSQLDB: %v", err)
	}
	if err := InitGORM(); err != nil {
		t.Fatalf("InitGORM: %v", err)
	}

	requireHealthy(t, "sql/pg")
	requireHealthy(t, "sql/mysql")

	// The pools are real, not just pingable.
	for name, query := range map[string]string{"pg": "SELECT 1", "mysql": "SELECT 1"} {
		var n int
		if err := GetSQLDB(name).QueryRow(query).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s: SELECT 1 = %d, %v", name, n, err)
		}
	}

	stopContainer(t, pg)
	requireUnhealthy(t, "sql/pg")
	requireHealthy(t, "sql/mysql") // one failing dependency does not affect the other
	stopContainer(t, my)
	requireUnhealthy(t, "sql/mysql")
}

func TestHealthCheckMongoSuccess(t *testing.T) {
	requireDocker(t)
	c := startContainer(t, func(ctx context.Context) (testcontainers.Container, error) {
		return tcmongo.Run(ctx, mongoImage)
	})
	uri, err := c.(*tcmongo.MongoDBContainer).ConnectionString(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	withCoreStore(t, mod.StoreConfig{Mongo: map[string]mod.MongoConfig{
		"primary": {
			URI:                    uri,
			MaxPoolSize:            20,
			MinPoolSize:            1,
			ConnectTimeout:         3 * time.Second,
			ServerSelectionTimeout: time.Second,
		},
	}})
	if err := InitMongo(); err != nil {
		t.Fatalf("InitMongo: %v", err)
	}

	requireHealthy(t, "mongo/primary")
	client := GetMongoClients("primary")
	if client == nil {
		t.Fatal("GetMongoClients returned nil")
	}
	coll := client.Database("app").Collection("things")
	if _, err := coll.InsertOne(context.Background(), map[string]any{"a": 1}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	stopContainer(t, c)
	requireUnhealthy(t, "mongo/primary")
}

// TestHealthChecksAllStores wires Redis, SQL and Mongo through the public Init
// path and checks that HealthChecks reports all of them healthy together.
func TestHealthChecksAllStores(t *testing.T) {
	requireDocker(t)

	var (
		wg                  sync.WaitGroup
		rc, pg, mc          testcontainers.Container
		rcErr, pgErr, mcErr error
	)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	wg.Add(3)
	go func() { defer wg.Done(); rc, rcErr = tcredis.Run(ctx, redisImage) }()
	go func() {
		defer wg.Done()
		pg, pgErr = tcpostgres.Run(ctx, postgresImage, tcpostgres.WithDatabase("app"),
			tcpostgres.WithUsername("app"), tcpostgres.WithPassword("pw"), tcpostgres.BasicWaitStrategies())
	}()
	go func() { defer wg.Done(); mc, mcErr = tcmongo.Run(ctx, mongoImage) }()
	wg.Wait()
	for _, c := range []testcontainers.Container{rc, pg, mc} {
		testcontainers.CleanupContainer(t, c)
	}
	for _, err := range []error{rcErr, pgErr, mcErr} {
		if err != nil {
			t.Fatalf("start container: %v", err)
		}
	}

	rHost, rPort := hostPort(t, rc, "6379/tcp")
	pHost, pPort := hostPort(t, pg, "5432/tcp")
	mongoURI, err := mc.(*tcmongo.MongoDBContainer).ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	withCoreStore(t, mod.StoreConfig{
		Redis: map[string]mod.RedisConfig{"cache": {Addr: fmt.Sprintf("%s:%d", rHost, rPort)}},
		DB:    map[string]mod.DBConfig{"main": {Driver: "postgres", Host: pHost, Port: pPort, User: "app", Password: "pw", DBName: "app"}},
		Mongo: map[string]mod.MongoConfig{"docs": {URI: mongoURI}},
	})
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	checks := HealthChecks()
	want := []string{"redis/cache", "sql/main", "mongo/docs"}
	if got := checkNames(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("checks = %v, want %v", got, want)
	}

	// Run them the way the readiness endpoint does.
	reg := health.New()
	if err := reg.Add(checks...); err != nil {
		t.Fatal(err)
	}
	reg.SetState(health.Ready)
	report := reg.Readiness(context.Background())
	if !report.OK() {
		t.Fatalf("readiness report = %+v, want all checks ok", report)
	}
	for _, name := range want {
		if report.Checks[name].Status != health.StatusOK {
			t.Errorf("%s: %+v", name, report.Checks[name])
		}
	}

	stopContainer(t, mc)
	reg2 := health.New(health.WithCacheTTL(0))
	_ = reg2.Add(HealthChecks()...)
	reg2.SetState(health.Ready)
	report = reg2.Readiness(context.Background())
	if report.OK() || report.Checks["mongo/docs"].Status != health.StatusFail ||
		report.Checks["redis/cache"].Status != health.StatusOK || report.Checks["sql/main"].Status != health.StatusOK {
		t.Fatalf("after stopping mongo: %+v", report)
	}
}
