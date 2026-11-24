// Command trackferry runs the TrackFerry home song-request service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"
	_ "time/tzdata" // the runtime image has no zoneinfo
	"unicode/utf8"

	"trackferry/internal/api"
	"trackferry/internal/auth"
	"trackferry/internal/config"
	"trackferry/internal/feiniu"
	"trackferry/internal/fscheck"
	"trackferry/internal/intent"
	"trackferry/internal/library"
	"trackferry/internal/live"
	"trackferry/internal/logx"
	"trackferry/internal/observe"
	"trackferry/internal/online"
	"trackferry/internal/relay"
	"trackferry/internal/routing"
	"trackferry/internal/store"
	"trackferry/internal/stream"
	"trackferry/internal/takeover"
	"trackferry/internal/update"
	"trackferry/internal/webui"
	"trackferry/internal/xiaomi"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "unknown"
	repo    = "madrays/trackferry" // where releases are published
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "relay":
		err = runRelay()
	case "healthcheck":
		err = healthcheck()
	case "setup-code":
		err = setupCode()
	case "reset-admin":
		err = resetAdmin()
	case "version":
		fmt.Printf("trackferry %s (%s)\n", version, commit)
	default:
		err = fmt.Errorf("unknown command %q (want serve, relay, healthcheck, setup-code, reset-admin or version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trackferry:", err)
		os.Exit(1)
	}
}

func configPath() string {
	if p := os.Getenv("TRACKFERRY_CONFIG"); p != "" {
		return p
	}
	return "/data/config.yaml"
}

func newLog() *slog.Logger {
	return logx.New(os.Stdout, os.Getenv("TRACKFERRY_LOG"), os.Getenv("TRACKFERRY_DEBUG") != "")
}

// writable explains the one thing that goes wrong on a first start: the folder Docker made
// belongs to root.
func writable(dir string) error {
	d := fscheck.CheckData(dir)
	if d.Writable {
		return nil
	}
	logx.Banner(os.Stderr, "启动不了：数据目录不可写",
		logx.Row{Name: "目录", Value: dir + "（compose.yaml 同级的 data）"},
		logx.Row{Name: "原因", Value: d.Error},
		logx.Row{Name: "解决", Value: fmt.Sprintf("在 compose.yaml 所在目录执行 sudo chown -R %d:%d data", os.Getuid(), os.Getgid())},
		logx.Row{Name: "然后", Value: "docker compose restart"})
	return errors.New("data directory is not writable: " + d.Error)
}

func serve() error {
	log := newLog()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}
	if err := writable(cfg.DataDir); err != nil {
		return err
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "trackferry.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	boot, err := st.RecordBoot(ctx, version, commit, time.Now())
	if err != nil {
		return err
	}
	token, err := api.LoadOrCreateToken(filepath.Join(cfg.DataDir, "private", "admin_token"))
	if err != nil {
		return err
	}

	private := filepath.Join(cfg.DataDir, "private")
	setupCodePath := filepath.Join(private, "setup_code")
	firstCode := ""
	// Deliberately logged: whoever can read the container log owns the install.
	showCode := func(code string) { logx.SetupCode(os.Stdout, code, cfg.PublicBaseURL) }
	if _, err := st.AdminCredential(ctx); errors.Is(err, store.ErrNotFound) {
		if firstCode, err = auth.LoadOrCreateSetupCode(setupCodePath); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	sink := func(ctx context.Context, found []xiaomi.Device) error {
		devs := make([]store.DiscoveredDevice, 0, len(found))
		for _, d := range found {
			devs = append(devs, store.DiscoveredDevice{DeviceID: d.DeviceID, Name: d.Name, Alias: d.Alias,
				Model: d.Model, Hardware: d.Hardware, MiotDID: d.MiotDID, Presence: d.Presence})
		}
		res, err := st.SyncDevices(ctx, devs, time.Now())
		if err == nil {
			log.Info("音箱　列表已保存", "新增", res.Added, "更新", res.Updated, "不见了", res.Missing)
		}
		return err
	}
	xm, err := xiaomi.NewManager(xiaomi.NewClient(xiaomi.DefaultEndpoints), filepath.Join(private, "xiaomi.json"), sink, log)
	if err != nil {
		return err
	}
	go xm.Run(ctx)
	hub := live.New()
	streams := stream.New(cfg.PublicBaseURL, cfg.MusicDir)
	streams.Log = log
	if streams.Ready() != nil {
		log.Warn("配置　没有填 TRACKFERRY_URL，音箱取不到歌，接管不会开始")
	}
	if err := st.FailRunningRuns(ctx, time.Now()); err != nil {
		return err
	}
	engine := takeover.New(xm, st, log, takeover.DefaultTiming)
	go engine.RecoverVolumes(ctx) // a crash mid-takeover must not leave a speaker muted
	lib := library.New(cfg.MusicDir, st, log)
	fn, err := feiniu.NewManager(filepath.Join(private, "feiniu.json"))
	if err != nil {
		return err
	}
	lib.Remote = fn
	streams.RemotePrefix = library.RemotePrefix
	streams.Remote = func(rel string) (stream.Opener, string) {
		t, ok := lib.ByPath(rel)
		if !ok || !fn.Ready() {
			return nil, ""
		}
		ext := t.Format
		if ext == "alac" {
			ext = "m4a"
		}
		return func(ctx context.Context, rng string) (*http.Response, error) { return fn.Open(ctx, t.GUID, rng) }, "." + ext
	}
	go lib.Run(ctx, 6*time.Hour)
	client := online.Client()
	sources := online.New(log, client, &online.Kuwo{Client: client}, &online.Kugou{Client: client}, &online.Migu{Client: client})
	router := &routing.Router{Library: lib, Online: sources, Log: log, Openable: streams.Openable}
	router.LoadSettings(ctx, st)
	speakerName := func(id string) string {
		d, err := st.Device(ctx, id)
		if err != nil {
			return ""
		}
		if d.Alias != "" {
			return d.Alias
		}
		return d.Name
	}
	dispatcher := &takeover.Dispatcher{Live: hub, Name: speakerName, Engine: engine, Store: st, Router: router, Streams: streams, Log: log,
		Allowed: func(id string) bool {
			name := speakerName(id)
			return name != "" && cfg.TakeoverAllowed(name)
		},
		Open: func(rt *routing.Route) stream.Opener { return sources.Opener(rt.Found()) }}
	obs := observe.New(xm, st, log, time.Second)
	obs.Live = hub
	dispatcher.Favorite = func(ctx context.Context, id int64, on bool) error {
		_, err := lib.SetFavorite(ctx, id, on)
		return err
	}
	obs.Follows = dispatcher.Follows
	obs.OnCommand(dispatcher.OnCommand)
	obs.OnOther = func(id string, r xiaomi.Record) {
		if slices.Contains(r.AnswerTypes, "AUDIO") {
			dispatcher.Forget(id)
		}
		engine.Hear(id, takeover.Heard{Stop: intent.Silences(r.Query), Audio: slices.Contains(r.AnswerTypes, "AUDIO"), Answer: utf8.RuneCountInString(r.Answer)})
	}
	go obs.Run(ctx)

	checker := update.New(repo, version, cfg.UpdateCheck, log)
	go checker.Run(ctx)
	srv := api.New(api.Options{
		Config: cfg, Store: st, Token: token, SetupCodePath: setupCodePath,
		Build: api.Build{Version: version, Commit: commit}, Boot: boot,
		Xiaomi: xm, Observer: obs, Engine: engine, Stream: streams, Library: lib, Router: router, Feiniu: fn, Live: hub, Played: dispatcher.Remember, Web: webui.Handler(), Log: log, Update: checker, SetupLog: showCode,
	})
	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second, // /stream/ extends its own deadline
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	if len(cfg.Legacy) > 0 {
		log.Warn("配置　config.yaml 里的 devices 已不使用，音箱在管理页设置")
	}
	or := func(yes bool, a, b string) string {
		if yes {
			return a
		}
		return b
	}
	address := cfg.PublicBaseURL
	if address == "" {
		address = "没有设置，音箱取不到歌"
	}
	logx.Banner(os.Stdout, fmt.Sprintf("TrackFerry %s（%s）已启动", version, commit),
		logx.Row{Name: "地址", Value: address},
		logx.Row{Name: "小米账号", Value: or(xm.Status().Linked, "已关联", "未关联")},
		logx.Row{Name: "飞牛音乐", Value: or(fn.Linked(), "已连接", "未连接")},
		logx.Row{Name: "接管", Value: or(len(cfg.TakeoverSpeakers) == 0, "没有放行", or(slices.Contains(cfg.TakeoverSpeakers, "*"), "所有音箱可选", fmt.Sprintf("放行 %d 项", len(cfg.TakeoverSpeakers))))},
		logx.Row{Name: "数据", Value: cfg.DataDir},
		logx.Row{Name: "检查更新", Value: or(cfg.UpdateCheck, "开", "关")})
	if firstCode != "" {
		showCode(firstCode)
	}
	srv.MarkReady()

	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	log.Info("停止　正在收尾")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Warn("停止　有连接没有收尾", "原因", err)
	}
	log.Info("停止　完成")
	return nil
}

// runRelay is the whole of the relay container (compose.yaml).
func runRelay() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	target := os.Getenv("TRACKFERRY_RELAY_TARGET")
	if target == "" {
		target = "127.0.0.1:5667"
	}
	return relay.Serve(ctx, relay.Socket, target, newLog())
}

// setupCode shows the code of an install without a password:
// docker exec trackferry /trackferry setup-code
func setupCode() error {
	ctx := context.Background()
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "trackferry.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	if _, err := st.AdminCredential(ctx); err == nil {
		fmt.Println("管理密码已经设置过，直接登录。忘记密码：docker exec trackferry /trackferry reset-admin")
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	code, err := auth.LoadOrCreateSetupCode(filepath.Join(cfg.DataDir, "private", "setup_code"))
	if err != nil {
		return err
	}
	logx.SetupCode(os.Stdout, code, cfg.PublicBaseURL)
	return nil
}

// resetAdmin clears the admin password so it can be set again with a fresh setup code.
// Run it inside the container: docker exec trackferry /trackferry reset-admin
func resetAdmin() error {
	ctx := context.Background()
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "trackferry.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.ResetAdmin(ctx); err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, "private", "setup_code")
	_ = os.Remove(path)
	code, err := auth.LoadOrCreateSetupCode(path)
	if err != nil {
		return err
	}
	fmt.Println("管理密码已清除，所有浏览器已退出登录。")
	logx.SetupCode(os.Stdout, code, cfg.PublicBaseURL)
	return nil
}

// healthcheck is the container probe: liveness of the local HTTP server only.
func healthcheck() error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}
	port, err := config.ListenPort(cfg.Listen)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz/live", port))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("live check: HTTP %d", resp.StatusCode)
	}
	return nil
}
