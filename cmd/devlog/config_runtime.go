package main

import internalconfig "github.com/amo/devlog/internal/config"

func loadRuntimeConfig() (internalconfig.Config, error) {
	return internalconfig.Load()
}
