package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/timeglass/snow/monitor"
)

func main() {
	log := log.New(os.Stdout, "Snow: ", 0)
	log.Printf(`Greetings, I'm the file system watcher that knows nothing...`)

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %s", err)
	}

	m, err := monitor.New(cwd, nil, 0)
	if err != nil {
		log.Fatalf("Failed to create monitor for '%s': %s", cwd, err)
	}

	go func() {
		for err := range m.Errors() {
			log.Println(err)
		}
	}()

	evs, err := m.Start()
	if err != nil {
		log.Fatalf("Failed to start monitor for '%s': %s", m.Dir(), err)
	}

	log.Printf(`Watching directory '%s'`, m.Dir())
	for ev := range evs {
		rel, err := filepath.Rel(m.Dir(), ev.Dir())
		if err != nil {
			log.Fatalf("Failure to determine relative path for '%s': %s", ev.Dir(), err)
		}

		log.Printf("Something happened to or in '/%s'", rel)
	}
}
