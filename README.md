# Ontario AQHI Exporter

[![build](https://github.com/ztroop/ontario-aqhi-exporter/actions/workflows/build.yml/badge.svg)](https://github.com/ztroop/ontario-aqhi-exporter/actions/workflows/build.yml)
[![docker pulls](https://img.shields.io/docker/pulls/ztroop/ontario-aqhi-exporter)](https://hub.docker.com/r/ztroop/ontario-aqhi-exporter)

A simple scraper and prometheus exporter for Ontario Air Quality Health Index (AQHI) data. This fetches data from the [Air Quality Ontario](http://www.airqualityontario.com/aqhi/index.php) website. A proper API would've been preferable, but you work with what ya got.

## Configuration

| Environment Var       	       | Flags              | Default                 	    | Description                                                                                                      |
|----------------------------|-----------------------------|---------------------------- |------------------------------------------------------------------------------------------------------------------|
| `LISTEN_ADDR`           | `listen`            | `127.0.0.1:8085`                     | Network address for `/metrics` to listen on |
| `SCRAPE_URL`           | `scrape`            | `http://www.airqualityontario.com/aqhi/index.php`                     | Default URL to scrape from |
| `STATION_LOCATION`           | `station`            | `Kitchener`                     | Default URL to scrape from |

To see what stations are available, [see the website](http://www.airqualityontario.com/aqhi/locations.php?text_only=1).

### Prometheus Configuration

The code block below would be sufficient to configure `prometheus` to scrape our `/metrics` endpoint. Readings are updated hourly, so there's no need to fetch faster than that.

```yml
scrape_configs:
  - job_name: 'ontario-aqhi-exporter'
    scrape_interval: 60m
    static_configs:
    - targets:
      - 127.0.0.1:8085
```

## Usage

There's a few different ways you can interact with this software, below are a few examples.

### Binary

```sh
./ontario-aqhi-exporter -station Kitchener -listen 127.0.0.1:8085 -scrape http://www.airqualityontario.com/aqhi/index.php
```

### Docker

```sh
docker run -d --restart on-failure --name=ontario-aqhi-exporter -p 8085:8085 ztroop/ontario-aqhi-exporter -station Kitchener -listen 127.0.0.1:8085
```

### Docker Compose

```yml
version: '3.2'
services:
  ontario-aqhi-exporter:
    image: ontario-aqhi-exporter:latest
    build: .
    ports:
    - 8085:8085
    environment:
    - LISTEN_ADDR=0.0.0.0:8085
    - SCRAPE_URL=http://www.airqualityontario.com/aqhi/index.php
    - STATION_LOCATION=KITCHENER
    restart: unless-stopped
```

## AQHI Level Descriptions

It's important to understand what the readings actually mean. Below is a snippet from [Air Quality Ontario](http://www.airqualityontario.com/aqhi/index.php):

| Health Risk       	       | Air Quality Health Index              | Description                                                                                                     |
|----------------------------|-----------------------------|---------------------------- |
| Low | 1 - 3 | Ideal air quality for outdoor activities. |
| Moderate | 4 - 6 | No need to modify your usual outdoor activities unless you experience symptoms such as coughing and throat irritation.
| High | 7 - 10 | Consider reducing or rescheduling strenuous activities outdoors if you experience symptoms such as coughing and throat irritation.
| Very High | 10+ | Reduce or reschedule strenuous activities outdoors, especially if you experience symptoms such as coughing and throat irritation.

To see what is measured for each station, [see the website](http://www.airqualityontario.com/history/summary.php).