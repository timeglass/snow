package main

import (
	"log"
	"os"
	// "path/filepath"

	"github.com/timeglass/snow/index"
	"github.com/timeglass/snow/monitor"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("snow: ")

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %s", err)
	}

	m, err := monitor.New(cwd, nil, 0)
	if err != nil {
		log.Fatalf("Failed to create monitor for '%s': %s", cwd, err)
	}

	log.Printf(`Greetings, I'm the file system watcher that knows nothing...`)
	go func() {
		for err := range m.Errors() {
			log.Println(err)
		}
	}()

	evs, err := m.Start()
	if err != nil {
		log.Fatalf("Failed to start monitor for '%s': %s", cwd, err)
	}

	//use an index for file events
	idx, err := index.NewLazy(evs)
	if err != nil {
		log.Fatalf("Failed to create index: %s", err)
	}

	idx.Start()

	// //but also log dir events
	// log.Printf(`Watching directory '%s'`, m.Dir())
	// for ev := range evs {
	// 	rel, err := filepath.Rel(m.Dir(), ev.Dir())
	// 	if err != nil {
	// 		log.Fatalf("Failure to determine relative path for '%s': %s", ev.Dir(), err)
	// 	}

	// 	log.Printf("Something happened to or in '/%s'", rel)
	// }
}
