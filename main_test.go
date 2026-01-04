package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/go-plugins-helpers/volume"
)

// setupTestDriver creates a temporary directory and initializes a driver for testing
func setupTestDriver(t *testing.T) (*sshfsDriver, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "sshfs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create state directory
	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	driver, err := newSshfsDriver(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create driver: %v", err)
	}

	return driver, tmpDir
}

// cleanupTestDriver removes the temporary directory
func cleanupTestDriver(tmpDir string) {
	os.RemoveAll(tmpDir)
}

// TestNewSshfsDriver tests driver initialization
func TestNewSshfsDriver(t *testing.T) {
	t.Run("new driver with empty state", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		if driver == nil {
			t.Fatal("Expected driver to be initialized")
		}

		if driver.root != filepath.Join(tmpDir, "volumes") {
			t.Errorf("Expected root to be %s, got %s", filepath.Join(tmpDir, "volumes"), driver.root)
		}

		if driver.statePath != filepath.Join(tmpDir, "state", "sshfs-state.json") {
			t.Errorf("Expected statePath to be %s, got %s", filepath.Join(tmpDir, "state", "sshfs-state.json"), driver.statePath)
		}

		if len(driver.volumes) != 0 {
			t.Errorf("Expected no volumes, got %d", len(driver.volumes))
		}
	})

	t.Run("new driver with existing state", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "sshfs-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer cleanupTestDriver(tmpDir)

		stateDir := filepath.Join(tmpDir, "state")
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			t.Fatalf("Failed to create state dir: %v", err)
		}

		// Create a state file with existing volumes
		statePath := filepath.Join(stateDir, "sshfs-state.json")
		existingState := map[string]*sshfsVolume{
			"test-volume": {
				Sshcmd:      "user@host:/path",
				Password:    "password",
				Port:        "22",
				Mountpoint:  "/mnt/test",
				connections: 0,
			},
		}
		data, err := json.Marshal(existingState)
		if err != nil {
			t.Fatalf("Failed to marshal state: %v", err)
		}
		if err := os.WriteFile(statePath, data, 0o644); err != nil {
			t.Fatalf("Failed to write state file: %v", err)
		}

		driver, err := newSshfsDriver(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create driver: %v", err)
		}

		if len(driver.volumes) != 1 {
			t.Errorf("Expected 1 volume, got %d", len(driver.volumes))
		}

		vol, ok := driver.volumes["test-volume"]
		if !ok {
			t.Fatal("Expected test-volume to be loaded")
		}

		if vol.Sshcmd != "user@host:/path" {
			t.Errorf("Expected Sshcmd to be user@host:/path, got %s", vol.Sshcmd)
		}
	})
}

// TestSaveState tests state persistence
func TestSaveState(t *testing.T) {
	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	// Add a volume
	driver.volumes["test-volume"] = &sshfsVolume{
		Sshcmd:      "user@host:/path",
		Password:    "secret",
		Port:        "2222",
		Mountpoint:  "/mnt/test",
		connections: 1,
	}

	// Save state
	driver.saveState()

	// Read the state file
	data, err := os.ReadFile(driver.statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var loadedVolumes map[string]*sshfsVolume
	if err := json.Unmarshal(data, &loadedVolumes); err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	vol, ok := loadedVolumes["test-volume"]
	if !ok {
		t.Fatal("Expected test-volume in saved state")
	}

	if vol.Sshcmd != "user@host:/path" {
		t.Errorf("Expected Sshcmd to be user@host:/path, got %s", vol.Sshcmd)
	}

	if vol.Password != "secret" {
		t.Errorf("Expected Password to be secret, got %s", vol.Password)
	}

	if vol.Port != "2222" {
		t.Errorf("Expected Port to be 2222, got %s", vol.Port)
	}
}

// TestCreate tests volume creation
func TestCreate(t *testing.T) {
	t.Run("create volume with sshcmd", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.CreateRequest{
			Name: "test-volume",
			Options: map[string]string{
				"sshcmd":   "user@host:/path",
				"password": "secret",
				"port":     "2222",
			},
		}

		err := driver.Create(req)
		if err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}

		vol, ok := driver.volumes["test-volume"]
		if !ok {
			t.Fatal("Expected volume to be created")
		}

		if vol.Sshcmd != "user@host:/path" {
			t.Errorf("Expected Sshcmd to be user@host:/path, got %s", vol.Sshcmd)
		}

		if vol.Password != "secret" {
			t.Errorf("Expected Password to be secret, got %s", vol.Password)
		}

		if vol.Port != "2222" {
			t.Errorf("Expected Port to be 2222, got %s", vol.Port)
		}

		if vol.connections != 0 {
			t.Errorf("Expected connections to be 0, got %d", vol.connections)
		}
	})

	t.Run("create volume with options", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.CreateRequest{
			Name: "test-volume",
			Options: map[string]string{
				"sshcmd":      "user@host:/path",
				"allow_other": "",
				"compression": "yes",
			},
		}

		err := driver.Create(req)
		if err != nil {
			t.Fatalf("Failed to create volume: %v", err)
		}

		vol := driver.volumes["test-volume"]
		if len(vol.Options) != 2 {
			t.Errorf("Expected 2 options, got %d", len(vol.Options))
		}

		// Check if options are present
		hasAllowOther := false
		hasCompression := false
		for _, opt := range vol.Options {
			if opt == "allow_other" {
				hasAllowOther = true
			}
			if opt == "compression=yes" {
				hasCompression = true
			}
		}

		if !hasAllowOther {
			t.Error("Expected allow_other option")
		}

		if !hasCompression {
			t.Error("Expected compression=yes option")
		}
	})

	t.Run("create volume without sshcmd fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.CreateRequest{
			Name: "test-volume",
			Options: map[string]string{
				"password": "secret",
			},
		}

		err := driver.Create(req)
		if err == nil {
			t.Fatal("Expected error when creating volume without sshcmd")
		}
	})
}

// TestRemove tests volume removal
func TestRemove(t *testing.T) {
	t.Run("remove existing volume", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")

		// Create a volume
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  mountpoint,
			connections: 0,
		}

		// Create the mountpoint directory
		if err := os.MkdirAll(mountpoint, 0o755); err != nil {
			t.Fatalf("Failed to create mountpoint: %v", err)
		}

		req := &volume.RemoveRequest{
			Name: "test-volume",
		}

		err := driver.Remove(req)
		if err != nil {
			t.Fatalf("Failed to remove volume: %v", err)
		}

		if _, ok := driver.volumes["test-volume"]; ok {
			t.Error("Expected volume to be removed")
		}

		// Verify mountpoint is removed
		if _, err := os.Stat(mountpoint); !os.IsNotExist(err) {
			t.Error("Expected mountpoint to be removed")
		}
	})

	t.Run("remove non-existent volume fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.RemoveRequest{
			Name: "non-existent",
		}

		err := driver.Remove(req)
		if err == nil {
			t.Fatal("Expected error when removing non-existent volume")
		}
	})

	t.Run("remove volume with active connections fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:      "user@host:/path",
			Mountpoint:  filepath.Join(tmpDir, "volumes", "test"),
			connections: 1,
		}

		req := &volume.RemoveRequest{
			Name: "test-volume",
		}

		err := driver.Remove(req)
		if err == nil {
			t.Fatal("Expected error when removing volume with active connections")
		}

		if _, ok := driver.volumes["test-volume"]; !ok {
			t.Error("Expected volume to still exist")
		}
	})
}

// TestPath tests getting volume path
func TestPath(t *testing.T) {
	t.Run("get path for existing volume", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:     "user@host:/path",
			Mountpoint: mountpoint,
		}

		req := &volume.PathRequest{
			Name: "test-volume",
		}

		resp, err := driver.Path(req)
		if err != nil {
			t.Fatalf("Failed to get path: %v", err)
		}

		if resp.Mountpoint != mountpoint {
			t.Errorf("Expected mountpoint to be %s, got %s", mountpoint, resp.Mountpoint)
		}
	})

	t.Run("get path for non-existent volume fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.PathRequest{
			Name: "non-existent",
		}

		_, err := driver.Path(req)
		if err == nil {
			t.Fatal("Expected error when getting path for non-existent volume")
		}
	})
}

// TestGet tests getting volume info
func TestGet(t *testing.T) {
	t.Run("get existing volume", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		mountpoint := filepath.Join(tmpDir, "volumes", "test")
		driver.volumes["test-volume"] = &sshfsVolume{
			Sshcmd:     "user@host:/path",
			Mountpoint: mountpoint,
		}

		req := &volume.GetRequest{
			Name: "test-volume",
		}

		resp, err := driver.Get(req)
		if err != nil {
			t.Fatalf("Failed to get volume: %v", err)
		}

		if resp.Volume.Name != "test-volume" {
			t.Errorf("Expected volume name to be test-volume, got %s", resp.Volume.Name)
		}

		if resp.Volume.Mountpoint != mountpoint {
			t.Errorf("Expected mountpoint to be %s, got %s", mountpoint, resp.Volume.Mountpoint)
		}
	})

	t.Run("get non-existent volume fails", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		req := &volume.GetRequest{
			Name: "non-existent",
		}

		_, err := driver.Get(req)
		if err == nil {
			t.Fatal("Expected error when getting non-existent volume")
		}
	})
}

// TestList tests listing volumes
func TestList(t *testing.T) {
	t.Run("list empty volumes", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		resp, err := driver.List()
		if err != nil {
			t.Fatalf("Failed to list volumes: %v", err)
		}

		if len(resp.Volumes) != 0 {
			t.Errorf("Expected 0 volumes, got %d", len(resp.Volumes))
		}
	})

	t.Run("list multiple volumes", func(t *testing.T) {
		driver, tmpDir := setupTestDriver(t)
		defer cleanupTestDriver(tmpDir)

		driver.volumes["volume1"] = &sshfsVolume{
			Sshcmd:     "user@host1:/path1",
			Mountpoint: filepath.Join(tmpDir, "volumes", "vol1"),
		}

		driver.volumes["volume2"] = &sshfsVolume{
			Sshcmd:     "user@host2:/path2",
			Mountpoint: filepath.Join(tmpDir, "volumes", "vol2"),
		}

		resp, err := driver.List()
		if err != nil {
			t.Fatalf("Failed to list volumes: %v", err)
		}

		if len(resp.Volumes) != 2 {
			t.Errorf("Expected 2 volumes, got %d", len(resp.Volumes))
		}

		// Check if both volumes are in the list
		volumeNames := make(map[string]bool)
		for _, vol := range resp.Volumes {
			volumeNames[vol.Name] = true
		}

		if !volumeNames["volume1"] {
			t.Error("Expected volume1 in list")
		}

		if !volumeNames["volume2"] {
			t.Error("Expected volume2 in list")
		}
	})
}

// TestCapabilities tests driver capabilities
func TestCapabilities(t *testing.T) {
	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	resp := driver.Capabilities()
	if resp.Capabilities.Scope != "local" {
		t.Errorf("Expected scope to be local, got %s", resp.Capabilities.Scope)
	}
}

// TestMountpoint tests mountpoint generation
func TestMountpointGeneration(t *testing.T) {
	driver, tmpDir := setupTestDriver(t)
	defer cleanupTestDriver(tmpDir)

	// Create two volumes with different sshcmd values
	req1 := &volume.CreateRequest{
		Name: "volume1",
		Options: map[string]string{
			"sshcmd": "user@host1:/path1",
		},
	}

	req2 := &volume.CreateRequest{
		Name: "volume2",
		Options: map[string]string{
			"sshcmd": "user@host2:/path2",
		},
	}

	driver.Create(req1)
	driver.Create(req2)

	vol1 := driver.volumes["volume1"]
	vol2 := driver.volumes["volume2"]

	// Mountpoints should be different
	if vol1.Mountpoint == vol2.Mountpoint {
		t.Error("Expected different mountpoints for different sshcmd values")
	}

	// Create a volume with the same sshcmd as volume1
	req3 := &volume.CreateRequest{
		Name: "volume3",
		Options: map[string]string{
			"sshcmd": "user@host1:/path1",
		},
	}

	driver.Create(req3)
	vol3 := driver.volumes["volume3"]

	// Mountpoint should be the same as volume1
	if vol1.Mountpoint != vol3.Mountpoint {
		t.Error("Expected same mountpoint for same sshcmd value")
	}
}

// TestLogError tests the logError function
func TestLogError(t *testing.T) {
	err := logError("test error: %s", "message")
	if err == nil {
		t.Fatal("Expected error to be returned")
	}

	if err.Error() != "test error: message" {
		t.Errorf("Expected error message to be 'test error: message', got '%s'", err.Error())
	}
}
