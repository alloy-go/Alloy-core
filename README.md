```markdown
# Alloy

![Status](https://img.shields.io/badge/status-active-success)
![Go](https://img.shields.io/badge/backend-Go-00ADD8)
![Kubernetes](https://img.shields.io/badge/platform-Kubernetes-326CE5)
![Postgres](https://img.shields.io/badge/database-PostgreSQL-336791)

**Alloy** is a Kubernetes-native **continuous deployment orchestration system** designed to eliminate "deployment anxiety." While CI tools excel at building artifacts, they often lack the real-time cluster awareness required to manage high-stakes production releases.

Alloy bridges this gap by acting as an intelligent controller that sits between your CI pipeline and your Kubernetes API. It ensures every release is verified progressively and provides an **automated safety net** to revert to a stable state the moment something goes wrong.

---

## 🚀 Key Features

* **Kubernetes-Native Core:** Built on the official `client-go` SDK for direct, low-latency communication with the Kubernetes API layer.
* **Progressive Traffic Shifting:** Decouples deployment from release by gradually shifting traffic percentages.
* **Automated Health Guard:** Monitors live pod metrics and status during rollouts; triggers instant rollbacks on failure detection.
* **Stateful Release Memory:** Maintains a persistent record of every deployment, allowing for "one-click" or automated promotion of stable versions.
* **Webhook-Driven:** Seamlessly integrates with any CI provider (GitHub Actions, GitLab CI, Jenkins, etc.).

---

## 🏗 Architecture

Alloy operates as a specialized orchestration layer. Unlike static scripts, it maintains a continuous observation loop with your cluster.

### The Release Flow
1.  **Trigger:** CI system sends a JSON webhook to Alloy after a successful build.
2.  **Validate:** Alloy verifies project metadata and current cluster health via its persistent database.
3.  **Orchestrate:** Alloy uses the `client-go` library to create/update Kubernetes resources (Deployments, Services, ReplicaSets).
4.  **Observe:** The engine monitors live pod readiness and availability metrics in real-time.
5.  **Reconcile:** Depending on the health data, Alloy either promotes the version to "Stable" or executes an "Automatic Rollback."

```text
  [ CI Tool ] --(Webhook)--> [ Alloy API ] <---> [ Postgres DB ]
                                  |
                                  | (client-go SDK)
                                  v
                        [ Kubernetes API Server ]
                                  |
              --------------------------------------------
              |                   |                      |
      [ Canary Pods ]      [ Stable Pods ]        [ Traffic Rules ]

```

---

## 🚦 Deployment Strategies

### 1. Rolling Update

Standard zero-downtime deployment. Best for routine updates where risk is minimal.

### 2. Canary Progression (Advanced)

Alloy uses a tiered stage model to minimize the "blast radius" of new releases. It follows a specific, data-driven progression:

| Stage | Replicas | Traffic Share | Observation Period |
| --- | --- | --- | --- |
| **Stage 1** | 1 Pod | **17%** | 3 Minutes |
| **Stage 2** | 3 Pods | **50%** | 3 Minutes |
| **Stage 3** | 5 Pods | **83%** | 2 Minutes |
| **Promotion** | Full Cluster | **100%** | Permanent |

> **Note:** If any stage reports health degradation or pod crashes, Alloy immediately cuts traffic back to the previous stable version and scales down the faulty canary.

---

## 🛠 Getting Started

### Environment Configuration

Create a `.env` file with your generic environment settings:

```env
# Alloy Orchestrator Settings
APP_PORT=8080
APP_ENV=production

# Database Configuration
DB_HOST=postgres
DB_PORT=5432
DB_USER=alloy_admin
DB_PASSWORD=secure_password_placeholder
DB_NAME=alloy_orchestrator

```

### Docker Compose Deployment

Use the following configuration to bootstrap the Alloy control plane:

```yaml
version: '3.8'

services:
  alloy-api:
    image: alloy/orchestrator:latest
    ports:
      - "${APP_PORT}:8080"
    environment:
      - DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable
    depends_on:
      - postgres

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    volumes:
      - alloy_storage:/var/lib/postgresql/data

volumes:
  alloy_storage:

```

---

## 📊 Why Alloy?

| Feature | Direct `kubectl` | Standard CI/CD | Alloy Orchestrator |
| --- | --- | --- | --- |
| **Traffic Control** | No | Limited | **Native Canary/Blue-Green** |
| **Rollback Trigger** | Manual | Manual Scripting | **Automated (Health-Based)** |
| **API Awareness** | Static | Polling | **Real-time (SDK-Native)** |
| **Release History** | None | Limited | **Persistent SQL Storage** |


```
