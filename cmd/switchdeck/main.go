package main

import (
	"context"
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
	"github.com/t0mer/SwitchDeck/internal/auth"
	"github.com/t0mer/SwitchDeck/internal/config"
	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/server"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

var version = "dev"

func main() {
	cfg := config.Default()

	flag.IntVar(&cfg.Port, "port", cfg.Port, "HTTP listening port")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: debug, info, warning, error")
	flag.StringVar(&cfg.DataDir, "data", cfg.DataDir, "Data directory")
	showVersion := flag.Bool("version", false, "Print version and exit")
	resetPassword := flag.Bool("reset-password", false, "Reset admin credentials interactively and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	cfg.DBPath = cfg.DataDir + "/switchdeck.db"

	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	if *resetPassword {
		if err := runResetPassword(cfg.DataDir); err != nil {
			log.Fatalf("reset-password: %v", err)
		}
		os.Exit(0)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	encKey, err := st.EncryptionKey()
	if err != nil {
		log.Fatalf("encryption key: %v", err)
	}

	notifStore := notification.NewStore(st.DB(), encKey)
	notifSvc := notification.NewService(notifStore)

	clientFactory := func(insecure bool) switchclient.Client {
		return tplink.New(insecure)
	}

	mgr := manager.New(clientFactory)
	mgr.SetSnapshotHandler(func(snap *models.SwitchSnapshot, _ bool) {
		st.UpsertSnapshot(context.Background(), snap)
	})
	mgr.SetNotificationService(notifSvc)

	if err := mgr.LoadFromStore(context.Background(), st, encKey); err != nil {
		log.Fatalf("load switches: %v", err)
	}

	h := handlers.New(mgr, st, encKey, notifStore)
	srv := server.New(cfg, h, st)

	log.Printf("SwitchDeck %s listening on :%d", version, cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runResetPassword(dataDir string) error {
	st, err := store.Open(dataDir + "/switchdeck.db")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	fmt.Print("New username: ")
	var username string
	fmt.Scanln(&username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	fmt.Print("New password: ")
	pw1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	if len(pw1) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	fmt.Print("Confirm password: ")
	pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	if string(pw1) != string(pw2) {
		return fmt.Errorf("passwords do not match")
	}

	hash, err := auth.HashPassword(string(pw1))
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx := context.Background()
	st.SetSetting(ctx, "auth_username", username)
	st.SetSetting(ctx, "auth_password_hash", hash)

	fmt.Println("Credentials updated. Restart the server to apply.")
	return nil
}
