package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	yaml "gopkg.in/yaml.v2"
)

const (
	defaultSpiderTime = 30 * time.Second
	defaultTimeout    = 20 * time.Second
)

var (
	configFile = flag.String("config", "./config.yml", "config file")
	listen     = flag.String("listen", ":8080", "listen address")
	verbose    = flag.Bool("v", false, "verbose")

	reqHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "http",
			Name:      "time",
			Help:      "end-to-end time of requests",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 16), // 0.001 -> 32 seconds
		},
		[]string{"name", "result"},
	)
)

func init() {
	prometheus.MustRegister(reqHist)
}

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile(*configFile)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	c := Config{}
	if err := yaml.Unmarshal(b, &c); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	for name, t := range c.Targets {
		st := t.SpiderTime
		if st == 0 {
			st = c.SpiderTime
		}
		if st == 0 {
			st = defaultSpiderTime
		}

		timeout := t.Timeout
		if timeout == 0 {
			timeout = c.Timeout
		}
		if timeout == 0 {
			timeout = defaultTimeout
		}

		// sanity check
		if u, err := url.Parse(t.URL); err != nil {
			panic(fmt.Sprintf("invalid target %q: %s\n", name, err))
		} else {
			switch u.Scheme {
			case "http", "https":
			default:
				fmt.Printf("invalid target %q: unsupported scheme %q\n", name, u.Scheme)
				os.Exit(1)
			}
		}

		go spider(name, t, st, timeout)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `see <a href="/metrics">/metrics</a>`)
	})
	http.Handle("/metrics", promhttp.Handler())
	fmt.Printf("listening on %s...\n", *listen)
	http.ListenAndServe(*listen, nil)
}

func spider(name string, target Target, spiderTime, timeout time.Duration) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			TLSHandshakeTimeout: timeout,
			DialContext: (&net.Dialer{
				Timeout: timeout,
			}).DialContext,
		},
	}

	if *verbose {
		fmt.Printf("starting %q spider:%s timeout:%s\n", name, spiderTime, timeout)
	}

	for {
		var (
			t       = time.Now()
			success = false
		)
		req, err := http.NewRequest(target.Method, target.URL, nil)
		if err != nil {
			fmt.Printf("%s: %s\n", name, err)
		} else {
			resp, err := client.Do(req)
			if err == nil {
				if _, err := ioutil.ReadAll(resp.Body); err == nil {
					resp.Body.Close()
					if resp.StatusCode == 200 {
						success = true
					} else {
						if *verbose {
							fmt.Printf("%s: status code %d\n", name, resp.StatusCode)
						}
					}
				}
			} else {
				if *verbose {
					fmt.Printf("%s: %s\n", name, err)
				}
			}
		}

		dt := time.Since(t)
		sv := "error"
		if success {
			sv = "ok"
		}
		if *verbose {
			fmt.Printf("- %s: dt:%s success:%s\n", name, dt, sv)
		}
		reqHist.WithLabelValues(name, sv).Observe(dt.Seconds())

		time.Sleep(spiderTime)
	}
}
