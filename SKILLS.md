# SKILL.md: MagicWormhole Desktop (Wails + Vue/Vuetify)

This document outlines the technical competencies and conceptual domains required to build a cross-platform secure file transfer application.

---

## 1. Core Architecture & Wails Fundamentals
* **Wails Lifecycle Management:** Mastering the bridge between Go and JavaScript. Implementing `wails.json` and managing the `frontend/` vs `internal/` directory structure.
* **Contextual Bridge:** Implementing `runtime.EventsEmit` for asynchronous progress updates (transfer speed, percentage, remaining time) from Go to Vue.
* **Cross-Platform Binaries:** Knowledge of OS-specific build constraints and handling file system permissions across Windows, macOS, and Linux.

## 2. Go Backend (The "Wormhole" Engine)
* **Wormhole-William Integration:** Integrating `wormhole-william` as a library rather than a CLI tool. 
    * Implementing custom `io.Reader` and `io.Writer` interfaces to intercept the data stream for real-time progress tracking.
* **Concurrency Patterns:** Using `context.Context` to handle user cancellations. Managing Goroutines to keep the GUI responsive during intense encryption/decryption tasks.
* **File System I/O:** Safely navigating paths using `os`, `path/filepath`, and `archive/zip` (necessary for recursive folder transfers).

## 3. Frontend & UX (Vue + Vuetify)
* **Vuetify 3 Implementation:** Utilizing `v-file-input` or custom Drag-and-Drop zones. Designing secure input forms for the 6-word codes (e.g., `v-otp-input`).
* **State Management:** Using Pinia to track the "Transfer State Machine": `IDLE` -> `SENDING_INITIALIZING` -> `SENDING_ACTIVE` -> `COMPLETED`.
* **Responsive UI:** Creating a clean, high-feedback interface focusing on UX patterns similar to modern file-sharing applications.

## 4. Security & Privacy
* **Zero-Knowledge Principles:** Understanding the "Magic Wormhole" key exchange. Ensuring the app never logs or stores the file content.
* **Temporary File Handling:** Securing the "staging" area for decryption. Implementing cleanup routines for partial/interrupted transfers.

## 5. Build & Distribution
* **Dependency Management:** Strict use of **pnpm** for frontend dependencies and **Astral uv** for Python (if needed for build scripts) or standard **go mod** for backend.
* **Bundling Assets:** Configuring `embed` files for any static assets (icons, loaders) required by the Go runtime.
* **Packaging:** Automating the creation of signed binaries for your target OS environments.

---

## Areas You Might Have Missed

### A. Folder Archiving Strategy
`wormhole-william` is optimized for single file streams. You must implement:
1.  **Compression:** Logic to detect directories, archive them (e.g., `.tar.gz`), and stream the result.
2.  **Decompression:** Automated extraction logic on the recipient's machine after a successful stream.

### B. "Wormhole" Code UX
The 6-word code can be tedious. 
* **Action:** Implement a "Copy to Clipboard" trigger for the sender.
* **Action:** Implement an auto-detect feature on the receive page that detects valid wormhole strings on the clipboard and auto-fills the input.

### C. Network Resilience
File transfers are prone to environment-specific drops.
* **Implementation:** An error-handling layer in Go that catches library timeouts and maps them to human-readable alerts (e.g., "Connection lost," "Peer disconnected") to be shown via a Vuetify `v-snackbar`.

### D. File System Permissions
Desktop apps often encounter permission errors when writing to sensitive system folders.
* **Best Practice:** Target the user's `Downloads` directory using `os.UserHomeDir()` by default, with a configuration option to set a custom destination folder.