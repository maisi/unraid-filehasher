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
		mux.HandleFunc("/api/progress", handleAPIProgress(runner))
	}

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

const cronCfgPath = "/boot/config/filehasher/cron.cfg"

type cronConfig struct {
	Enabled  bool
	Schedule string
	Mode     string // "full" or "quick"
}

func readCronConfig() cronConfig {
	cfg := cronConfig{
		Schedule: "0 3 * * 0",
		Mode:     "full",
	}

	f, err := os.Open(cronCfgPath)
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
		case "ENABLED":
			cfg.Enabled = val == "yes"
		case "SCHEDULE":
			cfg.Schedule = val
		case "MODE":
			cfg.Mode = val
		}
	}
	return cfg
}

func writeCronConfig(cfg cronConfig) error {
	enabled := "no"
	if cfg.Enabled {
		enabled = "yes"
	}
	content := fmt.Sprintf(`# filehasher cron schedule
# Set ENABLED=yes to enable automatic verification
ENABLED=%s
# Cron schedule (default: Sunday 3 AM)
SCHEDULE=%s
# Verify mode: "full" or "quick"
MODE=%s
`, enabled, cfg.Schedule, cfg.Mode)

	if err := os.MkdirAll("/boot/config/filehasher", 0755); err != nil {
		return err
	}
	if err := os.WriteFile(cronCfgPath, []byte(content), 0644); err != nil {
		return err
	}

	// Re-run the cron installer script if it exists
	script := "/boot/config/plugins/filehasher/filehasher-cron"
	if _, err := os.Stat(script); err == nil {
		exec.Command("/bin/bash", script).Run()
	}
	return nil
}

func handleSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()
			cfg := cronConfig{
				Enabled:  r.FormValue("enabled") == "yes",
				Schedule: r.FormValue("schedule"),
				Mode:     r.FormValue("mode"),
			}
			if cfg.Schedule == "" {
				cfg.Schedule = "0 3 * * 0"
			}
			if cfg.Mode == "" {
				cfg.Mode = "full"
			}
			err := writeCronConfig(cfg)
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

		cfg := readCronConfig()
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
		if err := runner.StartScan(); err != nil {
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
		if err := runner.StartVerify(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "started"})
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
