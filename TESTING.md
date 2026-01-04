# Testing Guide

This document describes the test suite for docker-volume-sshfs and how to run the tests.

## Overview

The test suite has been completely rewritten to use modern Go testing best practices and replaces the old Travis CI shell scripts with native Go tests.

## Test Structure

The test suite is organized into several files:

- **main_test.go**: Unit tests for core driver functionality
  - Driver initialization and state management
  - Volume CRUD operations (Create, Remove, Get, List, Path)
  - State persistence and loading
  - Mountpoint generation
  - Capabilities

- **mount_test.go**: Tests for mount/unmount operations
  - Connection counting
  - Concurrent operations
  - Mountpoint creation

- **integration_test.go**: End-to-end integration tests
  - Full workflow with real SSH server
  - State persistence across driver restarts
  - Multiple concurrent connections
  - Error scenarios
  - Volume listing

- **testhelpers_test.go**: Reusable test utilities and helpers
  - Mock command executor
  - Assertion helpers
  - File/directory helpers
  - SSH key generation

## Running Tests

### Prerequisites

- Go 1.20 or later
- Docker (for integration tests)
- SSH client tools (for integration tests)

### Unit Tests

Run all unit tests (fastest, no external dependencies):

```bash
make test-unit
```

Or directly with go:

```bash
go test -v -race ./...
```

### Integration Tests

Integration tests require Docker and will automatically start an SSH server container.

```bash
make test-integration
```

Or with environment variables:

```bash
INTEGRATION_TESTS=1 go test -v -race -tags=integration ./...
```

#### Custom SSH Server

You can also test against a custom SSH server:

```bash
export INTEGRATION_TESTS=1
export SSH_TEST_HOST=your-ssh-server.com
export SSH_TEST_PORT=22
export SSH_TEST_USER=testuser
export SSH_TEST_PASSWORD=testpass
go test -v -tags=integration ./...
```

### All Tests

Run both unit and integration tests:

```bash
make test-unit
make test-integration
```

### Coverage

Generate coverage reports:

```bash
# Unit test coverage only
make test-coverage

# Full coverage (unit + integration)
make test-coverage-full
```

This will generate HTML coverage reports:
- `coverage.html` - Unit test coverage
- `coverage-full.html` - Combined coverage

## Code Quality

### Format Code

```bash
make fmt
```

### Check Formatting

```bash
make fmt-check
```

### Run Linter

```bash
make lint
```

Note: Requires golangci-lint to be installed:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Run go vet

```bash
make vet
```

### Run All Quality Checks

```bash
make ci-unit  # Runs: fmt-check, vet, and unit tests
```

## Continuous Integration

The test suite is designed to replace the old Travis CI tests:

### Old Travis Tests → New Test Commands

| Old Travis Test | New Command | Description |
|----------------|-------------|-------------|
| `.travis/unit.sh` | `make ci-unit` | Format check, vet, and unit tests |
| `.travis/integration.sh` | `make ci-integration` | Integration tests with Docker |

### CI Configuration

The tests are designed to work in any CI environment. Example for GitHub Actions:

```yaml
# Unit tests
- name: Run unit tests
  run: make ci-unit

# Integration tests
- name: Run integration tests
  run: make ci-integration
```

## Test Coverage

The test suite provides comprehensive coverage of:

### Driver Functionality
- ✅ Driver initialization
- ✅ State persistence and loading
- ✅ Volume creation with various options
- ✅ Volume removal
- ✅ Volume listing
- ✅ Getting volume information
- ✅ Getting volume paths
- ✅ Driver capabilities

### Mount Operations
- ✅ Mounting volumes
- ✅ Unmounting volumes
- ✅ Connection counting
- ✅ Multiple concurrent mounts
- ✅ Mountpoint creation
- ✅ Thread safety

### Integration Scenarios
- ✅ Full workflow with real SSH server
- ✅ Password authentication
- ✅ SSH key authentication (when configured)
- ✅ Custom SSH options (compression, ciphers, etc.)
- ✅ State persistence across restarts
- ✅ Multiple containers using same volume
- ✅ Error handling (invalid credentials, unreachable hosts, etc.)

### Edge Cases
- ✅ Concurrent operations
- ✅ Invalid inputs
- ✅ Missing required options
- ✅ Removing volumes with active connections
- ✅ Operations on non-existent volumes

## Writing New Tests

### Test File Naming

- `*_test.go` - Regular unit tests
- Tests with build tags should include the tag at the top:
  ```go
  //go:build integration
  // +build integration
  ```

### Using Test Helpers

The `testhelpers_test.go` file provides many useful utilities:

```go
// Setup a test driver
driver, tmpDir := setupTestDriver(t)
defer cleanupTestDriver(tmpDir)

// Assertions
AssertEqual(t, expected, actual, "checking value")
AssertError(t, err, "expected error")
AssertFileExists(t, "/path/to/file")

// Command mocking
executor := NewTestCommandExecutor()
executor.AddMockResponse([]byte("output"), nil)
```

### Test Structure

Follow this pattern for new tests:

```go
func TestFeature(t *testing.T) {
    t.Run("descriptive test case name", func(t *testing.T) {
        // Setup
        driver, tmpDir := setupTestDriver(t)
        defer cleanupTestDriver(tmpDir)

        // Execute
        result, err := driver.SomeOperation()

        // Assert
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if result != expected {
            t.Errorf("expected %v, got %v", expected, result)
        }
    })
}
```

## Troubleshooting

### Integration Tests Fail

If integration tests fail, check:

1. Docker is running: `docker ps`
2. Port 2222 is available
3. SSH server container can be built: `docker build -t sshfs-test-sshd testdata/ssh`

### Tests Hang

- Check for deadlocks in concurrent operations
- Ensure cleanup functions are being called (use `defer`)
- Verify test timeouts are appropriate

### Coverage Issues

- Run tests with `-v` flag to see which tests are running
- Use `-coverprofile` to identify uncovered code:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -func=coverage.out
  ```

## Performance

The test suite is designed for speed:

- Unit tests complete in < 5 seconds
- Integration tests complete in < 2 minutes
- Tests run in parallel where possible (`-race` flag)

## Best Practices

1. **Always use t.Helper()** in helper functions
2. **Clean up resources** with defer statements
3. **Use descriptive test names** that explain what's being tested
4. **Test one thing per test** - use subtests for related scenarios
5. **Mock external dependencies** in unit tests
6. **Test error cases** as thoroughly as success cases
7. **Run tests with -race** to detect race conditions

## Migration from Travis CI

The new test suite completely replaces the Travis CI shell scripts while providing:

- ✅ Better error messages
- ✅ Faster execution
- ✅ Native Go testing features
- ✅ Better code coverage tracking
- ✅ Easier to maintain and extend
- ✅ Works with any CI system
- ✅ Can run locally without special setup

Old Travis CI commands have direct equivalents:

```bash
# Old
./.travis/unit.sh
# New
make ci-unit

# Old
./.travis/integration.sh
# New
make ci-integration
```
