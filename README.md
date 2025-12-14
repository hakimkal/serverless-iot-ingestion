# Fast IoT Data Ingestion Service 🚀

**Summary:**  
A **serverless, high-performance service** for ingesting IoT data in real-time. Designed to run on **AWS Lambda**, Azure Functions, or any other FaaS platform, it combines **secure API key authentication**, **per-key rate limiting**, and **burst-tolerant ingestion** to handle high-volume IoT traffic reliably. Redis-backed for multi-instance deployments, with an **in-memory fallback**, it ensures consistent operation with minimal operational overhead.

---

## Key Features
- **Serverless Ready** – Deploy on AWS Lambda or other FaaS platforms.
- **Health Endpoint** – Quickly check service status.
- **Batch Ingestion** – Efficiently handle multiple IoT events at once.
- **API Key Authentication** – Secure access using whitelisted API keys.
- **Per-Key Rate Limits** – Configurable requests per minute and burst capacity.
- **Redis-Backed Multi-Instance Support** – Shared rate limiting across serverless instances.
- **In-Memory Fallback** – Ensures continued operation if Redis is unavailable.
- **Middleware-Ready** – Easy integration with HTTP handlers.
- **Burst Support** – Token bucket implementation allows short bursts without dropping requests.

---

## Why Use This Service?
- **Scalable & Serverless** – Automatically scales with traffic using FaaS.
- **Reliable** – Prevents abuse while ensuring high-throughput ingestion.
- **Easy Integration** – Minimal configuration for deployment and service integration.
- **IoT-Optimized** – Handles real-world IoT workloads with thousands of devices sending data.

---

## Quick Start

1. **Install dependencies**

```bash
go mod tidy

```

2. **Run locally**

```bash
go run or air run (if you prefer air for hot reloading)
```
