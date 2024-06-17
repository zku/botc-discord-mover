// Launches a small Blood on the Clocktower discord mover bot.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	"github.com/zku/botc-discord-mover/mover"
)

var configPath = flag.String("config", ".config.json", "Path to config file (json).")

func main() {
	flag.Parse()

	cfg := &mover.Config{}
	if *configPath != "" {
		contents, err := ioutil.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("Cannot load config file %q: %v", *configPath, err)
		}
		if err := json.Unmarshal(contents, cfg); err != nil {
			log.Fatalf("Cannot parse config: %v", err)
		}
	} else {
		var err error
		cfg, err = mover.ConfigFromEnv()
		if err != nil {
			log.Fatalf("Cannot load config from environment variables: %v", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Incomplete config: %v", err)
	}

	m := mover.New(cfg)
	if err := m.RunForever(); err != nil {
		log.Fatalf("Mover terminated: %v", err)
	}
}
