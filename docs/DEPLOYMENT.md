# Deployment

This project supports a minimal local deployment on Windows. The deployed package contains a single executable plus scripts for starting the service, checking health, and running the review demo.

## Build Deployment Package

From the project root:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy.ps1
```

The package is generated at:

```text
dist\relationship-agent-runtime
```

## Start The Runtime

Open a PowerShell window and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\dist\relationship-agent-runtime\run-server.ps1
```

Keep this terminal open. The service listens on:

```text
http://localhost:8080
```

## Health Check

In a second PowerShell window:

```powershell
powershell -ExecutionPolicy Bypass -File .\dist\relationship-agent-runtime\health.ps1
```

Expected output:

```json
{"status":"ok"}
```

## Run Review Demo

With the server still running:

```powershell
powershell -ExecutionPolicy Bypass -File .\dist\relationship-agent-runtime\demo.ps1
```

The demo performs three turns:

1. Build basic profile and preference memory.
2. Add emotion, important event, and relationship preference.
3. Update a conflicting city memory from Shanghai to Shenzhen.

Then it reads the persisted profile from `/profile/review-demo`.

## Runtime Data

Memory is stored as JSON under:

```text
dist\relationship-agent-runtime\data\memory
```

The most useful file to show in code review is:

```text
dist\relationship-agent-runtime\data\memory\review-demo.json
```

It contains `basic_info`, `preferences`, `emotional_states`, `important_events`, `relationship_preference`, `relationship_state`, `memory_history`, and `conflicts`.
