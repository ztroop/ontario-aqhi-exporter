package main

import (
	"flag"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/gocolly/colly/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	qualityLevel        = regexp.MustCompile("([0-9]+)")
	listenAddr   string = "127.0.0.1:8085"
	scrapeURL    string = "http://www.airqualityontario.com/aqhi/index.php"
	cacheTTL     string = "300"
	notFound            = ttlcache.ErrNotFound
)

type Forecast struct {
	Station  string
	Current  float64
	Upcoming float64
	Tomorrow float64
}

type ForecastCollector struct {
	Cache    *ttlcache.Cache
	current  *prometheus.Desc
	upcoming *prometheus.Desc
	tomorrow *prometheus.Desc
}

func (c *ForecastCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.current
	ch <- c.upcoming
	ch <- c.tomorrow
}

func (c *ForecastCollector) Collect(ch chan<- prometheus.Metric) {
	var forecast []Forecast
	if val, err := c.Cache.Get("AQHI"); err != notFound || val != nil {
		forecast = val.([]Forecast)
	} else {
		forecast = fetchForecast()
		c.Cache.Set("AQHI", forecast)
	}

	for _, l := range forecast {
		ch <- prometheus.MustNewConstMetric(c.current, prometheus.GaugeValue, l.Current, l.Station)
		ch <- prometheus.MustNewConstMetric(c.upcoming, prometheus.GaugeValue, l.Upcoming, l.Station)
		ch <- prometheus.MustNewConstMetric(c.tomorrow, prometheus.GaugeValue, l.Tomorrow, l.Station)
	}
}

func NewAQHICollector(cache *ttlcache.Cache) *ForecastCollector {
	return &ForecastCollector{
		Cache: cache,
		current: prometheus.NewDesc("ontario_current_aqhi_level",
			"Current AQHI level",
			[]string{"station"}, nil,
		),
		upcoming: prometheus.NewDesc("ontario_upcoming_aqhi_level",
			"Upcoming AQHI level",
			[]string{"station"}, nil,
		),
		tomorrow: prometheus.NewDesc("ontario_tomorrow_aqhi_level",
			"Tomorrow AQHI level",
			[]string{"station"}, nil,
		),
	}
}

func getLevel(i string) float64 {
	match := qualityLevel.FindStringSubmatch(i)
	if len(match) == 0 {
		return 0
	} else {
		n, err := strconv.ParseFloat(match[0], 64)
		if err != nil {
			return 0
		} else {
			return n
		}
	}
}

func fetchForecast() []Forecast {
	forecast := []Forecast{}
	c := colly.NewCollector()

	c.OnHTML("table[class=\"resourceTable\"] tbody", func(e *colly.HTMLElement) {
		e.ForEach("tr", func(_ int, row *colly.HTMLElement) {
			f := Forecast{}
			columns := row.ChildTexts("td")
			if len(columns) == 4 {
				f.Station = columns[0]
				f.Current = getLevel(columns[1])
				f.Upcoming = getLevel(columns[2])
				f.Tomorrow = getLevel(columns[3])
			} else if len(columns) > 4 {
				f.Station = columns[0]
				f.Current = getLevel(columns[1])
				f.Upcoming = getLevel(columns[3])
				f.Tomorrow = getLevel(columns[4])
			}
			forecast = append(forecast, f)
		})
	})

	c.OnRequest(func(r *colly.Request) {
		log.Info("Fetching: ", r.URL)
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Error("Something went wrong: ", err)
	})

	c.Visit(scrapeURL)

	return forecast
}

func lookupEnvOrString(key string, defaultVal *string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return *defaultVal
}

func withLogging(h http.Handler) http.Handler {
	loggingFn := func(w http.ResponseWriter, req *http.Request) {
		h.ServeHTTP(w, req)
		log.WithFields(log.Fields{
			"host":   req.RemoteAddr,
			"uri":    req.RequestURI,
			"method": req.Method,
		}).Info("Incoming Request")
	}
	return http.HandlerFunc(loggingFn)
}

func main() {
	flag.StringVar(&listenAddr, "listen", lookupEnvOrString("LISTEN_ADDR", &listenAddr), "specify address to bind with port")
	flag.StringVar(&scrapeURL, "scrape", lookupEnvOrString("SCRAPE_URL", &scrapeURL), "specify url to fetch from")
	flag.StringVar(&cacheTTL, "cache", lookupEnvOrString("CACHE_TTL", &cacheTTL), "seconds to cache data")
	flag.Parse()

	cache := ttlcache.NewCache()
	ttl, err := strconv.Atoi(cacheTTL)
	if err != nil {
		log.Fatal("Invalid TTL value: ", err)
	}
	cache.SetTTL(time.Duration(ttl) * time.Second)
	cache.SkipTTLExtensionOnHit(true)

	expirationCallback := func(key string, _ ttlcache.EvictionReason, _ interface{}) {
		log.Info("Cache expired for key: ", key)
	}
	cache.SetExpirationReasonCallback(expirationCallback)

	airQualityCollector := NewAQHICollector(cache)
	prometheus.MustRegister(airQualityCollector)

	http.Handle("/metrics", withLogging(promhttp.Handler()))
	log.Info("Serving on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
