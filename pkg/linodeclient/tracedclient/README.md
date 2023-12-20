# Traced Client Wrapper

This package, `tracedclient`, is a generated client wrapper for the Linode Client. The code is generated using the tool `gowrap` with a specified template for OpenTelemetry instrumentation.

Code generation streamlines development, reduces boilerplate, and enhances consistency. It boosts productivity by automating repetitive tasks, saves time, and simplifies maintenance. It also minimizes errors and enforces standards.

## Purpose

1. **Instrumentation with OpenTelemetry:** The primary purpose of this code is to provide an instrumented version of the Linode COSI driver client with OpenTelemetry spans. This is achieved by adding tracing functionality to various methods in the client.

2. **Observability:** OpenTelemetry is used for distributed tracing, which helps in observing and understanding how requests propagate through a system, making it easier to identify performance bottlenecks and troubleshoot issues.

## Code Generation

The code in this package is generated using the `gowrap` tool. To generate the code, you can use the following commands:

- **Using `go generate`**:

  ```bash
  go generate ./...
  ```

  This command will trigger the `go:generate` directives in the code, causing the `gowrap` tool to generate the necessary files.

  Ensure that you have the `gowrap` tool installed before running the code generation command.

- **Using Makefile**:

  ```bash
  make codegen
  ```

  The Makefile target will ensure that the `gowrap` tool is installed.

## Usage Example

To use the instrumented client:

```go
baseClient := linodeclient.NewLinodeClient(token, ua, apiURL, apiVersion) // Initialize the original Linode client
tracedClient := tracedclient.NewClientWithTracing(baseClient, "instance_id")
```

Now, `tracedClient` can be used just like the original client, with the added benefit of OpenTelemetry tracing.

## Links

- [gowrap](http://github.com/hexdigest/gowrap): The `gowrap` tool used for code generation.
- [OpenTelemetry](https://opentelemetry.io/): OpenTelemetry project for observability and instrumentation.
