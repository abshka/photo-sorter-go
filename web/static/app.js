/**
 * PhotoSorter Web Interface JavaScript
 * Main application logic for the web interface
 */

class PhotoSorterApp {
  constructor() {
    this.wsConnection = null;
    this.isConnected = false;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectInterval = 3000;
    this._compressionPollInterval = null;

    this.initializeWebSocket();
    this.bindEvents();
    this.startStatusPolling();
    this.loadConfig();
  }

  /**
   * Initialize WebSocket connection
   */
  initializeWebSocket() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    try {
      this.ws = new WebSocket(wsUrl);
      this.setupWebSocketHandlers();
    } catch (error) {
      this.log("Failed to create WebSocket connection: " + error.message, "error");
      this.scheduleReconnect();
    }
  }

  /**
   * Set up WebSocket event handlers
   */
  setupWebSocketHandlers() {
    this.ws.onopen = () => {
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.log("Connected to server", "info");
      this.updateConnectionStatus(true);
    };

    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this.handleWebSocketMessage(message);
      } catch (error) {
        this.log("Failed to parse WebSocket message: " + error.message, "error");
      }
    };

    this.ws.onclose = (event) => {
      this.isConnected = false;
      this.updateConnectionStatus(false);

      if (event.wasClean) {
        this.log("Connection closed cleanly", "info");
      } else {
        this.log("Connection lost. Attempting to reconnect...", "warning");
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      this.log("WebSocket error occurred", "error");
      this.isConnected = false;
      this.updateConnectionStatus(false);
    };
  }

  /**
   * Schedule a reconnection attempt
   */
  scheduleReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      this.log(
        `Reconnection attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${this.reconnectInterval / 1000}s`,
        "info",
      );

      setTimeout(() => {
        this.initializeWebSocket();
      }, this.reconnectInterval);

      // Exponential backoff
      this.reconnectInterval = Math.min(this.reconnectInterval * 1.5, 30000);
    } else {
      this.log("Max reconnection attempts reached. Please refresh the page.", "error");
      this.showAlert("Connection lost. Please refresh the page to reconnect.", "error");
    }
  }

  /**
   * Update connection status indicator in the UI
   */
  updateConnectionStatus(connected) {
    const statusIndicator = document.getElementById("connectionStatus");
    if (statusIndicator) {
      statusIndicator.className = connected ? "connected" : "disconnected";
      statusIndicator.textContent = connected ? "Connected" : "Disconnected";
    }
  }

  /**
   * Bind UI events
   */
  bindEvents() {
    this.bindButton("scanBtn", () => {
      this.clearCompressionSummary();
      this.stopCompressionPollingAndStatus();
      this.scanDirectory();
    });
    this.bindButton("organizeBtn", () => {
      this.clearCompressionSummary();
      this.stopCompressionPollingAndStatus();
      this.organizePhotos();
    });
    this.bindButton("stopBtn", () => this.stopOperation());
    this.bindButton("saveConfigBtn", () => this.saveConfig());

    this.bindButton("startCompressionBtn", () => {
      this.clearCompressionSummary();
      this.stopCompressionPollingAndStatus();
      this.startCompression();
    });

    this.bindInput("sourceDir", (value) => this.validateSourceDirectory(value));
    this.bindInput("targetDir", (value) => this.validateTargetDirectory(value));

    this.bindSelect("dateFormat", () => this.updateConfigDisplay());
    this.bindSelect("duplicateHandling", () => this.updateConfigDisplay());
    this.bindCheckbox("moveFilesCheck", () => this.updateConfigDisplay());
    // this.bindCheckbox("dryRunCheck", () => this.updateConfigDisplay());

    this.bindCheckbox("compressionEnabled", () => this.updateConfigDisplay());
    this.bindKeyboardShortcuts();

    window.addEventListener("beforeunload", () => this.cleanup());
    window.addEventListener("focus", () => this.updateStatus());
  }

  /**
   * Bind a button with error handling
   */
  bindButton(id, handler) {
    const element = document.getElementById(id);
    if (element) {
      element.addEventListener("click", async (event) => {
        try {
          element.disabled = true;
          await handler(event);
        } catch (error) {
          this.log(`Error in ${id}: ${error.message}`, "error");
          this.showAlert(`Operation failed: ${error.message}`, "error");
        } finally {
          element.disabled = false;
        }
      });
    }
  }

  /**
   * Bind an input field with validation
   */
  bindInput(id, validator) {
    const element = document.getElementById(id);
    if (element) {
      element.addEventListener("input", (event) => {
        try {
          validator(event.target.value);
          if (typeof this.clearDropZoneState === "function") {
            this.clearDropZoneState(id);
          }
        } catch (error) {
          console.error("Input validation error:", error);
        }
      });
    }
  }

  /**
   * Bind a select field with handler
   */
  bindSelect(id, handler) {
    const element = document.getElementById(id);
    if (element) {
      element.addEventListener("change", (event) => {
        try {
          handler(event.target.value);
        } catch (error) {
          console.error("Select change error:", error);
        }
      });
    }
  }

  /**
   * Bind a checkbox field with handler
   */
  bindCheckbox(id, handler) {
    const element = document.getElementById(id);
    if (element) {
      element.addEventListener("change", (event) => {
        try {
          handler(event.target.checked);
        } catch (error) {
          console.error("Checkbox change error:", error);
        }
      });
    }
  }

  /**
   * Bind keyboard shortcuts for actions
   */
  bindKeyboardShortcuts() {
    document.addEventListener("keydown", (event) => {
      if (event.ctrlKey || event.metaKey) {
        switch (event.key) {
          case "s":
            event.preventDefault();
            this.scanDirectory();
            break;
          case "o":
            event.preventDefault();
            this.organizePhotos();
            break;
          case "Escape":
            event.preventDefault();
            this.stopOperation();
            break;
        }
      }
    });
  }

  /**
   * Start periodic status polling
   */
  startStatusPolling() {
    setInterval(() => {
      if (!this.isConnected) {
        this.updateStatus();
      }
      this.updateCompressionStatus();
    }, 5000);
  }

  /**
   * Update status from server
   */
  async updateStatus() {
    try {
      const response = await this.fetchWithTimeout("/api/status", {}, 5000);
      const data = await response.json();

      if (data.success) {
        this.updateUI(data.data);
      } else {
        throw new Error(data.error || "Failed to get status");
      }
    } catch (error) {
      if (!this.isConnected) {
        this.log("Status update failed (offline mode)", "warning");
      }
    }
  }

  /**
   * Update UI with status data
   */
  updateUI(data) {
    const { running, statistics } = data;

    this.updateElement("operationStatus", running ? "Running..." : "Ready");
    this.updateElement("scanBtn", null, { disabled: running });
    this.updateElement("organizeBtn", null, { disabled: running });
    this.toggleElement("stopBtn", running);

    if (statistics && statistics.files) {
      const { files } = statistics;
      this.updateElement("filesFound", files.total_found || 0);
      this.updateElement("filesProcessed", files.total_processed || 0);
      this.updateElement("filesOrganized", files.organized || 0);
      this.updateElement("filesMoved", files.moved || 0);
      this.updateElement("filesSkipped", files.skipped || 0);
      this.updateElement("errorsCount", files.errors || 0);
      this.updateElement("filesCopied", files.copied || 0);

      const progress =
        files.total_found > 0 ? (files.total_processed / files.total_found) * 100 : 0;
      this.updateProgressBar(progress);
    }
  }

  /**
   * Start scan directory operation
   */
  async scanDirectory() {
    const sourceDir = this.getInputValue("sourceDir");
    if (!sourceDir) {
      this.showAlert("Please enter a source directory", "error");
      return;
    }

    if (!this.validatePath(sourceDir)) {
      this.showAlert("Please enter a valid directory path", "error");
      return;
    }

    if (!sourceDir.startsWith("/") && !sourceDir.match(/^[A-Za-z]:/)) {
      this.showAlert(
        `Warning: "${sourceDir}" appears to be a relative path. Please enter the full absolute path to your folder (e.g., /home/user/Photos or C:\\Users\\User\\Photos)`,
        "warning",
      );
    }

    try {
      const response = await this.fetchWithTimeout("/api/scan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ directory: sourceDir }),
      });
      const data = await response.json();
      if (data.success) {
        // Scan started successfully
      } else {
        throw new Error(data.error || "Scan failed");
      }
    } catch (error) {
      let errorMessage = error.message;

      if (error.message.includes("400") || error.message.includes("Bad Request")) {
        errorMessage = `Directory not found: "${sourceDir}". Please check the path and ensure you're using the full absolute path to your folder.`;
      }

      this.showAlert(`Failed to start scan: ${errorMessage}`, "error");
      this.log(`Scan error: ${errorMessage}`, "error");
    }
  }

  /**
   * Start organize photos operation
   */
  async organizePhotos() {
    const sourceDir = this.getInputValue("sourceDir");
    const targetDir = this.getInputValue("targetDir");

    if (!sourceDir) {
      this.showAlert("Please enter a source directory", "error");
      return;
    }

    if (!this.validatePath(sourceDir)) {
      this.showAlert("Please enter a valid directory path", "error");
      return;
    }

    // Always ask for confirmation before organizing (since dry-run checkbox is removed)
    const confirmed = confirm(
      `Are you sure you want to organize photos?\nSource: ${sourceDir}\nTarget: ${targetDir || "In place"}\n\nThis will move/modify your files!`,
    );
    if (!confirmed) {
      return;
    }

    try {
      const dateFormat = this.getSelectValue("dateFormat");
      const moveFiles = this.getCheckboxValue("moveFilesCheck");

      const response = await this.fetchWithTimeout("/api/organize", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          source_directory: sourceDir,
          target_directory: targetDir || null,
          dry_run: false,
          date_format: dateFormat,
          move_files: moveFiles,
        }),
      });

      const data = await response.json();
      if (data.success) {
        // Organization started successfully
      } else {
        throw new Error(data.error || "Organization failed");
      }
    } catch (error) {
      let errorMessage = error.message;
      this.showAlert(`Failed to start organization: ${errorMessage}`, "error");
      this.log(`Organization error: ${errorMessage}`, "error");
    }
  }

  /**
   * Stop current operation
   */
  async stopOperation() {
    try {
      const response = await this.fetchWithTimeout("/api/stop", { method: "POST" });
      const data = await response.json();

      if (data.success) {
        this.showAlert("Operation stopped", "info");
      } else {
        throw new Error(data.error || "Failed to stop operation");
      }
    } catch (error) {
      this.showAlert(`Failed to stop operation: ${error.message}`, "error");
      this.log(`Stop error: ${error.message}`, "error");
    }
  }

  /**
   * Start image compression operation
   */
  async startCompression() {
    const enabled = document.getElementById("compressionEnabled").checked;
    if (!enabled) {
      this.showAlert("Enable the compression checkbox to start.", "warning");
      this.updateElement("compressionStatus", "");
      this.updateElement("compressionSummary", "");
      return;
    }
    const quality = parseInt(document.getElementById("compressionQuality").value, 10) || 85;
    const threshold = parseFloat(document.getElementById("compressionThreshold").value) || 1.01;
    const formats = document
      .getElementById("compressionFormats")
      .value.split(",")
      .map((f) => f.trim())
      .filter(Boolean);

    this.updateElement("compressionStatus", "Starting compression...");
    this.updateElement("compressionResults", "");

    try {
      const response = await fetch("/api/compress", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          quality,
          threshold,
          formats,
        }),
      });
      const data = await response.json();
      if (data.success) {
        this.updateElement("compressionStatus", "Compression started...");
        this.pollCompressionStatus();
      } else {
        this.updateElement(
          "compressionStatus",
          "Failed to start compression: " + (data.error || data.message),
        );
      }
    } catch (error) {
      this.updateElement("compressionStatus", "Error: " + error.message);
    }
  }

  /**
   * Poll compression status periodically
   */
  pollCompressionStatus() {
    if (this._compressionPollInterval) clearInterval(this._compressionPollInterval);
    this._compressionPollInterval = setInterval(() => this.updateCompressionStatus(), 2000);
    this.updateCompressionStatus();
  }

  /**
   * Update compression status/result from server
   */
  async updateCompressionStatus() {
    try {
      const response = await fetch("/api/compression-status");
      const data = await response.json();
      if (!data.success) {
        this.updateElement(
          "compressionStatus",
          "Failed to get compression status: " + (data.error || data.message),
        );
        return;
      }
      const { running, results, error } = data.data || {};
      if (running) {
        this.updateElement("compressionStatus", "Compression in progress...");
      } else if (error) {
        this.updateElement("compressionStatus", "Compression error: " + error);
        if (this._compressionPollInterval) clearInterval(this._compressionPollInterval);
      } else if (results && results.length > 0) {
        this.updateElement("compressionStatus", "Compression finished.");
        let compressed = results.filter(
          (r) =>
            r.action === "compressed" ||
            r.Action === "compressed" ||
            r.action === "original" ||
            r.Action === "original",
        );
        if (compressed.length === 0) {
          this.updateElement("compressionSummary", "All files were skipped (already compressed).");
          this.autoClearCompressionSummary();
        } else {
          let origSize = 0,
            compSize = 0;
          for (const r of compressed) {
            origSize += r.originalSize || r.OriginalSize || 0;
            compSize += r.compressedSize || r.CompressedSize || 0;
          }
          let percent = origSize > 0 ? ((origSize - compSize) * 100) / origSize : 0;
          const summary = [
            `Original Size: ${this.formatSize(origSize)}`,
            `Compressed Size: ${this.formatSize(compSize)}`,
            `Saved (%): ${percent.toFixed(1)}`,
          ].join("\n");
          this.updateElement("compressionSummary", summary);
          this.autoClearCompressionSummary();
        }
        if (this._compressionPollInterval) clearInterval(this._compressionPollInterval);
      } else {
        this.updateElement("compressionStatus", "");
        this.updateElement("compressionSummary", "");
        if (this._compressionPollInterval) clearInterval(this._compressionPollInterval);
      }
    } catch (error) {
      this.updateElement("compressionStatus", "Error: " + error.message);
    }
  }

  /**
   * Render compression results (legacy, not used)
   */
  renderCompressionResults(results) {
    this.updateElement("compressionResults", "");
    if (!Array.isArray(results) || results.length === 0) {
      this.updateElement("compressionSummary", "");
      return;
    }
    let origSize = 0,
      compSize = 0;
    for (const r of results) {
      origSize += r.originalSize || r.OriginalSize || 0;
      compSize += r.compressedSize || r.CompressedSize || 0;
    }
    let percent = origSize > 0 ? ((origSize - compSize) * 100) / origSize : 0;
    const summary = [
      `Original Size: ${this.formatBytes(origSize)}`,
      `Compressed Size: ${this.formatBytes(compSize)}`,
      `Saved (%): ${percent.toFixed(1)}`,
    ].join("<br>");
    this.updateElement("compressionSummary", summary);
  }

  /**
   * Format file size in bytes
   */
  formatSize(size) {
    if (!size || isNaN(size)) return "";
    if (size < 1024) return size + " B";
    if (size < 1024 * 1024) return (size / 1024).toFixed(1) + " KB";
    return (size / (1024 * 1024)).toFixed(2) + " MB";
  }

  /**
   * Escape HTML special characters
   */
  escapeHtml(str) {
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  /**
   * Handle WebSocket messages
   */
  handleWebSocketMessage(message) {
    const { type, data } = message;

    console.log("WebSocket message received:", { type, data, timestamp: new Date().toISOString() });

    switch (type) {
      case "log":
        // Логи DRY-RUN/Would move/copy и др. из backend
        this.log(
          data && data.message
            ? `[${data.timestamp || new Date().toLocaleTimeString()}] ${data.message}`
            : "Log message",
          data && data.level ? data.level : "info",
        );
        break;
      case "scan_started":
        console.log("Processing scan_started message:", data);
        this.log(`Scan started for: ${data.directory}`, "info");
        break;
      case "scan_completed":
        let filesFound = null;
        if (data && data.statistics) {
          const match =
            typeof data.statistics === "string"
              ? data.statistics.match(/Total Found:\s*(\d+)/)
              : null;
          if (match) {
            filesFound = parseInt(match[1], 10);
          }
        }
        if (filesFound !== null) {
          this.log(`Scan completed successfully. Files found: ${filesFound}`, "success");
        } else {
          this.log("Scan completed successfully", "success");
        }
        this.showAlert("Scan completed!", "success");
        break;
      case "scan_error":
        this.log(`Scan error: ${data.error}`, "error");
        this.showAlert(`Scan failed: ${data.error}`, "error");
        break;
      case "organize_started":
        {
          const targetInfo = data.target_directory ? ` → ${data.target_directory}` : " (in place)";
          this.log(
            `Organization started (${data.dry_run ? "DRY RUN" : "LIVE"}) for: ${data.source_directory}${targetInfo}`,
            "info",
          );
        }
        break;
      case "organize_completed":
        this.log("Organization completed successfully", "success");
        this.showAlert("Organization completed!", "success");
        break;
      case "organize_error":
        this.log(`Organization error: ${data.error}`, "error");
        this.showAlert(`Organization failed: ${data.error}`, "error");
        break;
      case "compression_started":
        this.log("Compression started", "info");
        this.showAlert("Compression started...", "info");
        break;
      case "compression_completed":
        {
          let origSize = 0,
            compSize = 0,
            percent = 0;
          if (Array.isArray(data.results) && data.results.length > 0) {
            let compressed = data.results.filter(
              (r) =>
                r.action === "compressed" ||
                r.Action === "compressed" ||
                r.action === "original" ||
                r.Action === "original",
            );
            for (const r of compressed) {
              origSize += r.originalSize || r.OriginalSize || 0;
              compSize += r.compressedSize || r.CompressedSize || 0;
            }
            percent = origSize > 0 ? ((origSize - compSize) * 100) / origSize : 0;
          } else if (
            typeof data.original_size !== "undefined" &&
            typeof data.compressed_size !== "undefined"
          ) {
            origSize = data.original_size;
            compSize = data.compressed_size;
            percent = typeof data.percent_saved === "number" ? data.percent_saved : 0;
          }
          let msg = "Compression finished";
          if (typeof data.files_processed !== "undefined") {
            msg += `: ${data.files_processed} files`;
          }
          msg += ` | Original Size: ${this.formatSize(origSize)}, Compressed Size: ${this.formatSize(compSize)}, Saved: ${percent.toFixed(1)}%`;
          this.log(msg, "success");
          if (
            typeof data.files_processed === "number" &&
            data.files_processed > 0 &&
            (!origSize || !compSize || compSize === 0)
          ) {
            this.updateElement(
              "compressionSummary",
              "All files were skipped (already compressed).",
            );
          } else {
            const summary = [
              `Original Size: ${this.formatSize(origSize)}`,
              `Compressed Size: ${this.formatSize(compSize)}`,
              `Saved (%): ${percent.toFixed(1)}`,
            ].join("\n");
            this.updateElement("compressionSummary", summary);
            this.autoClearCompressionSummary();
          }
        }
        break;
      case "compression_error":
        this.log(`Compression error: ${data.error || ""}`, "error");
        this.showAlert(`Compression failed: ${data.error || ""}`, "error");
        break;
      case "operation_stopped":
        this.log("Operation stopped by user", "info");
        break;
      case "progress_update":
        if (data.statistics) {
          this.updateUI({ running: true, statistics: data.statistics });
        }
        break;
      default:
        this.log(`Unknown message type: ${type}`, "warning");
    }

    this.updateStatus();
  }

  /**
   * Log message to console and UI
   */
  log(message, type = "info") {
    const container = document.getElementById("logContainer");
    if (!container) return;

    const entry = document.createElement("div");
    entry.className = `log-entry log-${type}`;

    // Форматировать время в 24-часовом формате HH:mm:ss
    const now = new Date();
    const timestamp = now.toLocaleTimeString("ru-RU", { hour12: false }).padStart(8, "0");

    // Убрать дублирующийся timestamp из сообщения (например, [2025-07-13 17:21:31])
    let cleanMessage = message.replace(/^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]\s*/, "");

    entry.innerHTML = `
      <span class="log-timestamp">[${timestamp}]</span>
      <span class="log-type">[${type.toUpperCase()}]</span>
      <span class="log-message">${this.escapeHtml(cleanMessage)}</span>
    `;

    container.appendChild(entry);
    container.scrollTop = container.scrollHeight;

    while (container.children.length > 100) {
      container.removeChild(container.firstChild);
    }

    if (type === "error") {
      console.error(`[PhotoSorter] ${message}`);
    } else {
      console.log(`[PhotoSorter] ${message}`);
    }
  }

  /**
   * Show alert message for errors and warnings
   */
  showAlert(message, type) {
    if (type === "error" || type === "warning") {
      const alertsContainer = document.getElementById("alerts");
      if (!alertsContainer) return;

      const alert = document.createElement("div");
      alert.className = `alert alert-${type}`;
      alert.innerHTML = `
        <span>${this.escapeHtml(message)}</span>
        <button class="btn-close" onclick="this.parentElement.remove()"></button>
      `;

      alertsContainer.appendChild(alert);

      setTimeout(() => {
        alert.remove();
      }, 5000);
    }
  }

  /**
   * Fetch with timeout
   */
  async fetchWithTimeout(url, options = {}, timeout = 10000) {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    try {
      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      return response;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Validate file path
   */
  validatePath(path) {
    if (!path || typeof path !== "string") return false;
    if (path.includes("..")) return false;
    const trimmed = path.trim();
    if (trimmed.length === 0) return false;
    const isValidFormat = /^([a-zA-Z]:[\\\/]|[\\\/]|[a-zA-Z0-9])[^<>:"|?*]*$/.test(trimmed);
    return isValidFormat;
  }

  /**
   * Validate source directory
   */
  validateSourceDirectory(value) {
    const isValid = this.validatePath(value);
    this.updateInputValidation("sourceDir", isValid);
    return isValid;
  }

  /**
   * Validate target directory
   */
  validateTargetDirectory(value) {
    const isValid = !value || this.validatePath(value);
    this.updateInputValidation("targetDir", isValid);
    return isValid;
  }

  /**
   * Update input validation state
   */
  updateInputValidation(id, isValid) {
    const element = document.getElementById(id);
    if (element) {
      element.classList.remove("is-valid", "is-invalid");
      if (isValid) {
        element.classList.add("is-valid");
      } else {
        element.classList.add("is-invalid");
      }
    }
  }

  /**
   * Update element content and attributes
   */
  updateElement(id, content, attributes = {}) {
    const element = document.getElementById(id);
    if (!element) return;

    if (content !== null && content !== undefined) {
      element.textContent = content;
    }

    Object.entries(attributes).forEach(([key, value]) => {
      if (key === "disabled") {
        element.disabled = value;
      } else {
        element.setAttribute(key, value);
      }
    });
  }

  /**
   * Toggle element visibility
   */
  toggleElement(id, show) {
    const element = document.getElementById(id);
    if (element) {
      element.classList.toggle("d-none", !show);
    }
  }

  /**
   * Hide element
   */
  hideElement(id) {
    const element = document.getElementById(id);
    if (element) {
      element.classList.add("d-none");
    }
  }

  /**
   * Get input value
   */
  getInputValue(id) {
    const element = document.getElementById(id);
    return element ? element.value.trim() : "";
  }

  /**
   * Set input value
   */
  setInputValue(id, value) {
    const element = document.getElementById(id);
    if (element) {
      element.value = value;
    }
  }

  /**
   * Clear compression summary
   */
  clearCompressionSummary() {
    this.updateElement("compressionSummary", "");
  }

  /**
   * Stop polling compression status and reset status
   */
  stopCompressionPollingAndStatus() {
    if (this._compressionPollInterval) {
      clearInterval(this._compressionPollInterval);
      this._compressionPollInterval = null;
    }
    this.updateElement("compressionStatus", "");
  }

  /**
   * Automatically clear compression summary after 10 seconds
   */
  autoClearCompressionSummary() {
    clearTimeout(this._compressionSummaryTimeout);
    this._compressionSummaryTimeout = setTimeout(() => {
      this.clearCompressionSummary();
    }, 10000);
  }

  /**
   * Get checkbox value
   */
  getCheckboxValue(id) {
    const element = document.getElementById(id);
    return element ? element.checked : false;
  }

  /**
   * Get select value
   */
  getSelectValue(id) {
    const element = document.getElementById(id);
    return element ? element.value : "";
  }

  /**
   * Set select value
   */
  setSelectValue(id, value) {
    const element = document.getElementById(id);
    if (element) {
      element.value = value;
    }
  }

  /**
   * Set checkbox value
   */
  setCheckboxValue(id, checked) {
    const element = document.getElementById(id);
    if (element) {
      element.checked = checked;
    }
  }

  /**
   * Load configuration from server
   */
  async loadConfig() {
    try {
      const response = await this.fetchWithTimeout("/api/config");
      const data = await response.json();

      if (data.success) {
        const config = data.data;

        this.setSelectValue("dateFormat", config.date_format || "2006/01/02");
        this.setCheckboxValue("moveFilesCheck", config.move_files !== false);
        this.setSelectValue("duplicateHandling", config.duplicate_handling || "rename");
        this.setCheckboxValue("dryRunCheck", config.dry_run !== false);

        if (config.source_directory) {
          this.setInputValue("sourceDir", config.source_directory);
        }
        if (config.target_directory) {
          this.setInputValue("targetDir", config.target_directory);
        }

        this.updateConfigDisplay();
        this.log("Configuration loaded successfully", "info");
      } else {
        throw new Error(data.error || "Failed to load config");
      }
    } catch (error) {
      this.log(`Failed to load configuration: ${error.message}`, "error");
    }
  }

  /**
   * Save configuration to server
   */
  async saveConfig() {
    try {
      const config = {
        date_format: this.getSelectValue("dateFormat"),
        move_files: this.getCheckboxValue("moveFilesCheck"),
        duplicate_handling: this.getSelectValue("duplicateHandling"),
        dry_run: this.getCheckboxValue("dryRunCheck"),
        source_directory: this.getInputValue("sourceDir"),
        target_directory: this.getInputValue("targetDir") || null,
      };

      const response = await this.fetchWithTimeout("/api/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });

      const data = await response.json();
      if (data.success) {
        this.log("Configuration saved", "info");
        this.updateConfigDisplay();
      } else {
        throw new Error(data.error || "Failed to save config");
      }
    } catch (error) {
      this.showAlert(`Failed to save configuration: ${error.message}`, "error");
      this.log(`Save config error: ${error.message}`, "error");
    }
  }

  /**
   * Update configuration display in the UI
   */
  updateConfigDisplay() {
    const dateFormat = this.getSelectValue("dateFormat");
    const moveFiles = this.getCheckboxValue("moveFilesCheck");
    const duplicateHandling = this.getSelectValue("duplicateHandling");
    const dryRun = this.getCheckboxValue("dryRunCheck");
    const compressionEnabled = document.getElementById("compressionEnabled")
      ? document.getElementById("compressionEnabled").checked
      : false;

    const formatName =
      {
        "2006/01/02": "Year/Month/Day",
        "2006/01": "Year/Month",
        2006: "Year Only",
        "2006-01-02": "Year-Month-Day",
        "2006-01": "Year-Month",
      }[dateFormat] || dateFormat;

    const configText = `
      Format: ${formatName} |
      Action: ${moveFiles ? "Move" : "Copy"} |
      Duplicates: ${duplicateHandling} |
      Mode: ${dryRun ? "Dry Run" : "Live"} |
      Compression: ${compressionEnabled ? "On" : "Off"}
    `;

    this.updateElement("currentConfig", configText.trim());
  }

  /**
   * Update progress bar in the UI
   */
  updateProgressBar(percentage) {
    const progressFill = document.getElementById("progressFill");
    if (progressFill) {
      progressFill.style.width = `${Math.min(100, Math.max(0, percentage))}%`;
    }
  }

  /**
   * Escape HTML to prevent XSS
   */
  escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  /**
   * Cleanup resources on page unload
   */
  cleanup() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.close();
    }
    if (this._compressionPollInterval) {
      clearInterval(this._compressionPollInterval);
      this._compressionPollInterval = null;
    }
  }
}

document.addEventListener("DOMContentLoaded", () => {
  window.photoSorterApp = new PhotoSorterApp();
});

if (typeof module !== "undefined" && module.exports) {
  module.exports = PhotoSorterApp;
}
