package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

type Config struct {
	Modules map[string]Module `yaml:"modules"`
}

type SafeConfig struct {
	sync.RWMutex
	C *Config
}

type Module struct {
	Prober  string        `yaml:"prober"`
	Timeout time.Duration `yaml:"timeout"`
	HTTP    HTTPProbe     `yaml:"http"`
}

type HTTPProbe struct {
	// Defaults to 2xx.
	ValidStatusCodes []int             `yaml:"valid_status_codes"`
	Prefix           string            `yaml:"prefix"`
	Headers          map[string]string `yaml:"headers"`
}

var Probers = map[string]func(string, http.ResponseWriter, Module) bool{
	"http": probeHTTP,
}

func (sc *SafeConfig) reloadConfig(confFile string) (err error) {
	var c = &Config{}

	yamlFile, err := ioutil.ReadFile(confFile)
	if err != nil {
		log.Errorf("Error reading config file: %s", err)
		return err
	}

	if err := yaml.Unmarshal(yamlFile, c); err != nil {
		log.Errorf("Error parsing config file: %s", err)
		return err
	}

	sc.Lock()
	sc.C = c
	sc.Unlock()

	log.Infoln("Loaded config file")
	return nil
}

func probeHandler(w http.ResponseWriter, r *http.Request, conf *Config) {
	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", 400)
		return
	}

	moduleName := params.Get("module")
	if moduleName == "" {
		moduleName = "sentry"
	}
	module, ok := conf.Modules[moduleName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown module %q", moduleName), 400)
		return
	}
	prober, ok := Probers[module.Prober]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown prober %q", module.Prober), 400)
		return
	}

	start := time.Now()
	success := prober(target, w, module)
	fmt.Fprintf(w, "probe_duration_seconds %f\n", time.Since(start).Seconds())
	if success {
		fmt.Fprintln(w, "probe_success 1")
	} else {
		fmt.Fprintln(w, "probe_success 0")
	}
}

func init() {
	prometheus.MustRegister(version.NewCollector("sentry_exporter"))
}

func main() {
	var (
		configFile    = flag.String("config.file", "sentry_exporter.yml", "Sentry exporter configuration file.")
		listenAddress = flag.String("web.listen-address", ":9412", "The address to listen on for HTTP requests.")
		showVersion   = flag.Bool("version", false, "Print version information.")
		sc            = &SafeConfig{
			C: &Config{},
		}
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("sentry_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting sentry_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	if err := sc.reloadConfig(*configFile); err != nil {
		log.Fatalf("Error loading config: %s", err)
	}

	hup := make(chan os.Signal)
	reloadCh := make(chan chan error)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				if err := sc.reloadConfig(*configFile); err != nil {
					log.Errorf("Error reloading config: %s", err)
				}
			case rc := <-reloadCh:
				if err := sc.reloadConfig(*configFile); err != nil {
					log.Errorf("Error reloading config: %s", err)
					rc <- err
				} else {
					rc <- nil
				}
			}
		}
	}()

	http.Handle("/metrics", prometheus.Handler())
	http.HandleFunc("/probe",
		func(w http.ResponseWriter, r *http.Request) {
			sc.RLock()
			c := sc.C
			sc.RUnlock()

			probeHandler(w, r, c)
		})
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
		w.Write([]byte(`<html>
            <head><title>Sentry Exporter</title></head>
            <body>
            <h1>Sentry Exporter</h1>
            <p><a href="/probe?target=apimutate">Probe sentry project</a></p>
            <p><a href="/metrics">Metrics</a></p>
            </body>
            </html>`))
	})

	log.Infoln("Listening on", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %s", err)
	}
}
