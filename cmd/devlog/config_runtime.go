package main

import internalconfig "github.com/amohamma8029/devlog/internal/config"

func loadRuntimeConfig() (internalconfig.Config, error) {
	return internalconfig.Load()
}
