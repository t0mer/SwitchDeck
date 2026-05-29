package main

import (
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/t0mer/SwitchDeck/internal/config"
	"github.com/t0mer/SwitchDeck/internal/manager"
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

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	clientFactory := func(insecure bool) switchclient.Client {
		return tplink.New(insecure)
	}
	mgr := manager.New(clientFactory)
	srv := server.New(cfg, mgr)

	log.Printf("SwitchDeck %s listening on :%d", version, cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
