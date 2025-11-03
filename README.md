<div align="center">
<h1>Cloudflare-ddns</h1>
<h2>A free, reliable dynamic DNS updater for Cloudflare</h2>

[![Publish Status](https://github.com/oberwager/cloudflare-ddns/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/oberwager/cloudflare-ddns/actions/workflows/docker-publish.yml)
[![Build Status](https://github.com/oberwager/cloudflare-ddns/actions/workflows/build.yml/badge.svg)](https://github.com/oberwager/cloudflare-ddns/actions/workflows/build.yml)
[![License](https://img.shields.io/badge/License-MIT-orange.svg)](https://github.com/oberwager/cloudflare-ddns/blob/master/LICENSE)
[![Coverage Status](https://coveralls.io/repos/github/oberwager/cloudflare-ddns/badge.svg?branch=main)](https://coveralls.io/github/oberwager/cloudflare-ddns?branch=main)
[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20A%20Coffee-yellow?style=flat-square&logo=buy-me-a-coffee&logoColor=white)](https://www.buymeacoffee.com/lucasobe)
</div>

Supports multiple zones, IPv6, and configurable TTL. Designed to run as a Kubernetes CronJob or standalone container.

## Features

- Automatic A and AAAA record management
- Multi-zone support with concurrent processing
- Configurable TTL at global, zone, and subdomain levels
- Exponential backoff for transient failures
- Structured JSON logging
- Optimized for Kubernetes deployment

## Why This Version

This is a complete rewrite with several improvements over the [original implementation](https://github.com/timothymiller/cloudflare-ddns):

**Reliability**
- Exponential backoff with jitter for IP detection
- Proper error handling throughout
- HTTP client timeouts and context support
- Validates all API responses

**Performance**
- Concurrent zone processing
- Semaphore-based concurrency for subdomain updates
- 5-10x faster for configs with many zones and subdomains
- Binary size reduced from 51MB to 6MB
- Cronjob optimized for short-lived runs, minimizing resource usage

**Security**
- Uses Cloudflare API tokens with least privilege
- Docker image doesn't run as root
- No dependencies on external tools
- [Zero-log IP provider](https://www.ipify.org/)

## Installation

### Kubernetes
All files are in the `kubernetes/` directory. 

Create the namespace:

```bash
kubectl apply -f https://raw.githubusercontent.com/oberwager/cloudflare-ddns/refs/heads/main/kubernetes/namespace.yaml
```

Create the secret and config map:

```bash
kubectl create secret generic cloudflare-ddns -n cloudflare-ddns --from-literal=CF_API_TOKEN="your-api-token"
```

Then edit `kubernetes/cronjob.yaml` to add your configuration as a config map.

### Standalone

```bash
export CF_API_TOKEN="your-token"
export CF_CONFIG='{"zones":[{"zone_id":"...","subdomains":[{"name":"home","proxied":true}]}]}'
export CF_IPV6_ENABLED="true"
./cloudflare-ddns
```

**NOTE**: You will have to set this to run periodically using your own scheduling method (e.g. cron).

### Docker

```bash
docker run -e CF_API_TOKEN="your-token" \
  -e CF_CONFIG='...' \
  -e CF_IPV6_ENABLED="true" \
  oberwager/cloudflare-ddns:latest
```

**NOTE**: You will have to set this to run periodically using your own scheduling method (e.g. cron).

## Configuration

The application is configured via environment variables and a JSON configuration.

Its designed to use config maps and secrets in Kubernetes, where only the API key is in a secret, and everything else is written in yaml. It can also be run standalone or in Docker.

### Environment Variables

- `CF_API_TOKEN` (required): Cloudflare API token with DNS edit permissions
- `CF_CONFIG` (required): JSON configuration string
- `CF_IPV6_ENABLED` (optional): Set to "true" to enable IPv6 AAAA records

### Configuration Format

```json
{
  "default_ttl": 300,
  "concurrency_limit": 10,
  "zones": [
    {
      "zone_id": "your-zone-id-here",
      "ttl": 600,
      "subdomains": [
        {
          "name": "home",
          "proxied": true
        },
        {
          "name": "@",
          "proxied": false,
          "ttl": 300
        },
        {
          "name": "vpn",
          "proxied": false,
          "ttl": 120
        }
      ]
    }
  ]
}
```

**Configuration hierarchy:**
- Subdomain TTL overrides zone TTL
- Zone TTL overrides default TTL
- Default TTL is 300 seconds if not specified
- Default concurrency limit is 10 if not specified. Cloudflare API rate limits are 1,200 requests per five-minute period per user.

**Note:** TTL is ignored for proxied records (Cloudflare sets them to automatic).

### Getting Your Zone ID

```bash
curl -X GET "https://api.cloudflare.com/client/v4/zones" \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -H "Content-Type: application/json"
```

## Logging

All output is structured JSON for easy parsing:

```json
{"time":"2025-1-1T01:01:19Z","level":"INFO","msg":"starting cloudflare-ddns","version":"88fb18a"}
{"time":"2025-1-1T01:01:19Z","level":"INFO","msg":"detected public ip","type":"ipv4","ip":"1.2.3.4"}
{"time":"2025-1-1T01:01:20Z","level":"INFO","msg":"updated record","fqdn":"home.example.com","type":"A","ip":"1.2.3.4","proxied":true,"ttl":300,"old_ip":"5.6.7.8","old_proxied":true,"old_ttl":1}
{"time":"2025-1-1T01:01:20Z","level":"INFO","msg":"cloudflare-ddns completed successfully"}
```

## Contributing

Pull requests welcome. Right now my use case is for kubernetes, so contributions for other deployment methods are appreciated.

## License

MIT
