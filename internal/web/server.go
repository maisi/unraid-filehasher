package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/maisi/unraid-filehasher/internal/db"
	"github.com/maisi/unraid-filehasher/internal/format"
)

// Serve starts the web dashboard on the given address.
func Serve(database *db.DB, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleOverview(database))
	mux.HandleFunc("/disks", handleDisks(database))
	mux.HandleFunc("/corrupted", handleCorrupted(database))
	mux.HandleFunc("/missing", handleMissing(database))
	mux.HandleFunc("/search", handleSearch(database))
	mux.HandleFunc("/history", handleHistory(database))

	// API endpoints (JSON)
	mux.HandleFunc("/api/stats", handleAPIStats(database))
	mux.HandleFunc("/api/disks", handleAPIDisks(database))

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
			"Files": files,
			"Count": len(files),
			"Page":  "corrupted",
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
			"Files": files,
			"Count": len(files),
			"Page":  "missing",
		}
		renderTemplate(w, "status_list", data)
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
