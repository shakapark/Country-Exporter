package main

import (
	//"errors"
	"fmt"
	"net/http"
	//"time"
	"os"
	"os/signal"
	"syscall"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	
	"github.com/shakapark/Country-Exporter/config"
)

var (
	sc = &config.SafeConfig{C: &config.Config{},}
	configFile = kingpin.Flag("config.file", "Country exporter configuration file.").Default("config.yml").String()
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9144").String()
)

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("Country_Exporter : Beta"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if err := sc.ReloadConfig(*configFile); err != nil {
		log.Fatalf("Error loading config: %s", err)
	}
	log.Infoln("Loaded config file")
	
	log.Infoln("Starting country exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	hup := make(chan os.Signal)
	reloadCh := make(chan chan error)
	signal.Notify(hup, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-hup:
				if err := sc.ReloadConfig(*configFile); err != nil {
					log.Errorf("Error reloading config: %s", err)
					continue
				}
				log.Infoln("Loaded config file")
			case rc := <-reloadCh:
				if err := sc.ReloadConfig(*configFile); err != nil {
					log.Errorf("Error reloading config: %s", err)
					rc <- err
				} else {
					log.Infoln("Loaded config file")
					rc <- nil
				}
			}
		}
	}()
	
	http.HandleFunc("/-/reload",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				fmt.Fprintf(w, "This endpoint requires a POST request.\n")
				return
			}

			rc := make(chan error)
			reloadCh <- rc
			if err := <-rc; err != nil {
				http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
			}
		})
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`Country Exporter`))
	})
	
	http.HandleFunc("/country", func(w http.ResponseWriter, r *http.Request) {
		log.Infof("EndPoint")
		sc.Lock()
		conf := sc.C
		sc.Unlock()
		
		registry := prometheus.NewRegistry()
		log.Infof("Registery")
		for _, db := range conf.Objects {
		
			if db.Target == "" {
				http.Error(w, "'target' parameter must be specified", 400)
				return
			}

			if db.Database == "" {
				http.Error(w, "'database' parameter must be specified", 400)
				return
			}
			
			if db.Login == "" {
				http.Error(w, "'login' parameter must be specified", 400)
				return
			}
			
			if db.Request == "" {
				http.Error(w, "'request' parameter must be specified", 400)
				return
			}
			
			collector := collector{target: db.Target, database: db.Database, login: db.Login, passwd: db.Passwd, request: db.Request}
			registry.MustRegister(collector)
		}
		
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	log.Infof("Listening on %s", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %s", err)
	}
}
