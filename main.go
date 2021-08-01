package main

import (
	"flag"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	qualityLevel           = regexp.MustCompile("([0-9]+)")
	listenAddr      string = "127.0.0.1:8085"
	scrapeURL       string = "http://www.airqualityontario.com/aqhi/index.php"
	stationLocation string = "KITCHENER"
)

type Forecast struct {
	Station  string
	Current  float64
	Upcoming float64
	Tomorrow float64
}

type ForecastCollector struct {
	station  string
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
	forecast := fetchForecast(c.station)

	ch <- prometheus.MustNewConstMetric(c.current, prometheus.GaugeValue, forecast.Current, forecast.Station)
	ch <- prometheus.MustNewConstMetric(c.upcoming, prometheus.GaugeValue, forecast.Upcoming, forecast.Station)
	ch <- prometheus.MustNewConstMetric(c.tomorrow, prometheus.GaugeValue, forecast.Tomorrow, forecast.Station)
}

func NewAQHICollector(station string) *ForecastCollector {
	return &ForecastCollector{
		station: station,
		current: prometheus.NewDesc("current_aqhi_level",
			"Current AQHI level",
			[]string{"station"}, nil,
		),
		upcoming: prometheus.NewDesc("upcoming_aqhi_level",
			"Upcoming AQHI level",
			[]string{"station"}, nil,
		),
		tomorrow: prometheus.NewDesc("tomorrow_aqhi_level",
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

func fetchForecast(s string) Forecast {
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

	for _, v := range forecast {
		if strings.ToUpper(v.Station) == s {
			return v
		}
	}

	return Forecast{}
}

func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	flag.StringVar(&listenAddr, "listen", lookupEnvOrString("LISTEN_ADDR", listenAddr), "specify address to bind with port")
	flag.StringVar(&scrapeURL, "scrape", lookupEnvOrString("SCRAPE_URL", scrapeURL), "specify url to fetch from")
	flag.StringVar(&stationLocation, "station", lookupEnvOrString("STATION_LOCATION", stationLocation), "specify weather station")
	flag.Parse()

	airQualityCollector := NewAQHICollector(strings.ToUpper(stationLocation))
	prometheus.MustRegister(airQualityCollector)

	http.Handle("/metrics", promhttp.Handler())
	log.Info("Serving on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
