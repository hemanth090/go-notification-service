package main

import "embed"

// uiFiles is the web page, baked into the program so there are no extra files
// to copy around when you run it.
//
//go:embed index.html
var uiFiles embed.FS
