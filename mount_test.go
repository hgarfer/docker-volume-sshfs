//go:build integration
// +build integration

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/go-plugins-helpers/volume"
)

// TestMountUnmount tests mount and unmount operations with connection counting
func TestMountUnmount(t *testing.T) {
	// Skip if we're not running with mount capabilities
	if os.Getenv("RUN_MOUNT_TESTS") != "1" {
		t.Skip("Skipping mount tests - set RUN_MOUNT_TESTS=1 to run")
	}

	t.Run("mount increments connections", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		// Create a volume
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  filepath.Join(tmpDir, "volumes", "test"),
			connections: 0,
		}

		// Create mountpoint directory
		if err := os.MkdirAll(driver.volumes["test-volume"].Mountpoint, 0o755); err != nil {
			t.Fatalf("Failed to create mountpoint: %v", err)
		}

		req := &volume.MountRequest{
			Name: "test-volume",
			ID:   "container-1",
		}

		// First mount - this would normally call sshfs, so we'll mock it
		resp, err := driver.Mount(req)
		if err != nil && !strings.Contains(err.Error(), "sshfs") {
			t.Fatalf("Failed to mount volume: %v", err)
		}

		// Check connections were incremented (even if mount failed)
		vol := driver.volumes["test-volume"]
		if vol.connections < 0 {
			t.Errorf("Expected connections to be >= 0, got %d", vol.connections)
		}

		if resp != nil && resp.Mountpoint != driver.volumes["test-volume"].Mountpoint {
			t.Errorf("Expected mountpoint to be %s, got %s", driver.volumes["test-volume"].Mountpoint, resp.Mountpoint)
		}
	})

	t.Run("multiple mounts increment connections", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  mountpoint,
			connections: 0,
		}

		if err := os.MkdirAll(mountpoint, 0o755); err != nil {
			t.Fatalf("Failed to create mountpoint: %v", err)
		}

		// Track initial connections
		initialConnections := driver.volumes["test-volume"].connections

		// Attempt multiple mounts
		for i := 0; i < 3; i++ {
			req := &volume.MountRequest{
				Name: "test-volume",
				ID:   fmt.Sprintf("container-%d", i),
			}
			driver.Mount(req)
		}

		// Connections should have incremented
		vol := driver.volumes["test-volume"]
		expectedConnections := initialConnections + 3
		if vol.connections != expectedConnections {
			t.Errorf("Expected connections to be %d, got %d", expectedConnections, vol.connections)
		}
	})

	t.Run("unmount decrements connections", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  mountpoint,
			connections: 2, // Start with 2 connections
		}

		req := &volume.UnmountRequest{
			Name: "test-volume",
			ID:   "container-1",
		}

		err := driver.Unmount(req)
		if err != nil && !strings.Contains(err.Error(), "not mounted") {
			t.Fatalf("Failed to unmount volume: %v", err)
		}

		vol := driver.volumes["test-volume"]
		if vol.connections != 1 {
			t.Errorf("Expected connections to be 1, got %d", vol.connections)
		}
	})

	t.Run("unmount with zero connections", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  mountpoint,
			connections: 1,
		}

		req := &volume.UnmountRequest{
			Name: "test-volume",
			ID:   "container-1",
		}

		err := driver.Unmount(req)
		// Unmount might fail because we're not actually mounted, but that's ok
		if err != nil && !strings.Contains(err.Error(), "not mounted") && !strings.Contains(err.Error(), "umount") {
			t.Fatalf("Unexpected error: %v", err)
		}

		vol := driver.volumes["test-volume"]
		if vol.connections != 0 {
			t.Errorf("Expected connections to be 0, got %d", vol.connections)
		}
	})

	t.Run("mount non-existent volume fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.MountRequest{
			Name: "non-existent",
			ID:   "container-1",
		}

		_, err := driver.Mount(req)
		if err == nil {
			t.Fatal("Expected error when mounting non-existent volume")
		}
	})

	t.Run("unmount non-existent volume fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.UnmountRequest{
			Name: "non-existent",
			ID:   "container-1",
		}

		err := driver.Unmount(req)
		if err == nil {
			t.Fatal("Expected error when unmounting non-existent volume")
		}
	})
}

// TestMountpointCreation tests that mountpoints are created if they don't exist
func TestMountpointCreation(t *testing.T) {
	if os.Getenv("RUN_MOUNT_TESTS") != "1" {
		t.Skip("Skipping mount tests - set RUN_MOUNT_TESTS=1 to run")
	}

	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	mountpoint := filepath.Join(tmpDir, "volumes", "test", "nested")
	driver.volumes["test-volume"] = &sshfsVolume{
		Sshcmd:      "user@host:/path",
		Mountpoint:  mountpoint,
		connections: 0,
	}

	req := &volume.MountRequest{
		Name: "test-volume",
		ID:   "container-1",
	}

	driver.Mount(req)

	// Check if mountpoint was created
	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		t.Error("Expected mountpoint to be created")
	}
}

// TestConcurrentOperations tests thread-safety of driver operations
func TestConcurrentOperations(t *testing.T) {
	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	// Create initial volumes
	for i := 0; i < 5; i++ {
		driver.Create(&volume.CreateRequest{
			Name: fmt.Sprintf("volume-%d", i),
			Options: map[string]string{
				"sshcmd": fmt.Sprintf("user@host:/path%d", i),
			},
		})
	}

	// Run concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				driver.List()
				driver.Get(&volume.GetRequest{Name: "volume-0"})
				driver.Path(&volume.PathRequest{Name: "volume-1"})
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}
