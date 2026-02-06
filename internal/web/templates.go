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
        }
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
    </style>
</head>
<body>
    <nav>
        <div class="container">
            <a href="/" class="logo">filehasher</a>
            <a href="/" {{if eq .Page "overview"}}class="active"{{end}}>Overview</a>
            <a href="/disks" {{if eq .Page "disks"}}class="active"{{end}}>Disks</a>
            <a href="/corrupted" {{if eq .Page "corrupted"}}class="active"{{end}}>Corrupted</a>
            <a href="/missing" {{if eq .Page "missing"}}class="active"{{end}}>Missing</a>
            <a href="/search" {{if eq .Page "search"}}class="active"{{end}}>Search</a>
            <a href="/history" {{if eq .Page "history"}}class="active"{{end}}>History</a>
        </div>
    </nav>
    <div class="container">
        {{template "content" .}}
    </div>
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
                <td class="text-right">{{formatBytes .TotalSize}}</td>
                <td class="text-right {{if gt .CorruptedFiles 0}}status-corrupted{{end}}">{{.CorruptedFiles}}</td>
                <td class="text-right {{if gt .MissingFiles 0}}status-missing{{end}}">{{.MissingFiles}}</td>
                <td class="text-muted">{{formatTime .LastVerified}}</td>
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
                <td class="text-right">{{formatBytes .TotalSize}}</td>
                <td class="text-right {{if gt .CorruptedFiles 0}}status-corrupted{{end}}">{{.CorruptedFiles}}</td>
                <td class="text-right {{if gt .MissingFiles 0}}status-missing{{end}}">{{.MissingFiles}}</td>
                <td class="text-muted">{{formatTime .LastVerified}}</td>
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
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted">{{formatTimeVal .LastVerified}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{end}}`,

	"status_list": `{{define "content"}}
<div class="card">
    <h2>{{.Count}} files found</h2>
    {{if .Files}}
    <table>
        <thead>
            <tr>
                <th>Status</th>
                <th>Disk</th>
                <th>Path</th>
                <th class="text-right">Size</th>
                <th>SHA-256</th>
                <th>Last Verified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
                <td class="text-muted">{{formatTimeVal .LastVerified}}</td>
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
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td class="{{statusClass .Status}}">{{.Status}}</td>
                <td><a href="/disks?name={{.Disk}}" class="disk-link">{{.Disk}}</a></td>
                <td class="path-cell mono">{{.Path}}</td>
                <td class="text-right">{{formatBytes .Size}}</td>
                <td class="mono">{{truncHash .SHA256}}</td>
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
}
