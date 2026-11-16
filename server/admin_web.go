package main

import (
	"embed"
	"io/fs"
)

func webRoot() fs.FS { root, _ := fs.Sub(adminFiles, "web"); return root }

//go:embed web/*
var adminFiles embed.FS
