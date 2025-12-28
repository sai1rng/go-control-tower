# Go Control Tower - The "Commander" Tool

This project implements the "Commander" architecture. It is a tool designed to run on a central node (laptop, Bastion host, or Lambda) to orchestrate commands across your EC2 fleet.

## Overview

The tool uses the AWS SDK to dynamically find your instances based on tags and then fires HTTP requests to a Go Service running on **Port 5000** on each instance.

## Prerequisites

1.  **Tagging**: Ensure your EC2 instances have a common tag, e.g., `App=WindowsExporter`.
2.  **Security Group**: Your "Commander" machine must have network access to **Port 5000** on the instances.
3.  **AWS Credentials**: The environment where you run this script needs AWS credentials configured (e.g., `~/.aws/credentials` or IAM Role).

## Usage

### 1. Run the Server

Start the Control Tower application:

```bash
go run main.go
```

The server will start listening on port `8000`.

### 2. Trigger a Command

You can control your fleet by sending HTTP GET requests to the `/control` endpoint.

**Example:**

To check the status of all instances tagged with `App=WindowsExporter`:

```bash
curl "http://localhost:8000/control?key=App&val=WindowsExporter&action=status"
```

### API Reference

**Endpoint**: `GET /control`

| Parameter | Description |
|-----------|-------------|
| `key`     | The AWS Tag Key to filter instances. |
| `val`     | The AWS Tag Value to filter instances. |
| `action`  | The command to send to the agents (`start`, `stop`, `status`). |

## Configuration

*   **AgentPort**: `5000` (Defined in `main.go`, port where the agent listens on nodes)
*   **ServerPort**: `:8000` (Defined in `main.go`, port this Control Tower listens on)