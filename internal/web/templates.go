package web

var baseTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>filehasher - File Integrity Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            line-height: 1.6;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 0 20px; }
        
        /* Navigation */
        nav {
            background: #161b22;
            border-bottom: 1px solid #30363d;
            padding: 12px 0;
            margin-bottom: 24px;
        }
        nav .container {
            display: flex;
            align-items: center;
            gap: 24px;
        }
        nav .logo {
            font-size: 18px;
            font-weight: 700;
            color: #58a6ff;
            text-decoration: none;
        }
        nav a {
            color: #8b949e;
            text-decoration: none;
            font-size: 14px;
            padding: 4px 8px;
            border-radius: 4px;
        }
        nav a:hover, nav a.active {
            color: #c9d1d9;
            background: #21262d;
        }
        
        /* Cards */
        .card {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 8px;
            padding: 20px;
            margin-bottom: 16px;
        }
        .card h2 {
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 16px;
            color: #e6edf3;
        }
        
        /* Stats Grid */
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
            margin-bottom: 24px;
        }
        .stat-card {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 8px;
            padding: 16px;
            text-align: center;
        }
        .stat-card .value {
            font-size: 32px;
            font-weight: 700;
            color: #e6edf3;
        }
        .stat-card .label {
            font-size: 12px;
            text-transform: uppercase;
            color: #8b949e;
            margin-top: 4px;
        }
        .stat-card.danger .value { color: #f85149; }
        .stat-card.warning .value { color: #d29922; }
        .stat-card.success .value { color: #3fb950; }
        
        /* Tables */
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            text-align: left;
            padding: 8px 12px;
            border-bottom: 1px solid #21262d;
            font-size: 13px;
        }
        th {
            color: #8b949e;
            font-weight: 600;
            text-transform: uppercase;
            font-size: 11px;
            cursor: pointer;
            user-select: none;
            position: relative;
            padding-right: 20px;
        }
        th:hover { color: #c9d1d9; }
        th::after {
            content: "";
            position: absolute;
            right: 4px;
            top: 50%;
            transform: translateY(-50%);
            font-size: 10px;
            color: #484f58;
        }
        th.sort-asc::after { content: "\25B2"; color: #58a6ff; }
        th.sort-desc::after { content: "\25BC"; color: #58a6ff; }
        tr:hover { background: #1c2128; }
        
        /* Status badges */
        .status-ok { color: #3fb950; }
        .status-corrupted { color: #f85149; font-weight: 700; }
        .status-missing { color: #d29922; }
        .status-unknown { color: #8b949e; }
        
        /* Search */
        .search-form {
            display: flex;
            gap: 8px;
            margin-bottom: 20px;
        }
        .search-form input {
            flex: 1;
            padding: 8px 12px;
            background: #0d1117;
            border: 1px solid #30363d;
            border-radius: 6px;
            color: #c9d1d9;
            font-size: 14px;
        }
        .search-form button {
            padding: 8px 16px;
            background: #238636;
            color: #fff;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
        }
        .search-form button:hover { background: #2ea043; }
        
        .mono { font-family: "SFMono-Regular", Consolas, monospace; font-size: 12px; }
        .text-muted { color: #8b949e; }
        .text-right { text-align: right; }
        a.disk-link { color: #58a6ff; text-decoration: none; }
        a.disk-link:hover { text-decoration: underline; }
        .path-cell { word-break: break-all; max-width: 500px; }

        /* Pagination */
        .pagination {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 16px;
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid #21262d;
        }
        .btn {
            padding: 6px 14px;
            background: #21262d;
            color: #c9d1d9;
            border: 1px solid #30363d;
            border-radius: 6px;
            text-decoration: none;
            font-size: 13px;
        }
        .btn:hover { background: #30363d; }
        .btn-primary { background: #238636; border-color: #238636; color: #fff; }
        .btn-primary:hover { background: #2ea043; }
        .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

        /* Progress bar */
        .progress-bar-container {
            width: 100%;
            background: #21262d;
            border-radius: 6px;
            height: 8px;
            overflow: hidden;
        }
        .progress-bar {
            height: 100%;
            background: #58a6ff;
            border-radius: 6px;
            transition: width 0.3s ease;
        }
    </style>
</head>
<body>
    <nav>
        <div class="container">
            <a href="/" class="logo">filehasher</a>
            <a href="/" {{if eq .Page "overview"}}class="active"{{end}}>Overview</a>
            <a href="/disks" {{if eq .Page "disks"}}class="active"{{end}}>Disks</a>
            <a href="/ok" {{if eq .Page "ok"}}class="active"{{end}}>OK</a>
            <a href="/new" {{if eq .Page "new"}}class="active"{{end}}>New</a>
            <a href="/corrupted" {{if eq .Page "corrupted"}}class="active"{{end}}>Corrupted</a>
            <a href="/missing" {{if eq .Page "missing"}}class="active"{{end}}>Missing</a>
            <a href="/search" {{if eq .Page "search"}}class="active"{{end}}>Search</a>
            <a href="/files" {{if eq .Page "files"}}class="active"{{end}}>All Files</a>
            <a href="/history" {{if eq .Page "history"}}class="active"{{end}}>History</a>
            <a href="/settings" {{if eq .Page "settings"}}class="active"{{end}}>Settings</a>
            {{if .Version}}<span class="text-muted" style="margin-left:auto;font-size:12px;">v{{.Version}}</span>{{end}}
        </div>
    </nav>
    <div class="container">
        {{template "content" .}}
    </div>
    <script>
    document.querySelectorAll("table").forEach(function(table) {
        var headers = table.querySelectorAll("thead th");
        if (!headers.length) return;
        headers.forEach(function(th, colIdx) {
            th.addEventListener("click", function() {
                var tbody = table.querySelector("tbody");
                if (!tbody) return;
                var rows = Array.from(tbody.querySelectorAll("tr"));
                if (!rows.length) return;

                var asc = !th.classList.contains("sort-asc");
                headers.forEach(function(h) { h.classList.remove("sort-asc", "sort-desc"); });
                th.classList.add(asc ? "sort-asc" : "sort-desc");

                rows.sort(function(a, b) {
                    var cellA = a.children[colIdx];
                    var cellB = b.children[colIdx];
                    if (!cellA || !cellB) return 0;

                    var valA = cellA.getAttribute("data-sort-value");
                    var valB = cellB.getAttribute("data-sort-value");
                    if (valA === null) valA = cellA.textContent.trim();
                    if (valB === null) valB = cellB.textContent.trim();

                    var numA = parseFloat(valA);
                    var numB = parseFloat(valB);
                    if (!isNaN(numA) && !isNaN(numB) && String(numA) === valA && String(numB) === valB) {
                        return asc ? numA - numB : numB - numA;
                    }
                    return asc ? valA.localeCompare(valB) : valB.localeCompare(valA);
                });

                rows.forEach(function(row) { tbody.appendChild(row); });
            });
        });
    });
    </script>
    <script>
    function startOp(type) {
        var btn = document.getElementById("btn-" + type);
        if (btn) btn.disabled = true;
        fetch("/api/" + type, {method: "POST"})
            .then(function(r) { return r.json(); })
            .then(function(d) {
                if (d.error) { alert(d.error); if (btn) btn.disabled = false; }
                else connectSSE();
            })
            .catch(function(e) { alert("Error: " + e); if (btn) btn.disabled = false; });
    }

    var evtSource = null;
    function connectSSE() {
        if (evtSource) evtSource.close();
        var section = document.getElementById("progress-section");
        if (section) section.style.display = "block";

        evtSource = new EventSource("/api/progress");
        evtSource.onmessage = function(e) {
            var p = JSON.parse(e.data);
            var stateEl = document.getElementById("progress-state");
            var phaseEl = document.getElementById("progress-phase");
            var barEl = document.getElementById("progress-bar");
            var msgEl = document.getElementById("progress-message");
            var btnScan = document.getElementById("btn-scan");
            var btnVerify = document.getElementById("btn-verify");

            if (stateEl) stateEl.textContent = p.state;
            if (phaseEl) phaseEl.textContent = p.phase;
            if (msgEl) msgEl.textContent = p.message;

            var pct = 0;
            if (p.total > 0) pct = Math.round((p.done / p.total) * 100);
            else if (p.done > 0) pct = 50;
            if (barEl) barEl.style.width = pct + "%";

            if (p.state === "idle") {
                if (btnScan) btnScan.disabled = false;
                if (btnVerify) btnVerify.disabled = false;
                if (p.phase === "complete" || p.message) {
                    if (barEl) barEl.style.width = "100%";
                }
                if (evtSource) { evtSource.close(); evtSource = null; }
            } else {
                if (btnScan) btnScan.disabled = true;
                if (btnVerify) btnVerify.disabled = true;
            }
        };
        evtSource.onerror = function() {
            if (evtSource) { evtSource.close(); evtSource = null; }
        };
    }

    // On page load, check if an operation is running
    (function() {
        if (!document.getElementById("progress-section")) return;
        connectSSE();
    })();
    </script>
</body>
</html>`

var templates = map[string]string{
	"overview": `{{define "content"}}
<div class="stats-grid">
    <div class="stat-card">
        <div class="value">{{.Stats.TotalFiles}}</div>
        <div class="label">Total Files</div>
    </div>
    <div class="stat-card">
        <div class="value">{{formatBytes .Stats.TotalSize}}</div>
        <div class="label">Total Size</div>
    </div>
    <div class="stat-card success">
        <div class="value">{{.Stats.OKFiles}}</div>
        <div class="label">OK</div>
    </div>
    <div class="stat-card {{if gt .Stats.CorruptedFiles 0}}danger{{end}}">
        <div class="value">{{.Stats.CorruptedFiles}}</div>
        <div class="label">Corrupted</div>
    </div>
    <div class="stat-card {{if gt .Stats.MissingFiles 0}}warning{{end}}">
        <div class="value">{{.Stats.MissingFiles}}</div>
        <div class="label">Missing</div>
    </div>
    {{if gt .Stats.NewFiles 0}}
    <div class="stat-card">
        <div class="value">{{.Stats.NewFiles}}</div>
        <div class="label">New</div>
    </div>
    {{end}}
</div>

<div class="card">
    <h2>Actions</h2>
    <div style="display:flex;gap:8px;margin-bottom:12px;">
        <button id="btn-scan" class="btn btn-primary" onclick="startOp('scan')">Start Scan</button>
        <button id="btn-verify" class="btn btn-primary" onclick="startOp('verify')">Start Verify</button>
    </div>
    <div id="progress-section" style="display:none;">
        <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px;">
            <span id="progress-state" class="text-muted"></span>
            <span id="progress-phase" class="text-muted"></span>
        </div>
        <div class="progress-bar-container">
            <div id="progress-bar" class="progress-bar" style="width:0%"></div>
        </div>
        <p id="progress-message" class="text-muted" style="margin-top:8px;font-size:13px;"></p>
    </div>
</div>

<div class="card">
    <h2>Scan Information</h2>
    <table>
        <tr><td>Last Scan</td><td>{{formatTime .Stats.LastScan}}</td></tr>
        <tr><td>Last Verify</td><td>{{formatTime .Stats.LastVerify}}</td></tr>
    </table>
</div>

{{if .DiskStats}}
<div class="card">
    <h2>Disk Breakdown</h2>
    <table>
        <thead>
            <tr>
                <th>Disk</th>
                <th class="text-right">Files</th>
                <th class="text-right">Size</th>
                <th class="text-right">Corrupted</th>
                <th class="text-right">Missing</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .DiskStats}}
            <tr>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="text-right">{{.TotalFiles}}</td>
                <td class="text-right" data-sort-value="{{.TotalSize}}">{{formatBytes .TotalSize}}</td>
                <td class="text-right {{if gt .CorruptedFiles 0}}status-corrupted{{end}}">{{.CorruptedFiles}}</td>
                <td class="text-right {{if gt .MissingFiles 0}}status-missing{{end}}">{{.MissingFiles}}</td>
                <td class="text-muted" data-sort-value="{{unixTime .LastVerified}}">{{formatTime .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{end}}
{{end}}`,

	"disks": `{{define "content"}}
<div class="card">
    <h2>All Disks</h2>
    <table>
        <thead>
            <tr>
                <th>Disk</th>
                <th class="text-right">Files</th>
                <th class="text-right">Size</th>
                <th class="text-right">Corrupted</th>
                <th class="text-right">Missing</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .DiskStats}}
            <tr>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="text-right">{{.TotalFiles}}</td>
                <td class="text-right" data-sort-value="{{.TotalSize}}">{{formatBytes .TotalSize}}</td>
                <td class="text-right {{if gt .CorruptedFiles 0}}status-corrupted{{end}}">{{.CorruptedFiles}}</td>
                <td class="text-right {{if gt .MissingFiles 0}}status-missing{{end}}">{{.MissingFiles}}</td>
                <td class="text-muted" data-sort-value="{{unixTime .LastVerified}}">{{formatTime .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{end}}`,

	"disk_detail": `{{define "content"}}
<div class="card">
    <h2>Disk: {{.Disk}} ({{.Count}} files)</h2>
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>Path</th>
                <th class="text-right">Size</th>
                <th>SHA-256</th>
                <th>Modified</th>
                <th>First Seen</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right" data-sort-value="{{.Size}}">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted" data-sort-value="{{.Mtime}}">{{formatMtime .Mtime}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .FirstSeen}}">{{formatTimeVal .FirstSeen}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .LastVerified}}">{{formatTimeVal .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{end}}`,

	"status_list": `{{define "content"}}
<div class="card">
    <h2>{{if .StatusName}}{{.StatusName}} — {{end}}{{.Count}} files found</h2>
    {{if .Files}}
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>Disk</th>
                <th>Path</th>
                <th class="text-right">Size</th>
                <th>SHA-256</th>
                <th>Modified</th>
                <th>First Seen</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right" data-sort-value="{{.Size}}">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted" data-sort-value="{{.Mtime}}">{{formatMtime .Mtime}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .FirstSeen}}">{{formatTimeVal .FirstSeen}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .LastVerified}}">{{formatTimeVal .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <p class="text-muted">No files with this status. That's good!</p>
    {{end}}
</div>
{{end}}`,

	"search": `{{define "content"}}
<div class="card">
    <h2>Search Files</h2>
    <form class="search-form" method="GET" action="/search">
        <input type="text" name="q" placeholder="Search by file path..." value="{{.Query}}" autofocus>
        <button type="submit">Search</button>
    </form>
    {{if .Query}}
    <p class="text-muted" style="margin-bottom: 12px;">{{.Count}} results for "{{.Query}}"</p>
    {{if .Files}}
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>Disk</th>
                <th>Path</th>
                <th class="text-right">Size</th>
                <th>SHA-256</th>
                <th>Modified</th>
                <th>First Seen</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right" data-sort-value="{{.Size}}">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted" data-sort-value="{{.Mtime}}">{{formatMtime .Mtime}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .FirstSeen}}">{{formatTimeVal .FirstSeen}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{end}}
    {{end}}
</div>
{{end}}`,

	"history": `{{define "content"}}
<div class="card">
    <h2>Scan History</h2>
    {{if .History}}
    <table>
        <thead>
            <tr>
                <th>Type</th>
                <th>Started</th>
                <th>Ended</th>
                <th>Duration</th>
                <th>Disks</th>
                <th class="text-right">Files</th>
                <th class="text-right">Errors</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
            {{range .History}}
            <tr>
                <td>{{.scan_type}}</td>
                <td class="text-muted">{{.started_at}}</td>
                <td class="text-muted">{{if .ended_at}}{{.ended_at}}{{else}}-{{end}}</td>
                <td class="text-muted">{{if .duration}}{{.duration}}{{else}}-{{end}}</td>
                <td>{{.disks}}</td>
                <td class="text-right">{{.files_processed}}</td>
                <td class="text-right {{if gt .errors 0}}status-corrupted{{end}}">{{.errors}}</td>
                <td>{{.status}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <p class="text-muted">No scan history yet. Run a scan first!</p>
    {{end}}
</div>
{{end}}`,

	"files": `{{define "content"}}
<div class="card">
    <h2>All Files ({{.Total}} total, showing {{.Count}} on page {{.CurrentPage}} of {{.TotalPages}})</h2>
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>Disk</th>
                <th>Path</th>
                <th class="text-right">Size</th>
                <th>SHA-256</th>
                <th>Modified</th>
                <th>First Seen</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right" data-sort-value="{{.Size}}">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted" data-sort-value="{{.Mtime}}">{{formatMtime .Mtime}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .FirstSeen}}">{{formatTimeVal .FirstSeen}}</td>
                <td class="text-muted" data-sort-value="{{unixTimeVal .LastVerified}}">{{formatTimeVal .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    <div class="pagination">
        {{if .HasPrev}}<a href="/files?page={{.PrevPage}}&per_page={{.PerPage}}" class="btn">Previous</a>{{end}}
        <span class="text-muted">Page {{.CurrentPage}} of {{.TotalPages}}</span>
        {{if .HasNext}}<a href="/files?page={{.NextPage}}&per_page={{.PerPage}}" class="btn">Next</a>{{end}}
    </div>
</div>
{{end}}`,

	"settings": `{{define "content"}}
<div class="card">
    <h2>Scheduled Verification</h2>
    {{if .Message}}<p style="color:#3fb950;margin-bottom:12px;">{{.Message}}</p>{{end}}
    <form method="POST" action="/settings" style="max-width:500px;">
        <div style="margin-bottom:16px;">
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
                <input type="checkbox" name="enabled" value="yes" {{if .Config.Enabled}}checked{{end}}
                    style="width:16px;height:16px;">
                <span>Enable scheduled verification</span>
            </label>
        </div>
        <div style="margin-bottom:16px;">
            <label style="display:block;margin-bottom:4px;color:#8b949e;font-size:12px;text-transform:uppercase;">
                Cron Schedule
            </label>
            <input type="text" name="schedule" value="{{.Config.Schedule}}"
                placeholder="0 3 * * 0"
                style="width:100%;padding:8px 12px;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:14px;font-family:monospace;">
            <p class="text-muted" style="margin-top:4px;font-size:12px;">
                Default: <code>0 3 * * 0</code> (Sunday 3:00 AM).
                Uses standard cron syntax: minute hour day month weekday.
            </p>
        </div>
        <div style="margin-bottom:16px;">
            <label style="display:block;margin-bottom:4px;color:#8b949e;font-size:12px;text-transform:uppercase;">
                Verify Mode
            </label>
            <select name="mode"
                style="width:100%;padding:8px 12px;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:14px;">
                <option value="full" {{if eq .Config.Mode "full"}}selected{{end}}>Full — re-hash every file</option>
                <option value="quick" {{if eq .Config.Mode "quick"}}selected{{end}}>Quick — skip unchanged files</option>
            </select>
        </div>
        <button type="submit" class="btn btn-primary" style="padding:8px 20px;">Save Settings</button>
    </form>
</div>
{{end}}`,
}
