# Monitoring Stack

Prometheus + Grafana observability for the Recipe Processor API.

## Quick Start

The monitoring stack starts automatically with `docker compose up`. No additional steps needed.

## Access

| Service    | URL                        | Notes                          |
|------------|----------------------------|--------------------------------|
| Grafana    | http://localhost:3000      | No login required (anonymous)  |
| Prometheus | http://localhost:9090      | Query and target status        |
| API metrics| http://localhost:8080/metrics | Raw Prometheus metrics       |

## Dashboard: Recipe Processor Overview

The pre-provisioned Grafana dashboard includes these panels:

| Panel                    | Description                                                    |
|--------------------------|----------------------------------------------------------------|
| Request Rate             | HTTP requests per second by status code                        |
| Request Latency          | p50 / p95 / p99 latency from `http_request_duration_seconds`  |
| Error Rate               | Rate of non-2xx HTTP responses                                 |
| LLM Processing Time      | p50 / p95 / p99 latency from `llm_request_duration_seconds`   |
| LLM Success / Error Rate | LLM requests per second by status                              |
| DB Query Duration        | p50 / p95 / p99 latency from `db_query_duration_seconds`      |
| Event Processing         | Events processed per second by type and status                 |
| Event Queue Depth        | Current depth of the event queue                               |

## Architecture

```
API (:8080/metrics) ──scrape 15s──▶ Prometheus (:9090) ──query──▶ Grafana (:3000)
```

- **Prometheus** scrapes the API's `/metrics` endpoint every 15 seconds.
- **Grafana** queries Prometheus and renders the pre-provisioned dashboard.
- Both services store data in named Docker volumes (`prometheus_data`, `grafana_data`).

## Configuration Files

```
monitoring/
├── prometheus.yml                           # Prometheus scrape config
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/prometheus.yml       # Auto-register Prometheus datasource
│   │   └── dashboards/dashboard.yml         # Dashboard provisioning config
│   └── dashboards/
│       └── recipe-processor.json            # Grafana dashboard definition
└── README.md                                # This file
```
