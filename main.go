// Launches a small Blood on the Clocktower discord mover bot.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	"github.com/zku/botc-discord-mover/mover"
)

var configPath = flag.String("config", ".config.json", "Path to config file (json).")

func main() {
	flag.Parse()

	// Load config.
	configData, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("Cannot load config file %q: %v", *configPath, err)
	}
	cfg := &mover.Config{}
	if err := json.Unmarshal(configData, cfg); err != nil {
		log.Fatalf("Cannot parse config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Incomplete config: %v", err)
	}

	m := mover.New(cfg)
	if err := m.RunForever(context.Background()); err != nil {
		log.Fatalf("Mover terminated: %v", err)
	}
}
