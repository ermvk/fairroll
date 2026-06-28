# Fairroll

Distributed Event-Driven payment platform focused on performance engineering.

The project starts with a deliberately naive architecture and evolves through load testing, profiling, bottleneck analysis, and measured optimization.

No premature optimization. Every improvement must be justified by data.

## Principles

- Build first
- Measure second
- Optimize third


## Architecture

```mermaid
graph TD
    Client[Client REST] --> Payment[payment]

    Payment --> Kafka[(Kafka)]

    Kafka --> Wallet[wallet]
    Wallet --> Kafka

    Kafka --> Notification[notification]
    Notification --> Auth[auth]
```

## Stack

- Go
- Redis
- PostgreSQL
- Kafka
- Franz-go
- Docker
- Prometheus
- Grafana
