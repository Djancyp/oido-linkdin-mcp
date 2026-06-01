package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

type configJSON struct {
	LinkedInCookie  string `json:"LINKEDIN_COOKIE"`
	LinkedInBaseURL string `json:"LINKEDIN_BASE_URL"`
}

func main() {
	configStr := flag.String("config", "", "JSON string with LinkedIn settings (overrides env vars)")
	flag.Parse()

	if *configStr != "" {
		var cfg configJSON
		if err := json.Unmarshal([]byte(*configStr), &cfg); err != nil {
			log.Fatalf("Failed to parse --config JSON: %v", err)
		}
		setIfNotEnv := func(key, val string) {
			if val != "" && os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
		setIfNotEnv("LINKEDIN_COOKIE", cfg.LinkedInCookie)
		setIfNotEnv("LINKEDIN_BASE_URL", cfg.LinkedInBaseURL)
	}

	log.Println("Starting Oido LinkedIn MCP Server v1.0.0...")
	RunMCPServer()
}
