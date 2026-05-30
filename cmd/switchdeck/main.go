package main

import (
	"context"
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
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
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	cfg.DBPath = cfg.DataDir + "/switchdeck.db"

	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		log.Fatalf("create data dir: %v", err)
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
