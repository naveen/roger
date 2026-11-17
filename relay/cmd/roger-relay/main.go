// Command roger-relay is the server a Roger radio talks to.
//
//	roger-relay serve    run the relay (default)
//	roger-relay token    mint a device token and its hash
//	roger-relay digest   render (and optionally email) a day's parent digest
//	roger-relay erase    delete everything stored for a device
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // device timezones must resolve in minimal containers

	"github.com/naveen/roger/relay/internal/config"
	"github.com/naveen/roger/relay/internal/digest"
	"github.com/naveen/roger/relay/internal/pipeline"
	"github.com/naveen/roger/relay/internal/server"
	"github.com/naveen/roger/relay/internal/store"
)

func main() {
	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "token":
		err = token()
	case "digest":
		err = runDigest(args)
	case "erase":
		err = erase(args)
	default:
		err = fmt.Errorf("unknown command %q (want serve, token, digest or erase)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "roger-relay:", err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	devs, err := config.LoadDevices(cfg.DevicesFile)
	if err != nil {
		return err
	}
	st, err := store.New(cfg.DataDir)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var pipe pipeline.Pipeline
	if cfg.Provider == "mock" {
		m := &pipeline.Mock{}
		pipe = pipeline.Pipeline{STT: m, LLM: m, TTS: m}
		log.Warn("running with the MOCK pipeline: replies are a chirp and an echo of the press. Development only.")
	} else {
		client := &http.Client{Timeout: 40 * time.Second}
		oa := &pipeline.OpenAIAudio{
			APIKey: cfg.AudioKey, BaseURL: cfg.AudioBaseURL,
			STTModel: cfg.STTModel, TTSModel: cfg.TTSModel, Voice: cfg.TTSVoice,
			Instructions: cfg.TTSInstructions, Language: cfg.Language, HTTP: client,
		}
		pipe = pipeline.Pipeline{
			STT: oa,
			LLM: &pipeline.Anthropic{APIKey: cfg.AnthropicKey, Model: cfg.LLMModel, BaseURL: cfg.AnthropicBaseURL, HTTP: client},
			TTS: oa,
		}
	}

	srv := server.New(devs, st, pipe, log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.RunJanitor(ctx, cfg.SMTP)
	if !cfg.SMTP.Enabled() {
		log.Info("SMTP is not configured; daily digests will not be emailed (use `roger-relay digest` to render one)")
	}

	hs := &http.Server{Addr: cfg.Addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(shut)
	}()
	log.Info("relay listening", "addr", cfg.Addr, "devices", len(devs.Devices), "provider", cfg.Provider)
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func token() error {
	tok, hash, err := config.NewToken()
	if err != nil {
		return err
	}
	fmt.Println("Device token (flash this onto the radio; it is shown only once):")
	fmt.Println("  " + tok)
	fmt.Println("Hash (put this in devices.json as token_sha256):")
	fmt.Println("  " + hash)
	return nil
}

func runDigest(args []string) error {
	fs := flag.NewFlagSet("digest", flag.ExitOnError)
	device := fs.String("device", "", "device id (required)")
	date := fs.String("date", "", "local date YYYY-MM-DD (default: today in the device's timezone)")
	out := fs.String("out", "", "write HTML to this file instead of stdout")
	send := fs.Bool("send", false, "email it to the device's parent_email")
	fs.Parse(args)

	devs, err := config.LoadDevices(envOr("ROGER_DEVICES", "./devices.json"))
	if err != nil {
		return err
	}
	dev := devs.ByID(*device)
	if dev == nil {
		return fmt.Errorf("no device %q in the devices file", *device)
	}
	st, err := store.New(envOr("ROGER_DATA_DIR", "./data"))
	if err != nil {
		return err
	}
	day := time.Now().In(dev.Location())
	if *date != "" {
		if day, err = time.ParseInLocation("2006-01-02", *date, dev.Location()); err != nil {
			return err
		}
	}
	turns, err := st.Day(dev.ID, day.Format("2006-01-02"))
	if err != nil {
		return err
	}
	remembered, _ := st.Remembered(dev.ID)
	data := digest.Build(dev.Kid.Name, day, turns, remembered, dev.EffectiveRules(), dev.Location())
	html, err := digest.Render(data)
	if err != nil {
		return err
	}
	if *send {
		cfg, err := config.FromEnv()
		if err != nil {
			return err
		}
		if dev.ParentEmail == "" {
			return errors.New("device has no parent_email")
		}
		return digest.Send(cfg.SMTP, dev.ParentEmail, digest.Subject(data), html)
	}
	if *out != "" {
		return os.WriteFile(*out, []byte(html), 0o600)
	}
	fmt.Print(html)
	return nil
}

func erase(args []string) error {
	fs := flag.NewFlagSet("erase", flag.ExitOnError)
	device := fs.String("device", "", "device id (required)")
	fs.Parse(args)
	if *device == "" {
		return errors.New("-device is required")
	}
	st, err := store.New(envOr("ROGER_DATA_DIR", "./data"))
	if err != nil {
		return err
	}
	if err := st.Erase(*device); err != nil {
		return err
	}
	fmt.Printf("Erased all stored text for %s.\n", *device)
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
