package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"sender/internal/wormhole"

	wh "github.com/psanford/wormhole-william/wormhole"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct manages the application lifecycle and secure transfers
type App struct {
	ctx           context.Context
	cancelContext context.Context
	cancelFunc    context.CancelFunc
	mu            sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectFile opens a native dialog to select a single file.
func (a *App) SelectFile() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File to Send",
	})
	if err != nil {
		return "", err
	}
	return file, nil
}

// SelectDirectory opens a native dialog to select a directory.
func (a *App) SelectDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder to Send",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// GetDefaultSaveDir returns the path to the user's Downloads folder.
func (a *App) GetDefaultSaveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	downloads := filepath.Join(home, "Downloads")
	// Verify if downloads exists, if not, fallback to home directory
	if _, err := os.Stat(downloads); os.IsNotExist(err) {
		return home, nil
	}
	return downloads, nil
}

// OpenFolder opens the folder containing the downloaded file or directory in Finder.
func (a *App) OpenFolder(path string) error {
	info, err := os.Stat(path)
	var dirToOpen string
	if err == nil && info.IsDir() {
		dirToOpen = path
	} else {
		dirToOpen = filepath.Dir(path)
	}

	var cmd *exec.Cmd
	switch osPlatform := runtime.Environment(a.ctx).Platform; osPlatform {
	case "darwin":
		cmd = exec.Command("open", dirToOpen)
	case "windows":
		cmd = exec.Command("explorer", dirToOpen)
	default: // Linux
		cmd = exec.Command("xdg-open", dirToOpen)
	}
	return cmd.Run()
}

// CancelTransfer aborts any active transfer by cancelling its context.
func (a *App) CancelTransfer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelFunc != nil {
		a.cancelFunc()
		a.cancelFunc = nil
	}
}

// Send starts the Magic Wormhole send process in a background goroutine.
func (a *App) Send(path string) (string, error) {
	if path == "" {
		return "", errors.New("no file or folder selected")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("cannot read path info: %w", err)
	}

	var finalPath string
	var fileName string
	var isDir bool
	var tempFile *os.File

	// If the path is a directory, compress it to a temporary zip archive first.
	if info.IsDir() {
		isDir = true
		fileName = info.Name() + ".zip"

		// Send compression status update to the frontend
		runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
			"status": "compressing",
			"role":   "send",
		})

		tempFile, err = os.CreateTemp("", "wormhole-send-*.zip")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary archive: %w", err)
		}
		tempPath := tempFile.Name()
		tempFile.Close() // Close file handle so ZipDirectory can write to it

		err = wormhole.ZipDirectory(path, tempPath)
		if err != nil {
			os.Remove(tempPath)
			return "", fmt.Errorf("failed to zip directory: %w", err)
		}

		finalPath = tempPath
	} else {
		finalPath = path
		fileName = info.Name()
	}

	// Open the file/archive for sending
	file, err := os.Open(finalPath)
	if err != nil {
		if tempFile != nil {
			os.Remove(tempFile.Name())
		}
		return "", fmt.Errorf("failed to open transfer source: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		if tempFile != nil {
			os.Remove(tempFile.Name())
		}
		return "", fmt.Errorf("failed to read transfer source details: %w", err)
	}
	fileSize := stat.Size()

	// Handle thread-safe transfer context management
	a.mu.Lock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.cancelContext, a.cancelFunc = context.WithCancel(a.ctx)
	transferCtx := a.cancelContext
	a.mu.Unlock()

	// Initialize our custom progress-tracking reader
	progressReader := wormhole.NewProgressReadSeeker(file, fileSize, transferCtx, a.ctx, "send")

	var client wh.Client
	code, statusChan, err := client.SendFile(transferCtx, fileName, progressReader)
	if err != nil {
		file.Close()
		if tempFile != nil {
			os.Remove(tempFile.Name())
		}
		return "", fmt.Errorf("failed to initialize wormhole handshaking: %w", err)
	}

	// Run status listener in the background
	go func() {
		defer file.Close()
		if tempFile != nil {
			defer os.Remove(tempFile.Name())
		}

		// Emit waiting state containing the wormhole code
		runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
			"status":   "waiting",
			"role":     "send",
			"code":     code,
			"fileName": fileName,
			"fileSize": fileSize,
			"isDir":    isDir,
		})

		select {
		case <-transferCtx.Done():
			runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
				"status": "failed",
				"role":   "send",
				"error":  "Transfer cancelled by user",
			})
			return
		case result := <-statusChan:
			if result.Error != nil {
				errMsg := result.Error.Error()
				if errors.Is(result.Error, context.Canceled) {
					errMsg = "Transfer cancelled by user"
				} else if strings.Contains(errMsg, "timeout") {
					errMsg = "Connection timed out"
				} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "broken pipe") {
					errMsg = "Connection lost. Peer disconnected."
				}

				runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
					"status": "failed",
					"role":   "send",
					"error":  errMsg,
				})
			} else {
				progressReader.ForceComplete()
				runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
					"status": "completed",
					"role":   "send",
				})
			}
		}
	}()

	return code, nil
}

// Receive starts the Magic Wormhole receive process in a background goroutine.
func (a *App) Receive(code string, saveDir string) error {
	if code == "" {
		return errors.New("no code provided")
	}

	// If no save directory specified, default to Downloads
	if saveDir == "" {
		downloads, err := a.GetDefaultSaveDir()
		if err != nil {
			return fmt.Errorf("failed to access Downloads folder: %w", err)
		}
		saveDir = downloads
	}

	// Validate output directory
	dirStat, err := os.Stat(saveDir)
	if err != nil || !dirStat.IsDir() {
		return fmt.Errorf("invalid save directory: %s", saveDir)
	}

	// Initialize thread-safe cancellation context
	a.mu.Lock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	a.cancelContext, a.cancelFunc = context.WithCancel(a.ctx)
	transferCtx := a.cancelContext
	a.mu.Unlock()

	go func() {
		// Emit initial connecting state
		runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
			"status": "connecting",
			"role":   "receive",
		})

		var client wh.Client
		msg, err := client.Receive(transferCtx, code)
		if err != nil {
			errMsg := err.Error()
			if errors.Is(err, context.Canceled) {
				errMsg = "Transfer cancelled by user"
			} else if strings.Contains(errMsg, "timeout") {
				errMsg = "Connection timed out"
			} else if strings.Contains(errMsg, "bad key") || strings.Contains(errMsg, "decrypt") {
				errMsg = "Invalid wormhole code or transfer rejected by peer"
			}

			runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
				"status": "failed",
				"role":   "receive",
				"error":  errMsg,
			})
			return
		}

		fileName := msg.Name
		isZipFolder := msg.Type == wh.TransferDirectory || strings.HasSuffix(strings.ToLower(fileName), ".zip")

		var finalPath string
		var tempZipPath string

		if isZipFolder {
			// Download to a temporary zip file
			tempFile, err := os.CreateTemp("", "wormhole-recv-*.zip")
			if err != nil {
				msg.Reject()
				runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
					"status": "failed",
					"role":   "receive",
					"error":  "Failed to create temporary archive file",
				})
				return
			}
			tempZipPath = tempFile.Name()
			tempFile.Close() // Close handle so we can write content
			finalPath = tempZipPath
		} else {
			finalPath = filepath.Join(saveDir, fileName)
		}

		// Emit active transfer state
		runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
			"status":   "active",
			"role":     "receive",
			"fileName": fileName,
			"fileSize": msg.TransferBytes64,
			"isDir":    isZipFolder,
		})

		// Open local file for writing
		outFile, err := os.OpenFile(finalPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			msg.Reject()
			if tempZipPath != "" {
				os.Remove(tempZipPath)
			}
			runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
				"status": "failed",
				"role":   "receive",
				"error":  fmt.Sprintf("Failed to write to destination: %s", err.Error()),
			})
			return
		}

		// Wrap writer with our progress tracker
		progressWriter := wormhole.NewProgressWriter(outFile, msg.TransferBytes64, transferCtx, a.ctx, "receive")

		// Stream content
		_, copyErr := io.Copy(progressWriter, msg)
		outFile.Close()

		if copyErr != nil {
			if tempZipPath != "" {
				os.Remove(tempZipPath)
			} else {
				os.Remove(finalPath)
			}

			errMsg := copyErr.Error()
			if errors.Is(copyErr, context.Canceled) {
				errMsg = "Transfer cancelled by user"
			} else if strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "broken pipe") {
				errMsg = "Connection lost. Peer disconnected."
			}

			runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
				"status": "failed",
				"role":   "receive",
				"error":  errMsg,
			})
			return
		}

		progressWriter.ForceComplete()

		// If it's a folder, extract the contents and remove the zip archive
		if isZipFolder {
			runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
				"status": "decompressing",
				"role":   "receive",
			})

			extractErr := wormhole.UnzipDirectory(tempZipPath, saveDir)
			os.Remove(tempZipPath)

			if extractErr != nil {
				runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
					"status": "failed",
					"role":   "receive",
					"error":  fmt.Sprintf("Extraction failed: %s", extractErr.Error()),
				})
				return
			}
		}

		// Emit completed state
		runtime.EventsEmit(a.ctx, "transfer:status", map[string]interface{}{
			"status": "completed",
			"role":   "receive",
			"path":   filepath.Join(saveDir, strings.TrimSuffix(fileName, ".zip")),
		})
	}()

	return nil
}
