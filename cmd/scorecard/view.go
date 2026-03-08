package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

//go:embed embed/dist/*
var embeddedUI embed.FS

//go:embed embed/cockpit-dist/*
var embeddedCockpitUI embed.FS

// startViewServer serves the embedded scorecard UI with the given scorecard JSON
// and optionally the cockpit UI with cockpit JSON. Opens the browser and blocks.
func startViewServer(scorecardJSON []byte, cockpitJSON []byte, clientID string) {
	uiFS, err := fs.Sub(embeddedUI, "embed/dist")
	if err != nil {
		log.Fatalf("  ✗ load embedded UI: %v", err)
	}
	cockpitFS, err := fs.Sub(embeddedCockpitUI, "embed/cockpit-dist")
	if err != nil {
		log.Fatalf("  ✗ load embedded cockpit UI: %v", err)
	}

	mux := http.NewServeMux()

	// Serve scorecard.json from memory
	mux.HandleFunc("/scorecard.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(scorecardJSON)
	})

	// Serve cockpit JSON from memory (if available)
	if cockpitJSON != nil && clientID != "" {
		jsonName := fmt.Sprintf("/cockpit/cockpit_%s.json", clientID)
		mux.HandleFunc(jsonName, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(cockpitJSON)
		})
	}

	// Serve cockpit UI under /cockpit/
	mux.Handle("/cockpit/", http.StripPrefix("/cockpit/", http.FileServer(http.FS(cockpitFS))))

	// Serve scorecard UI at root
	mux.Handle("/", http.FileServer(http.FS(uiFS)))

	port := findFreePort()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	scorecardURL := fmt.Sprintf("http://%s", addr)

	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  ✓ Realised scorecard:   %s\n", scorecardURL)
	if cockpitJSON != nil && clientID != "" {
		cockpitURL := fmt.Sprintf("http://%s/cockpit/?client=%s", addr, clientID)
		fmt.Printf("  ✓ Unrealised cockpit:   %s\n", cockpitURL)
	}
	fmt.Println()
	fmt.Println("    Press Ctrl+C to stop")
	fmt.Println()

	// Open both URLs in the browser
	go openBrowser(scorecardURL)
	if cockpitJSON != nil && clientID != "" {
		go openBrowser(fmt.Sprintf("http://%s/cockpit/?client=%s", addr, clientID))
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("  ✗ server error: %v", err)
	}
}

func findFreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 8080
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	cmd.Stderr = os.Stderr
	cmd.Start()
}
