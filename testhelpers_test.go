package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// MockCommandExecutor is an interface for executing commands
type MockCommandExecutor interface {
	Execute(name string, args ...string) ([]byte, error)
}

// RealCommandExecutor executes real commands
type RealCommandExecutor struct{}

func (e *RealCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// TestCommandExecutor is a mock for testing
type TestCommandExecutor struct {
	commands [][]string
	outputs  [][]byte
	errors   []error
	callIdx  int
}

func NewTestCommandExecutor() *TestCommandExecutor {
	return &TestCommandExecutor{
		commands: make([][]string, 0),
		outputs:  make([][]byte, 0),
		errors:   make([]error, 0),
		callIdx:  0,
	}
}

func (e *TestCommandExecutor) AddMockResponse(output []byte, err error) {
	e.outputs = append(e.outputs, output)
	e.errors = append(e.errors, err)
}

func (e *TestCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
	fullCmd := append([]string{name}, args...)
	e.commands = append(e.commands, fullCmd)

	if e.callIdx < len(e.outputs) {
		output := e.outputs[e.callIdx]
		err := e.errors[e.callIdx]
		e.callIdx++
		return output, err
	}

	return nil, fmt.Errorf("no mock response configured for call %d", e.callIdx)
}

func (e *TestCommandExecutor) GetCommands() [][]string {
	return e.commands
}

func (e *TestCommandExecutor) GetCommandCount() int {
	return len(e.commands)
}

func (e *TestCommandExecutor) Reset() {
	e.commands = make([][]string, 0)
	e.outputs = make([][]byte, 0)
	e.errors = make([]error, 0)
	e.callIdx = 0
}

// AssertCommand verifies that a specific command was executed
func (e *TestCommandExecutor) AssertCommand(t *testing.T, expectedCmd string) bool {
	t.Helper()
	for _, cmd := range e.commands {
		if strings.Join(cmd, " ") == expectedCmd {
			return true
		}
	}
	t.Errorf("Expected command '%s' was not executed. Commands: %v", expectedCmd, e.commands)
	return false
}

// AssertCommandContains verifies that a command containing the substring was executed
func (e *TestCommandExecutor) AssertCommandContains(t *testing.T, substring string) bool {
	t.Helper()
	for _, cmd := range e.commands {
		if strings.Contains(strings.Join(cmd, " "), substring) {
			return true
		}
	}
	t.Errorf("Expected command containing '%s' was not executed. Commands: %v", substring, e.commands)
	return false
}

// CheckCommandExists checks if a command is available in PATH
func CheckCommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// SkipIfCommandMissing skips the test if a required command is not available
func SkipIfCommandMissing(t *testing.T, cmd string) {
	t.Helper()
	if !CheckCommandExists(cmd) {
		t.Skipf("Command '%s' not found, skipping test", cmd)
	}
}

// RequireCommand fails the test if a required command is not available
func RequireCommand(t *testing.T, cmd string) {
	t.Helper()
	if !CheckCommandExists(cmd) {
		t.Fatalf("Command '%s' is required but not found", cmd)
	}
}

// CreateTempSSHKey creates a temporary SSH key pair for testing
func CreateTempSSHKey(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "sshfs-test-keys-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir for SSH keys: %v", err)
	}

	keyPath := fmt.Sprintf("%s/id_rsa", tmpDir)

	// Generate SSH key
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to generate SSH key: %v\n%s", err, output)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return keyPath, cleanup
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// AssertFileExists asserts that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if !FileExists(path) {
		t.Errorf("Expected file '%s' to exist", path)
	}
}

// AssertFileNotExists asserts that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if FileExists(path) {
		t.Errorf("Expected file '%s' to not exist", path)
	}
}

// AssertDirExists asserts that a directory exists
func AssertDirExists(t *testing.T, path string) {
	t.Helper()
	if !DirExists(path) {
		t.Errorf("Expected directory '%s' to exist", path)
	}
}

// AssertDirNotExists asserts that a directory does not exist
func AssertDirNotExists(t *testing.T, path string) {
	t.Helper()
	if DirExists(path) {
		t.Errorf("Expected directory '%s' to not exist", path)
	}
}

// AssertEqual asserts that two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertNotEqual asserts that two values are not equal
func AssertNotEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected == actual {
		t.Errorf("%s: expected values to be different, but both were %v", msg, expected)
	}
}

// AssertError asserts that an error occurred
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error but got nil", msg)
	}
}

// AssertNoError asserts that no error occurred
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// AssertContains asserts that a string contains a substring
func AssertContains(t *testing.T, haystack, needle string, msg string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected '%s' to contain '%s'", msg, haystack, needle)
	}
}

// AssertNotContains asserts that a string does not contain a substring
func AssertNotContains(t *testing.T, haystack, needle string, msg string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("%s: expected '%s' to not contain '%s'", msg, haystack, needle)
	}
}

// TestTestHelpers tests the test helper functions
func TestTestHelpers(t *testing.T) {
	t.Run("mock command executor", func(t *testing.T) {
		executor := NewTestCommandExecutor()

		// Add mock responses
		executor.AddMockResponse([]byte("output1"), nil)
		executor.AddMockResponse([]byte("output2"), fmt.Errorf("error2"))

		// Execute commands
		output1, err1 := executor.Execute("cmd1", "arg1")
		if err1 != nil {
			t.Errorf("Expected no error for first command, got %v", err1)
		}
		if string(output1) != "output1" {
			t.Errorf("Expected output1, got %s", output1)
		}

		output2, err2 := executor.Execute("cmd2", "arg2")
		if err2 == nil {
			t.Error("Expected error for second command")
		}
		if string(output2) != "output2" {
			t.Errorf("Expected output2, got %s", output2)
		}

		// Verify commands were tracked
		if executor.GetCommandCount() != 2 {
			t.Errorf("Expected 2 commands, got %d", executor.GetCommandCount())
		}
	})

	t.Run("file and directory helpers", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "helper-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Test directory checks
		if !DirExists(tmpDir) {
			t.Error("Expected temp dir to exist")
		}

		// Test file checks
		testFile := fmt.Sprintf("%s/test.txt", tmpDir)
		if FileExists(testFile) {
			t.Error("Expected test file to not exist yet")
		}

		// Create file
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		if !FileExists(testFile) {
			t.Error("Expected test file to exist")
		}
	})
}
