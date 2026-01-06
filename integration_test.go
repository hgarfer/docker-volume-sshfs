//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

// integrationTestConfig holds configuration for integration tests
type integrationTestConfig struct {
	skipIfNotAvailable bool
	sshHost            string
	sshPort            string
	sshUser            string
	sshPassword        string
	sshKeyPath         string
}

// getIntegrationConfig returns configuration for integration tests from environment
func getIntegrationConfig() *integrationTestConfig {
	return &integrationTestConfig{
		skipIfNotAvailable: os.Getenv("INTEGRATION_TESTS") != "1",
		sshHost:            getEnvOrDefault("SSH_TEST_HOST", "127.0.0.1"),
		sshPort:            getEnvOrDefault("SSH_TEST_PORT", "2222"),
		sshUser:            getEnvOrDefault("SSH_TEST_USER", "root"),
		sshPassword:        getEnvOrDefault("SSH_TEST_PASSWORD", "root"),
		sshKeyPath:         os.Getenv("SSH_TEST_KEY_PATH"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// checkSSHDAvailable checks if the SSH server is available
func checkSSHDAvailable(config *integrationTestConfig) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nc", "-z", config.sshHost, config.sshPort)
	err := cmd.Run()
	return err == nil
}

// setupSSHDContainer starts an SSH server container for testing
func setupSSHDContainer(t *testing.T) (string, func()) {
	t.Helper()

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping integration tests")
	}

	// Build the SSH container
	ctx := context.Background()
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", "sshfs-test-sshd", "testdata/ssh")
	buildCmd.Dir = "."
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("Failed to build SSH container: %v\n%s", err, output)
	}

	// Start the SSH container
	containerName := fmt.Sprintf("sshfs-test-%d", time.Now().Unix())
	startCmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", containerName, "-p", "2222:22", "sshfs-test-sshd")
	output, err := startCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start SSH container: %v\n%s", err, output)
	}

	containerID := strings.TrimSpace(string(output))

	// Wait for SSH to be ready
	time.Sleep(3 * time.Second)

	// Return cleanup function
	cleanup := func() {
		stopCmd := exec.Command("docker", "stop", containerID)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerID)
		rmCmd.Run()
	}

	return containerID, cleanup
}

// TestIntegrationFullWorkflow tests the complete workflow with a real SSH server
func TestIntegrationFullWorkflow(t *testing.T) {
	config := getIntegrationConfig()
	if config.skipIfNotAvailable {
		t.Skip("Skipping integration tests - set INTEGRATION_TESTS=1 to run")
	}

	// Check if we can connect to SSH server
	if !checkSSHDAvailable(config) {
		// Try to start SSH container
		_, cleanup := setupSSHDContainer(t)
		defer cleanup()

		// Wait a bit more for SSH to be ready
		time.Sleep(2 * time.Second)

		if !checkSSHDAvailable(config) {
			t.Skip("SSH server not available for integration tests")
		}
	}

	t.Run("complete volume lifecycle with password", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)

		// Create volume
		createReq := &volume.CreateRequest{
			Name: "integration-test-volume",
			Options: map[string]string{
				"sshcmd":   sshcmd,
				"password": config.sshPassword,
				"port":     config.sshPort,
			},
		}

		if err := driver.Create(createReq); err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}

		// Verify volume exists
		getReq := &volume.GetRequest{Name: "integration-test-volume"}
		getResp, err := driver.Get(getReq)
		if err != nil {
			t.Fatalf("Failed to get volume: %v", err)
		}

		if getResp.Volume.Name != "integration-test-volume" {
			t.Errorf("Expected volume name integration-test-volume, got %s", getResp.Volume.Name)
		}

		// Mount volume
		mountReq := &volume.MountRequest{
			Name: "integration-test-volume",
			ID:   "test-container",
		}

		mountResp, err := driver.Mount(mountReq)
		if err != nil {
			t.Fatalf("Failed to mount volume: %v", err)
		}

		if mountResp.Mountpoint == "" {
			t.Error("Expected non-empty mountpoint")
		}

		// Verify mount is active
		vol := driver.volumes["integration-test-volume"]
		if vol.connections != 1 {
			t.Errorf("Expected 1 connection, got %d", vol.connections)
		}

		// Unmount volume
		unmountReq := &volume.UnmountRequest{
			Name: "integration-test-volume",
			ID:   "test-container",
		}

		if err := driver.Unmount(unmountReq); err != nil {
			t.Fatalf("Failed to unmount volume: %v", err)
		}

		// Verify unmount
		if vol.connections != 0 {
			t.Errorf("Expected 0 connections after unmount, got %d", vol.connections)
		}

		// Remove volume
		removeReq := &volume.RemoveRequest{Name: "integration-test-volume"}
		if err := driver.Remove(removeReq); err != nil {
			t.Fatalf("Failed to remove volume: %v", err)
		}

		// Verify volume is removed
		_, err = driver.Get(getReq)
		if err == nil {
			t.Error("Expected error when getting removed volume")
		}
	})

	t.Run("volume with custom options", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)

		// Create volume with custom options
		createReq := &volume.CreateRequest{
			Name: "custom-options-volume",
			Options: map[string]string{
				"sshcmd":      sshcmd,
				"password":    config.sshPassword,
				"port":        config.sshPort,
				"Compression": "yes",
			},
		}

		if err := driver.Create(createReq); err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}

		vol := driver.volumes["custom-options-volume"]
		if len(vol.Options) == 0 {
			t.Error("Expected custom options to be stored")
		}

		// Check if Compression option exists
		hasCompression := false
		for _, opt := range vol.Options {
			if strings.Contains(opt, "Compression") {
				hasCompression = true
				break
			}
		}

		if !hasCompression {
			t.Error("Expected Compression option to be present")
		}

		// Cleanup
		driver.Remove(&volume.RemoveRequest{Name: "custom-options-volume"})
	})
}

// TestIntegrationStatePersistence tests that state is persisted across driver restarts
func TestIntegrationStatePersistence(t *testing.T) {
	config := getIntegrationConfig()
	if config.skipIfNotAvailable {
		t.Skip("Skipping integration tests - set INTEGRATION_TESTS=1 to run")
	}

	tmpDir, err := os.MkdirTemp("", "sshfs-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state directory
	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	// Create first driver instance
	driver1, err := newSshfsDriver(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create first driver: %v", err)
	}

	// Create volumes
	sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)
	driver1.Create(&volume.CreateRequest{
		Name: "persistent-volume-1",
		Options: map[string]string{
			"sshcmd":   sshcmd,
			"password": config.sshPassword,
			"port":     config.sshPort,
		},
	})

	driver1.Create(&volume.CreateRequest{
		Name: "persistent-volume-2",
		Options: map[string]string{
			"sshcmd":   sshcmd,
			"password": config.sshPassword,
			"port":     config.sshPort,
		},
	})

	// Create second driver instance (simulating restart)
	driver2, err := newSshfsDriver(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second driver: %v", err)
	}

	// Verify volumes are loaded
	if len(driver2.volumes) != 2 {
		t.Errorf("Expected 2 volumes after restart, got %d", len(driver2.volumes))
	}

	// Verify volume details
	vol1, ok := driver2.volumes["persistent-volume-1"]
	if !ok {
		t.Error("Expected persistent-volume-1 to be loaded")
	} else {
		if vol1.Sshcmd != sshcmd {
			t.Errorf("Expected sshcmd %s, got %s", sshcmd, vol1.Sshcmd)
		}
	}

	vol2, ok := driver2.volumes["persistent-volume-2"]
	if !ok {
		t.Error("Expected persistent-volume-2 to be loaded")
	} else {
		if vol2.Port != config.sshPort {
			t.Errorf("Expected port %s, got %s", config.sshPort, vol2.Port)
		}
	}
}

// TestIntegrationMultipleConnections tests multiple containers connecting to the same volume
func TestIntegrationMultipleConnections(t *testing.T) {
	config := getIntegrationConfig()
	if config.skipIfNotAvailable {
		t.Skip("Skipping integration tests - set INTEGRATION_TESTS=1 to run")
	}

	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)

	// Create volume
	driver.Create(&volume.CreateRequest{
		Name: "shared-volume",
		Options: map[string]string{
			"sshcmd":   sshcmd,
			"password": config.sshPassword,
			"port":     config.sshPort,
		},
	})

	// Mount from multiple "containers"
	containerIDs := []string{"container-1", "container-2", "container-3"}
	for _, containerID := range containerIDs {
		_, err := driver.Mount(&volume.MountRequest{
			Name: "shared-volume",
			ID:   containerID,
		})
		if err != nil {
			t.Fatalf("Failed to mount volume for %s: %v", containerID, err)
		}
	}

	// Verify connections
	vol := driver.volumes["shared-volume"]
	if vol.connections != 3 {
		t.Errorf("Expected 3 connections, got %d", vol.connections)
	}

	// Unmount from all containers
	for _, containerID := range containerIDs {
		if err := driver.Unmount(&volume.UnmountRequest{
			Name: "shared-volume",
			ID:   containerID,
		}); err != nil {
			t.Fatalf("Failed to unmount volume for %s: %v", containerID, err)
		}
	}

	// Verify all connections are closed
	if vol.connections != 0 {
		t.Errorf("Expected 0 connections after all unmounts, got %d", vol.connections)
	}

	// Cleanup
	driver.Remove(&volume.RemoveRequest{Name: "shared-volume"})
}

// TestIntegrationErrorCases tests various error scenarios
func TestIntegrationErrorCases(t *testing.T) {
	config := getIntegrationConfig()
	if config.skipIfNotAvailable {
		t.Skip("Skipping integration tests - set INTEGRATION_TESTS=1 to run")
	}

	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	t.Run("mount with invalid credentials", func(t *testing.T) {
		sshcmd := fmt.Sprintf("invalid-user@%s:/tmp", config.sshHost)

		driver.Create(&volume.CreateRequest{
			Name: "invalid-creds-volume",
			Options: map[string]string{
				"sshcmd":   sshcmd,
				"password": "wrong-password",
				"port":     config.sshPort,
			},
		})

		_, err := driver.Mount(&volume.MountRequest{
			Name: "invalid-creds-volume",
			ID:   "test-container",
		})

		if err == nil {
			t.Error("Expected error when mounting with invalid credentials")
		}

		// Cleanup
		driver.Remove(&volume.RemoveRequest{Name: "invalid-creds-volume"})
	})

	t.Run("mount with invalid host", func(t *testing.T) {
		driver.Create(&volume.CreateRequest{
			Name: "invalid-host-volume",
			Options: map[string]string{
				"sshcmd":   "user@invalid-host-that-does-not-exist:/tmp",
				"password": "password",
				"port":     "22",
			},
		})

		_, err := driver.Mount(&volume.MountRequest{
			Name: "invalid-host-volume",
			ID:   "test-container",
		})

		if err == nil {
			t.Error("Expected error when mounting with invalid host")
		}

		// Cleanup
		driver.Remove(&volume.RemoveRequest{Name: "invalid-host-volume"})
	})

	t.Run("remove volume with active connections", func(t *testing.T) {
		sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)

		driver.Create(&volume.CreateRequest{
			Name: "active-volume",
			Options: map[string]string{
				"sshcmd":   sshcmd,
				"password": config.sshPassword,
				"port":     config.sshPort,
			},
		})

		// Mount the volume
		driver.Mount(&volume.MountRequest{
			Name: "active-volume",
			ID:   "test-container",
		})

		// Try to remove while mounted
		err := driver.Remove(&volume.RemoveRequest{Name: "active-volume"})
		if err == nil {
			t.Error("Expected error when removing volume with active connections")
		}

		// Cleanup
		driver.Unmount(&volume.UnmountRequest{
			Name: "active-volume",
			ID:   "test-container",
		})
		driver.Remove(&volume.RemoveRequest{Name: "active-volume"})
	})
}

// TestIntegrationListVolumes tests listing volumes in various scenarios
func TestIntegrationListVolumes(t *testing.T) {
	config := getIntegrationConfig()
	if config.skipIfNotAvailable {
		t.Skip("Skipping integration tests - set INTEGRATION_TESTS=1 to run")
	}

	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	sshcmd := fmt.Sprintf("%s@%s:/tmp", config.sshUser, config.sshHost)

	// Create multiple volumes
	volumeNames := []string{"list-vol-1", "list-vol-2", "list-vol-3"}
	for _, name := range volumeNames {
		driver.Create(&volume.CreateRequest{
			Name: name,
			Options: map[string]string{
				"sshcmd":   sshcmd,
				"password": config.sshPassword,
				"port":     config.sshPort,
			},
		})
	}

	// List volumes
	listResp, err := driver.List()
	if err != nil {
		t.Fatalf("Failed to list volumes: %v", err)
	}

	if len(listResp.Volumes) != 3 {
		t.Errorf("Expected 3 volumes, got %d", len(listResp.Volumes))
	}

	// Verify all volumes are in the list
	foundVolumes := make(map[string]bool)
	for _, vol := range listResp.Volumes {
		foundVolumes[vol.Name] = true
	}

	for _, name := range volumeNames {
		if !foundVolumes[name] {
			t.Errorf("Expected volume %s in list", name)
		}
	}

	// Cleanup
	for _, name := range volumeNames {
		driver.Remove(&volume.RemoveRequest{Name: name})
	}
}
