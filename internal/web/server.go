package web

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/maisi/unraid-filehasher/internal/db"
	"github.com/maisi/unraid-filehasher/internal/format"
)

// appVersion is set by Serve() and injected into every template render.
var appVersion string

// Serve starts the web dashboard on the given address.
func Serve(database *db.DB, addr string, version string, runner *Runner) error {
	appVersion = version

	mux := http.NewServeMux()

	mux.HandleFunc("/", handleOverview(database))
	mux.HandleFunc("/disks", handleDisks(database))
	mux.HandleFunc("/corrupted", handleCorrupted(database))
	mux.HandleFunc("/missing", handleMissing(database))
	mux.HandleFunc("/ok", handleOK(database))
	mux.HandleFunc("/new", handleNew(database))
	mux.HandleFunc("/files", handleFiles(database))
	mux.HandleFunc("/search", handleSearch(database))
	mux.HandleFunc("/history", handleHistory(database))
	mux.HandleFunc("/settings", handleSettings())

	// API endpoints (JSON)
	mux.HandleFunc("/api/stats", handleAPIStats(database))
	mux.HandleFunc("/api/disks", handleAPIDisks(database))

	// Runner endpoints
	if runner != nil {
		mux.HandleFunc("/api/scan", handleAPIScan(runner))
		mux.HandleFunc("/api/verify", handleAPIVerify(runner))
		mux.HandleFunc("/api/stop", handleAPIStop(runner))
		mux.HandleFunc("/api/progress", handleAPIProgress(runner))
	}

	// Config endpoint (read-only, for JS options panel)
	mux.HandleFunc("/api/config", handleAPIConfig())

	return http.ListenAndServe(addr, mux)
}

func handleOverview(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		stats, err := database.GetStats()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		diskStats, err := database.GetDiskStats()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		data := map[string]interface{}{
			"Stats":     stats,
			"DiskStats": diskStats,
			"Page":      "overview",
		}
		renderTemplate(w, "overview", data)
	}
}

func handleDisks(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		disk := r.URL.Query().Get("name")
		if disk == "" {
			diskStats, err := database.GetDiskStats()
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			data := map[string]interface{}{
				"DiskStats": diskStats,
				"Page":      "disks",
			}
			renderTemplate(w, "disks", data)
			return
		}

		files, err := database.GetFilesByDisk(disk)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"Disk":  disk,
			"Files": files,
			"Count": len(files),
			"Page":  "disks",
		}
		renderTemplate(w, "disk_detail", data)
	}
}

func handleCorrupted(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFilesByStatus("corrupted")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"Files":      files,
			"Count":      len(files),
			"Page":       "corrupted",
			"StatusName": "Corrupted",
		}
		renderTemplate(w, "status_list", data)
	}
}

func handleMissing(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFilesByStatus("missing")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"Files":      files,
			"Count":      len(files),
			"Page":       "missing",
			"StatusName": "Missing",
		}
		renderTemplate(w, "status_list", data)
	}
}

func handleOK(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFilesByStatus("ok")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"Files":      files,
			"Count":      len(files),
			"Page":       "ok",
			"StatusName": "OK",
		}
		renderTemplate(w, "status_list", data)
	}
}

func handleNew(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFilesByStatus("new")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"Files":      files,
			"Count":      len(files),
			"Page":       "new",
			"StatusName": "New",
		}
		renderTemplate(w, "status_list", data)
	}
}

func handleFiles(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		perPage := 100
		if v := r.URL.Query().Get("page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				page = n
			}
		}
		if v := r.URL.Query().Get("per_page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
				perPage = n
			}
		}

		offset := (page - 1) * perPage
		files, total, err := database.GetAllFilesPaginated(perPage, offset)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		totalPages := int(total) / perPage
		if int(total)%perPage != 0 {
			totalPages++
		}

		data := map[string]interface{}{
			"Files":       files,
			"Count":       len(files),
			"Total":       total,
			"Page":        "files",
			"CurrentPage": page,
			"PerPage":     perPage,
			"TotalPages":  totalPages,
			"HasPrev":     page > 1,
			"HasNext":     page < totalPages,
			"PrevPage":    page - 1,
			"NextPage":    page + 1,
		}
		renderTemplate(w, "files", data)
	}
}

func handleSearch(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		var files []*db.FileRecord
		var err error
		if query != "" {
			files, err = database.SearchFiles(query, 200)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		data := map[string]interface{}{
			"Query": query,
			"Files": files,
			"Count": len(files),
			"Page":  "search",
		}
		renderTemplate(w, "search", data)
	}
}

func handleHistory(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		history, err := database.GetScanHistory(50)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data := map[string]interface{}{
			"History": history,
			"Page":    "history",
		}
		renderTemplate(w, "history", data)
	}
}

const cfgPath = "/boot/config/filehasher/cron.cfg"

// appConfig holds all persistent settings (cron, scan, verify, thermal).
type appConfig struct {
	// Scheduled verification
	Enabled  bool
	Schedule string
	Mode     string // "full" or "quick"

	// Scan defaults
	ScanExcludes       string // newline-separated regex patterns
	ScanExcludeAppdata bool
	ScanFull           bool
	ScanHddTwoPhase    bool
	ScanDiskType       string // "auto", "hdd", "ssd"

	// Verify defaults
	VerifyWorkers int
	VerifyQuick   bool

	// Thermal protection
	ThermalEnabled   bool
	ThermalPollSecs  int
	ThermalHddPause  int
	ThermalHddResume int
	ThermalSsdPause  int
	ThermalSsdResume int

	// Do Not Disturb schedule
	DndEnabled bool
	DndStart   string // "HH:MM" format, e.g. "18:00"
	DndEnd     string // "HH:MM" format, e.g. "23:00"
}

func defaultAppConfig() appConfig {
	return appConfig{
		Schedule:         "0 3 * * 0",
		Mode:             "full",
		ScanHddTwoPhase:  true,
		ScanDiskType:     "auto",
		VerifyWorkers:    4,
		ThermalEnabled:   true,
		ThermalPollSecs:  60,
		ThermalHddPause:  55,
		ThermalHddResume: 45,
		ThermalSsdPause:  70,
		ThermalSsdResume: 60,
		DndStart:         "18:00",
		DndEnd:           "06:00",
	}
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func readAppConfig() appConfig {
	cfg := defaultAppConfig()

	f, err := os.Open(cfgPath)
	if err != nil {
		return cfg
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		// Cron / scheduled verification
		case "ENABLED":
			cfg.Enabled = val == "yes"
		case "SCHEDULE":
			cfg.Schedule = val
		case "MODE":
			cfg.Mode = val

		// Scan defaults
		case "SCAN_EXCLUDES":
			// Stored with literal \n as line separator
			cfg.ScanExcludes = strings.ReplaceAll(val, `\n`, "\n")
		case "SCAN_EXCLUDE_APPDATA":
			cfg.ScanExcludeAppdata = val == "yes"
		case "SCAN_FULL":
			cfg.ScanFull = val == "yes"
		case "SCAN_HDD_TWO_PHASE":
			cfg.ScanHddTwoPhase = val == "yes"
		case "SCAN_DISK_TYPE":
			cfg.ScanDiskType = val

		// Verify defaults
		case "VERIFY_WORKERS":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.VerifyWorkers = n
			}
		case "VERIFY_QUICK":
			cfg.VerifyQuick = val == "yes"

		// Thermal protection
		case "THERMAL_ENABLED":
			cfg.ThermalEnabled = val == "yes"
		case "THERMAL_POLL_SECS":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.ThermalPollSecs = n
			}
		case "THERMAL_HDD_PAUSE":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.ThermalHddPause = n
			}
		case "THERMAL_HDD_RESUME":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.ThermalHddResume = n
			}
		case "THERMAL_SSD_PAUSE":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.ThermalSsdPause = n
			}
		case "THERMAL_SSD_RESUME":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.ThermalSsdResume = n
			}

		// Do Not Disturb
		case "DND_ENABLED":
			cfg.DndEnabled = val == "yes"
		case "DND_START":
			cfg.DndStart = val
		case "DND_END":
			cfg.DndEnd = val
		}
	}
	return cfg
}

func writeAppConfig(cfg appConfig) error {
	// Encode newlines in excludes as literal \n for single-line storage
	excludesEncoded := strings.ReplaceAll(cfg.ScanExcludes, "\n", `\n`)

	content := fmt.Sprintf(`# filehasher settings
# Set ENABLED=yes to enable automatic verification
ENABLED=%s
# Cron schedule (default: Sunday 3 AM)
SCHEDULE=%s
# Verify mode for scheduled runs: "full" or "quick"
MODE=%s

# Scan defaults
SCAN_EXCLUDES=%s
SCAN_EXCLUDE_APPDATA=%s
SCAN_FULL=%s
SCAN_HDD_TWO_PHASE=%s
SCAN_DISK_TYPE=%s

# Verify defaults
VERIFY_WORKERS=%d
VERIFY_QUICK=%s

# Thermal protection
THERMAL_ENABLED=%s
THERMAL_POLL_SECS=%d
THERMAL_HDD_PAUSE=%d
THERMAL_HDD_RESUME=%d
THERMAL_SSD_PAUSE=%d
THERMAL_SSD_RESUME=%d

# Do Not Disturb schedule
DND_ENABLED=%s
DND_START=%s
DND_END=%s
`,
		boolToYesNo(cfg.Enabled), cfg.Schedule, cfg.Mode,
		excludesEncoded, boolToYesNo(cfg.ScanExcludeAppdata),
		boolToYesNo(cfg.ScanFull), boolToYesNo(cfg.ScanHddTwoPhase), cfg.ScanDiskType,
		cfg.VerifyWorkers, boolToYesNo(cfg.VerifyQuick),
		boolToYesNo(cfg.ThermalEnabled), cfg.ThermalPollSecs,
		cfg.ThermalHddPause, cfg.ThermalHddResume,
		cfg.ThermalSsdPause, cfg.ThermalSsdResume,
		boolToYesNo(cfg.DndEnabled), cfg.DndStart, cfg.DndEnd,
	)

	if err := os.MkdirAll("/boot/config/filehasher", 0755); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return err
	}

	// Re-run the cron installer script if it exists
	script := "/boot/config/plugins/filehasher/filehasher-cron"
	if _, err := os.Stat(script); err == nil {
		exec.Command("/bin/bash", script).Run()
	}
	return nil
}

// toScanOptions converts saved config to ScanOptions.
func (c appConfig) toScanOptions() ScanOptions {
	var excludes []string
	for _, line := range strings.Split(c.ScanExcludes, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			excludes = append(excludes, line)
		}
	}
	return ScanOptions{
		Excludes:       excludes,
		ExcludeAppdata: c.ScanExcludeAppdata,
		FullScan:       c.ScanFull,
		HddTwoPhase:    c.ScanHddTwoPhase,
		DiskType:       c.ScanDiskType,
	}
}

// toVerifyOptions converts saved config to VerifyOptions.
func (c appConfig) toVerifyOptions() VerifyOptions {
	return VerifyOptions{
		Workers: c.VerifyWorkers,
		Quick:   c.VerifyQuick,
	}
}

// toThermalConfig converts saved config to ThermalConfig.
func (c appConfig) toThermalConfig() ThermalConfig {
	return ThermalConfig{
		Enabled:   c.ThermalEnabled,
		PollSecs:  c.ThermalPollSecs,
		HddPause:  c.ThermalHddPause,
		HddResume: c.ThermalHddResume,
		SsdPause:  c.ThermalSsdPause,
		SsdResume: c.ThermalSsdResume,
	}
}

// toDndConfig converts saved config to DndConfig.
func (c appConfig) toDndConfig() DndConfig {
	return DndConfig{
		Enabled: c.DndEnabled,
		Start:   c.DndStart,
		End:     c.DndEnd,
	}
}

func handleSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()

			cfg := defaultAppConfig()

			// Scheduled verification
			cfg.Enabled = r.FormValue("enabled") == "yes"
			cfg.Schedule = r.FormValue("schedule")
			cfg.Mode = r.FormValue("mode")
			if cfg.Schedule == "" {
				cfg.Schedule = "0 3 * * 0"
			}
			if cfg.Mode == "" {
				cfg.Mode = "full"
			}

			// Scan defaults
			cfg.ScanExcludes = r.FormValue("scan_excludes")
			cfg.ScanExcludeAppdata = r.FormValue("scan_exclude_appdata") == "yes"
			cfg.ScanFull = r.FormValue("scan_full") == "yes"
			cfg.ScanHddTwoPhase = r.FormValue("scan_hdd_two_phase") == "yes"
			cfg.ScanDiskType = r.FormValue("scan_disk_type")
			if cfg.ScanDiskType == "" {
				cfg.ScanDiskType = "auto"
			}

			// Verify defaults
			if v := r.FormValue("verify_workers"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.VerifyWorkers = n
				}
			}
			cfg.VerifyQuick = r.FormValue("verify_quick") == "yes"

			// Thermal protection
			cfg.ThermalEnabled = r.FormValue("thermal_enabled") == "yes"
			if v := r.FormValue("thermal_poll_secs"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ThermalPollSecs = n
				}
			}
			if v := r.FormValue("thermal_hdd_pause"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ThermalHddPause = n
				}
			}
			if v := r.FormValue("thermal_hdd_resume"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ThermalHddResume = n
				}
			}
			if v := r.FormValue("thermal_ssd_pause"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ThermalSsdPause = n
				}
			}
			if v := r.FormValue("thermal_ssd_resume"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ThermalSsdResume = n
				}
			}

			// Do Not Disturb
			cfg.DndEnabled = r.FormValue("dnd_enabled") == "yes"
			cfg.DndStart = r.FormValue("dnd_start")
			cfg.DndEnd = r.FormValue("dnd_end")
			if cfg.DndStart == "" {
				cfg.DndStart = "18:00"
			}
			if cfg.DndEnd == "" {
				cfg.DndEnd = "06:00"
			}

			err := writeAppConfig(cfg)
			msg := "Settings saved successfully."
			if err != nil {
				msg = fmt.Sprintf("Error saving settings: %v", err)
			}
			data := map[string]interface{}{
				"Page":    "settings",
				"Config":  cfg,
				"Message": msg,
			}
			renderTemplate(w, "settings", data)
			return
		}

		cfg := readAppConfig()
		data := map[string]interface{}{
			"Page":   "settings",
			"Config": cfg,
		}
		renderTemplate(w, "settings", data)
	}
}

func handleAPIStats(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := database.GetStats()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		json.NewEncoder(w).Encode(stats)
	}
}

func handleAPIDisks(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diskStats, err := database.GetDiskStats()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		json.NewEncoder(w).Encode(diskStats)
	}
}

func handleAPIScan(runner *Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Load saved defaults
		cfg := readAppConfig()
		opts := cfg.toScanOptions()
		thermal := cfg.toThermalConfig()

		// Parse optional JSON overrides from request body
		if r.Body != nil && r.ContentLength > 0 {
			var body struct {
				Excludes       *[]string `json:"excludes"`
				ExcludeAppdata *bool     `json:"excludeAppdata"`
				FullScan       *bool     `json:"fullScan"`
				HddTwoPhase    *bool     `json:"hddTwoPhase"`
				DiskType       *string   `json:"diskType"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if body.Excludes != nil {
					opts.Excludes = *body.Excludes
				}
				if body.ExcludeAppdata != nil {
					opts.ExcludeAppdata = *body.ExcludeAppdata
				}
				if body.FullScan != nil {
					opts.FullScan = *body.FullScan
				}
				if body.HddTwoPhase != nil {
					opts.HddTwoPhase = *body.HddTwoPhase
				}
				if body.DiskType != nil {
					opts.DiskType = *body.DiskType
				}
			}
		}

		if err := runner.StartScan(opts, thermal, cfg.toDndConfig()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "started"})
	}
}

func handleAPIVerify(runner *Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Load saved defaults
		cfg := readAppConfig()
		opts := cfg.toVerifyOptions()
		thermal := cfg.toThermalConfig()

		// Parse optional JSON overrides from request body
		if r.Body != nil && r.ContentLength > 0 {
			var body struct {
				Workers *int  `json:"workers"`
				Quick   *bool `json:"quick"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if body.Workers != nil && *body.Workers > 0 {
					opts.Workers = *body.Workers
				}
				if body.Quick != nil {
					opts.Quick = *body.Quick
				}
			}
		}

		if err := runner.StartVerify(opts, thermal, cfg.toDndConfig()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "started"})
	}
}

// handleAPIConfig returns the current config as JSON for the JS options panel.
func handleAPIConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := readAppConfig()
		resp := struct {
			Scan    ScanOptions   `json:"scan"`
			Verify  VerifyOptions `json:"verify"`
			Thermal ThermalConfig `json:"thermal"`
			Dnd     DndConfig     `json:"dnd"`
		}{
			Scan:    cfg.toScanOptions(),
			Verify:  cfg.toVerifyOptions(),
			Thermal: cfg.toThermalConfig(),
			Dnd:     cfg.toDndConfig(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleAPIStop(runner *Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := runner.Stop(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopping"})
	}
}

func handleAPIProgress(runner *Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// Send current state immediately
		p := runner.Progress()
		data, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// Subscribe for updates
		id, ch := runner.Subscribe()
		defer runner.Unsubscribe(id)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case prog, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(prog)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

// templateFuncMap is the shared FuncMap used across all templates.
var templateFuncMap = template.FuncMap{
	"formatBytes": format.Size,
	"formatTime": func(t *time.Time) string {
		if t == nil {
			return "Never"
		}
		return t.Format("2006-01-02 15:04:05")
	},
	"formatTimeVal": func(t time.Time) string {
		if t.IsZero() {
			return "Never"
		}
		return t.Format("2006-01-02 15:04:05")
	},
	"truncHash": func(s string) string {
		if len(s) > 16 {
			return s[:16] + "..."
		}
		return s
	},
	"statusClass": func(s string) string {
		switch s {
		case "ok":
			return "status-ok"
		case "corrupted":
			return "status-corrupted"
		case "missing":
			return "status-missing"
		default:
			return "status-unknown"
		}
	},
	"unixTime": func(t *time.Time) int64 {
		if t == nil {
			return 0
		}
		return t.Unix()
	},
	"unixTimeVal": func(t time.Time) int64 {
		if t.IsZero() {
			return 0
		}
		return t.Unix()
	},
	"formatMtime": func(mtime int64) string {
		if mtime == 0 {
			return "Unknown"
		}
		return time.Unix(mtime, 0).Format("2006-01-02 15:04:05")
	},
}

// cachedTemplates holds parsed templates, keyed by content template name.
var cachedTemplates map[string]*template.Template

func init() {
	cachedTemplates = make(map[string]*template.Template, len(templates))
	for name, contentTmpl := range templates {
		tmpl := template.Must(
			template.New("page").Funcs(templateFuncMap).Parse(baseTemplate),
		)
		template.Must(tmpl.Parse(contentTmpl))
		cachedTemplates[name] = tmpl
	}
}

func renderTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	tmpl, ok := cachedTemplates[name]
	if !ok {
		http.Error(w, "unknown template: "+name, 500)
		return
	}

	// Inject version into every render
	data["Version"] = appVersion

	// Buffer template output so errors don't result in partial HTML responses
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("template render error (%s): %v", name, err)
		http.Error(w, "internal server error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	buf.WriteTo(w)
}
