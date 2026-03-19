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
            cursor: pointer;
        }
        .btn:hover { background: #30363d; }
        .btn-primary { background: #238636; border-color: #238636; color: #fff; }
        .btn-primary:hover { background: #2ea043; }
        .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
        .btn-danger { background: #da3633; border-color: #da3633; color: #fff; }
        .btn-danger:hover { background: #f85149; }
        .btn-danger:disabled { opacity: 0.5; cursor: not-allowed; }

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
        .progress-bar.bar-success { background: #3fb950; }
        .progress-bar.bar-warning { background: #d29922; }
        .progress-bar.bar-danger { background: #f85149; }

        /* Result banner */
        .result-banner {
            padding: 12px 16px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            margin-bottom: 12px;
        }
        .result-banner.banner-success {
            background: rgba(63, 185, 80, 0.15);
            border: 1px solid rgba(63, 185, 80, 0.4);
            color: #3fb950;
        }
        .result-banner.banner-cancelled {
            background: rgba(210, 153, 34, 0.15);
            border: 1px solid rgba(210, 153, 34, 0.4);
            color: #d29922;
        }
        .result-banner.banner-error {
            background: rgba(248, 81, 73, 0.15);
            border: 1px solid rgba(248, 81, 73, 0.4);
            color: #f85149;
        }

        /* Per-disk progress */
        .disk-progress-list {
            margin-top: 12px;
        }
        .disk-progress-row {
            display: grid;
            grid-template-columns: 80px 1fr 180px;
            align-items: center;
            gap: 12px;
            padding: 6px 0;
            border-bottom: 1px solid #21262d;
            font-size: 13px;
        }
        .disk-progress-row:last-child { border-bottom: none; }
        .disk-progress-name {
            font-weight: 600;
            color: #58a6ff;
        }
        .disk-progress-bar-wrap {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .disk-progress-bar-wrap .progress-bar-container {
            flex: 1;
        }
        .disk-progress-pct {
            font-size: 12px;
            font-weight: 600;
            color: #e6edf3;
            min-width: 36px;
            text-align: right;
        }
        .disk-progress-stats {
            font-size: 12px;
            color: #8b949e;
            text-align: right;
            white-space: nowrap;
        }
        .disk-progress-phase {
            display: inline-block;
            font-size: 11px;
            padding: 1px 6px;
            border-radius: 4px;
            text-transform: uppercase;
            font-weight: 600;
        }
        .disk-progress-phase.phase-walking {
            background: rgba(88, 166, 255, 0.15);
            color: #58a6ff;
        }
        .disk-progress-phase.phase-hashing,
        .disk-progress-phase.phase-verifying {
            background: rgba(210, 153, 34, 0.15);
            color: #d29922;
        }
        .disk-progress-phase.phase-complete {
            background: rgba(63, 185, 80, 0.15);
            color: #3fb950;
        }
        .disk-progress-phase.phase-cancelled {
            background: rgba(210, 153, 34, 0.15);
            color: #d29922;
        }

        /* Thermal badge */
        .temp-badge {
            display: inline-block;
            font-size: 11px;
            padding: 1px 6px;
            border-radius: 4px;
            font-weight: 600;
            margin-left: 4px;
        }
        .temp-badge.temp-ok { color: #8b949e; }
        .temp-badge.temp-warm { color: #d29922; }
        .temp-badge.temp-hot {
            color: #f85149;
            background: rgba(248, 81, 73, 0.15);
        }
        .temp-badge.temp-paused {
            color: #f85149;
            background: rgba(248, 81, 73, 0.2);
            animation: pulse 1.5s ease-in-out infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }

        /* Options panel */
        .options-toggle {
            font-size: 12px;
            color: #8b949e;
            cursor: pointer;
            user-select: none;
            display: inline-flex;
            align-items: center;
            gap: 4px;
        }
        .options-toggle:hover { color: #c9d1d9; }
        .options-panel {
            display: none;
            margin-top: 12px;
            padding: 16px;
            background: #0d1117;
            border: 1px solid #30363d;
            border-radius: 6px;
        }
        .options-panel.open { display: block; }
        .options-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 12px 24px;
        }
        .opt-group { margin-bottom: 0; }
        .opt-group label {
            display: block;
            font-size: 11px;
            text-transform: uppercase;
            color: #8b949e;
            margin-bottom: 4px;
        }
        .opt-group input[type="number"],
        .opt-group input[type="text"],
        .opt-group select,
        .opt-group textarea {
            width: 100%;
            padding: 6px 10px;
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 4px;
            color: #c9d1d9;
            font-size: 13px;
            font-family: inherit;
        }
        .opt-group textarea {
            font-family: "SFMono-Regular", Consolas, monospace;
            font-size: 12px;
            resize: vertical;
            min-height: 60px;
        }
        .opt-check {
            display: flex;
            align-items: center;
            gap: 6px;
            cursor: pointer;
            font-size: 13px;
        }
        .opt-check input { width: 14px; height: 14px; }

        /* Settings cards */
        .settings-grid {
            display: grid;
            grid-template-columns: 1fr;
            gap: 16px;
            max-width: 640px;
        }

        /* Elapsed time */
        .progress-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 8px;
        }
        .progress-header-left {
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .elapsed-time {
            font-size: 13px;
            color: #8b949e;
            font-family: "SFMono-Regular", Consolas, monospace;
        }
        .overall-speed {
            font-size: 12px;
            color: #8b949e;
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
    // --- Utility functions ---
    function formatBytes(bytes) {
        if (bytes === 0) return "0 B";
        var units = ["B", "KB", "MB", "GB", "TB"];
        var i = 0;
        var v = bytes;
        while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
        return v.toFixed(i === 0 ? 0 : 1) + " " + units[i];
    }

    function formatElapsed(seconds) {
        var h = Math.floor(seconds / 3600);
        var m = Math.floor((seconds % 3600) / 60);
        var s = Math.floor(seconds % 60);
        if (h > 0) return h + "h " + (m < 10 ? "0" : "") + m + "m " + (s < 10 ? "0" : "") + s + "s";
        if (m > 0) return m + "m " + (s < 10 ? "0" : "") + s + "s";
        return s + "s";
    }

    // --- State ---
    var evtSource = null;
    var elapsedTimer = null;
    var opStartTime = null;

    function startOp(type) {
        var btn = document.getElementById("btn-" + type);
        if (btn) btn.disabled = true;

        // Collect options from the options panel (if present)
        var body = null;
        var headers = {};
        var panel = document.getElementById("options-panel");
        if (panel) {
            if (type === "scan") {
                var opts = {};
                var exclEl = document.getElementById("opt-excludes");
                if (exclEl && exclEl.value.trim()) {
                    opts.excludes = exclEl.value.trim().split("\n").filter(function(l){return l.trim();});
                }
                var exclApp = document.getElementById("opt-exclude-appdata");
                if (exclApp) opts.excludeAppdata = exclApp.checked;
                var fullEl = document.getElementById("opt-full-scan");
                if (fullEl) opts.fullScan = fullEl.checked;
                var twoPhase = document.getElementById("opt-hdd-two-phase");
                if (twoPhase) opts.hddTwoPhase = twoPhase.checked;
                var dtEl = document.getElementById("opt-disk-type");
                if (dtEl) opts.diskType = dtEl.value;
                body = JSON.stringify(opts);
                headers["Content-Type"] = "application/json";
            } else if (type === "verify") {
                var opts = {};
                var wEl = document.getElementById("opt-workers");
                if (wEl && wEl.value) opts.workers = parseInt(wEl.value, 10);
                var qEl = document.getElementById("opt-quick");
                if (qEl) opts.quick = qEl.checked;
                body = JSON.stringify(opts);
                headers["Content-Type"] = "application/json";
            }
        }

        fetch("/api/" + type, {method: "POST", body: body, headers: headers})
            .then(function(r) { return r.json(); })
            .then(function(d) {
                if (d.error) { alert(d.error); if (btn) btn.disabled = false; }
                else connectSSE();
            })
            .catch(function(e) { alert("Error: " + e); if (btn) btn.disabled = false; });
    }

    function stopOp() {
        var btn = document.getElementById("btn-stop");
        if (btn) btn.disabled = true;
        fetch("/api/stop", {method: "POST"})
            .then(function(r) { return r.json(); })
            .then(function(d) {
                if (d.error) { alert(d.error); if (btn) btn.disabled = false; }
            })
            .catch(function(e) { alert("Error: " + e); if (btn) btn.disabled = false; });
    }

    function startElapsedTimer() {
        stopElapsedTimer();
        elapsedTimer = setInterval(function() {
            if (!opStartTime) return;
            var elapsed = (Date.now() - opStartTime) / 1000;
            var el = document.getElementById("elapsed-time");
            if (el) el.textContent = formatElapsed(elapsed);
        }, 1000);
    }

    function stopElapsedTimer() {
        if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null; }
    }

    function renderDiskProgress(disks) {
        var container = document.getElementById("disk-progress-list");
        if (!container) return;
        if (!disks || disks.length === 0) {
            container.innerHTML = "";
            return;
        }

        var html = "";
        for (var i = 0; i < disks.length; i++) {
            var d = disks[i];
            var pct = 0;
            if (d.bytesTotal > 0) {
                pct = Math.round((d.bytesDone / d.bytesTotal) * 100);
            } else if (d.filesFound > 0 && d.filesDone > 0) {
                pct = Math.round((d.filesDone / d.filesFound) * 100);
            } else if (d.phase === "walking" && d.filesFound > 0) {
                pct = 0; // walking, no percentage yet
            }
            if (d.phase === "complete") pct = 100;

            var barClass = "progress-bar";
            if (d.phase === "complete") barClass += " bar-success";
            else if (d.phase === "cancelled") barClass += " bar-warning";

            var phaseClass = "disk-progress-phase phase-" + d.phase;

            var statsText = "";
            if (d.phase === "walking") {
                statsText = d.filesFound + " files found (" + formatBytes(d.bytesTotal) + ")";
            } else if (d.phase === "hashing" || d.phase === "verifying") {
                statsText = d.filesDone + " / " + d.filesFound + " files (" + formatBytes(d.bytesDone) + " / " + formatBytes(d.bytesTotal) + ")";
            } else if (d.phase === "complete") {
                statsText = d.filesDone + " files (" + formatBytes(d.bytesDone) + ")";
            } else if (d.phase === "cancelled") {
                statsText = d.filesDone + " / " + d.filesFound + " files";
            }

            // Temperature badge
            var tempBadge = "";
            if (d.paused && d.temp >= 0) {
                tempBadge = ' <span class="temp-badge temp-paused">PAUSED ' + d.temp + '\u00B0C</span>';
            } else if (d.temp >= 0) {
                var tClass = "temp-ok";
                if (d.temp >= 55) tClass = "temp-hot";
                else if (d.temp >= 45) tClass = "temp-warm";
                tempBadge = ' <span class="temp-badge ' + tClass + '">' + d.temp + '\u00B0C</span>';
            }

            html += '<div class="disk-progress-row">';
            html += '<div><span class="disk-progress-name">' + d.disk + '</span> <span class="' + phaseClass + '">' + d.phase + '</span>' + tempBadge + '</div>';
            html += '<div class="disk-progress-bar-wrap">';
            html += '<div class="progress-bar-container"><div class="' + barClass + '" style="width:' + pct + '%"></div></div>';
            html += '<span class="disk-progress-pct">' + pct + '%</span>';
            html += '</div>';
            html += '<div class="disk-progress-stats">' + statsText + '</div>';
            html += '</div>';
        }
        container.innerHTML = html;
    }

    function connectSSE() {
        if (evtSource) evtSource.close();
        var section = document.getElementById("progress-section");
        var banner = document.getElementById("result-banner");
        if (section) section.style.display = "block";
        if (banner) banner.style.display = "none";

        evtSource = new EventSource("/api/progress");
        evtSource.onmessage = function(e) {
            var p = JSON.parse(e.data);
            var barEl = document.getElementById("progress-bar");
            var msgEl = document.getElementById("progress-message");
            var btnScan = document.getElementById("btn-scan");
            var btnVerify = document.getElementById("btn-verify");
            var btnStop = document.getElementById("btn-stop");
            var speedEl = document.getElementById("overall-speed");

            if (p.state !== "idle") {
                // Active operation
                if (p.started) {
                    opStartTime = new Date(p.started).getTime();
                    startElapsedTimer();
                }

                if (btnScan) btnScan.disabled = true;
                if (btnVerify) btnVerify.disabled = true;
                if (btnStop) { btnStop.style.display = "inline-block"; btnStop.disabled = false; }
                if (section) section.style.display = "block";

                // Overall progress bar
                var pct = 0;
                var totalBytes = 0;
                var doneBytes = 0;
                if (p.disks && p.disks.length > 0) {
                    for (var i = 0; i < p.disks.length; i++) {
                        totalBytes += p.disks[i].bytesTotal;
                        doneBytes += p.disks[i].bytesDone;
                    }
                    if (totalBytes > 0) pct = Math.round((doneBytes / totalBytes) * 100);
                    else if (p.total > 0) pct = Math.round((p.done / p.total) * 100);
                    else if (p.done > 0) pct = 10; // walking phase, show minimal progress
                } else if (p.total > 0) {
                    pct = Math.round((p.done / p.total) * 100);
                } else if (p.done > 0) {
                    pct = 10;
                }
                if (barEl) barEl.style.width = pct + "%";
                if (barEl) { barEl.className = "progress-bar"; }

                // DnD paused indicator
                var dndEl = document.getElementById("dnd-paused-banner");
                if (dndEl) {
                    if (p.dndPaused) {
                        dndEl.style.display = "block";
                    } else {
                        dndEl.style.display = "none";
                    }
                }

                // Speed calculation
                if (speedEl && opStartTime && doneBytes > 0) {
                    var elapsedSec = (Date.now() - opStartTime) / 1000;
                    if (elapsedSec > 0) {
                        var speed = doneBytes / elapsedSec;
                        speedEl.textContent = formatBytes(speed) + "/s";
                    }
                } else if (speedEl) {
                    speedEl.textContent = "";
                }

                if (msgEl) msgEl.textContent = p.message || "";

                // Per-disk progress
                renderDiskProgress(p.disks);

            } else {
                // Idle - operation finished (or was never running)
                stopElapsedTimer();
                if (btnScan) btnScan.disabled = false;
                if (btnVerify) btnVerify.disabled = false;
                if (btnStop) btnStop.style.display = "none";

                var dndEl2 = document.getElementById("dnd-paused-banner");
                if (dndEl2) dndEl2.style.display = "none";

                if (p.phase === "complete" || p.phase === "cancelled" || p.phase === "error") {
                    // Show result banner
                    if (banner) {
                        banner.style.display = "block";
                        banner.className = "result-banner";
                        if (p.phase === "complete") {
                            banner.classList.add("banner-success");
                            banner.textContent = p.message;
                            if (barEl) { barEl.style.width = "100%"; barEl.className = "progress-bar bar-success"; }
                        } else if (p.phase === "cancelled") {
                            banner.classList.add("banner-cancelled");
                            banner.textContent = p.message;
                            if (barEl) { barEl.className = "progress-bar bar-warning"; }
                        } else if (p.phase === "error") {
                            banner.classList.add("banner-error");
                            banner.textContent = p.message;
                            if (barEl) { barEl.className = "progress-bar bar-danger"; }
                        }
                    }

                    // Show final elapsed time
                    if (opStartTime) {
                        var finalElapsed = (Date.now() - opStartTime) / 1000;
                        var el = document.getElementById("elapsed-time");
                        if (el) el.textContent = formatElapsed(finalElapsed);
                    }

                    // Show final disk progress
                    renderDiskProgress(p.disks);

                    // Keep progress section visible to show the result
                    if (section) section.style.display = "block";
                    if (msgEl) msgEl.textContent = "";

                    var speedEl2 = document.getElementById("overall-speed");
                    if (speedEl2) speedEl2.textContent = "";
                } else {
                    // Truly idle with no completion info (initial page load, no operation ran)
                    if (section) section.style.display = "none";
                }

                opStartTime = null;
                if (evtSource) { evtSource.close(); evtSource = null; }
            }
        };
        evtSource.onerror = function() {
            if (evtSource) { evtSource.close(); evtSource = null; }
        };
    }

    // On page load, check if an operation is running
    (function() {
        if (!document.getElementById("progress-section")) return;
        loadDefaults();
        connectSSE();
    })();

    // --- Options panel ---
    function toggleOptions() {
        var panel = document.getElementById("options-panel");
        var arrow = document.getElementById("options-arrow");
        if (!panel) return;
        if (panel.classList.contains("open")) {
            panel.classList.remove("open");
            if (arrow) arrow.innerHTML = "&#9654;";
        } else {
            panel.classList.add("open");
            if (arrow) arrow.innerHTML = "&#9660;";
        }
    }

    function switchOptTab(tab) {
        var scanTab = document.getElementById("opt-tab-scan");
        var verifyTab = document.getElementById("opt-tab-verify");
        var btnScan = document.getElementById("tab-scan");
        var btnVerify = document.getElementById("tab-verify");
        if (tab === "scan") {
            if (scanTab) scanTab.style.display = "";
            if (verifyTab) verifyTab.style.display = "none";
            if (btnScan) { btnScan.className = "btn btn-primary"; }
            if (btnVerify) { btnVerify.className = "btn"; }
        } else {
            if (scanTab) scanTab.style.display = "none";
            if (verifyTab) verifyTab.style.display = "";
            if (btnScan) { btnScan.className = "btn"; }
            if (btnVerify) { btnVerify.className = "btn btn-primary"; }
        }
    }

    function loadDefaults() {
        fetch("/api/config")
            .then(function(r) { return r.json(); })
            .then(function(cfg) {
                // Scan defaults
                var el;
                if (cfg.scan) {
                    el = document.getElementById("opt-excludes");
                    if (el && cfg.scan.excludes && cfg.scan.excludes.length > 0) {
                        el.value = cfg.scan.excludes.join("\n");
                    }
                    el = document.getElementById("opt-exclude-appdata");
                    if (el) el.checked = !!cfg.scan.excludeAppdata;
                    el = document.getElementById("opt-full-scan");
                    if (el) el.checked = !!cfg.scan.fullScan;
                    el = document.getElementById("opt-hdd-two-phase");
                    if (el) el.checked = !!cfg.scan.hddTwoPhase;
                    el = document.getElementById("opt-disk-type");
                    if (el && cfg.scan.diskType) el.value = cfg.scan.diskType;
                }
                // Verify defaults
                if (cfg.verify) {
                    el = document.getElementById("opt-workers");
                    if (el && cfg.verify.workers) el.value = cfg.verify.workers;
                    el = document.getElementById("opt-quick");
                    if (el) el.checked = !!cfg.verify.quick;
                }
            })
            .catch(function() { /* ignore - defaults are fine */ });
    }
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
        <button id="btn-stop" class="btn btn-danger" onclick="stopOp()" style="display:none;">Stop</button>
    </div>
    <div style="margin-bottom:12px;">
        <span class="options-toggle" onclick="toggleOptions()">
            <span id="options-arrow">&#9654;</span> Options
        </span>
    </div>
    <div id="options-panel" class="options-panel">
        <div style="display:flex;gap:12px;margin-bottom:12px;">
            <span id="tab-scan" class="btn btn-primary" style="padding:4px 12px;font-size:12px;cursor:pointer;" onclick="switchOptTab('scan')">Scan</span>
            <span id="tab-verify" class="btn" style="padding:4px 12px;font-size:12px;cursor:pointer;" onclick="switchOptTab('verify')">Verify</span>
        </div>
        <!-- Scan options tab -->
        <div id="opt-tab-scan" class="options-grid">
            <div class="opt-group" style="grid-column:1/3;">
                <label>Exclude Patterns (one regex per line)</label>
                <textarea id="opt-excludes" rows="3" placeholder="\.tmp$&#10;/\.Trash-"></textarea>
            </div>
            <div style="display:flex;flex-direction:column;gap:8px;">
                <label class="opt-check">
                    <input type="checkbox" id="opt-exclude-appdata">
                    <span>Exclude appdata</span>
                </label>
                <label class="opt-check">
                    <input type="checkbox" id="opt-full-scan">
                    <span>Full scan</span>
                </label>
            </div>
            <div style="display:flex;flex-direction:column;gap:8px;">
                <label class="opt-check">
                    <input type="checkbox" id="opt-hdd-two-phase">
                    <span>HDD two-phase</span>
                </label>
                <div class="opt-group">
                    <label>Disk Type</label>
                    <select id="opt-disk-type">
                        <option value="auto">Auto-detect</option>
                        <option value="hdd">HDD</option>
                        <option value="ssd">SSD</option>
                    </select>
                </div>
            </div>
        </div>
        <!-- Verify options tab -->
        <div id="opt-tab-verify" class="options-grid" style="display:none;">
            <div class="opt-group">
                <label>Worker Threads</label>
                <input type="number" id="opt-workers" min="1" max="64" value="4" style="max-width:80px;">
            </div>
            <div style="padding-top:18px;">
                <label class="opt-check">
                    <input type="checkbox" id="opt-quick">
                    <span>Quick verify</span>
                </label>
            </div>
        </div>
    </div>
    <div id="result-banner" class="result-banner" style="display:none;"></div>
    <div id="progress-section" style="display:none;">
        <div class="progress-header">
            <div class="progress-header-left">
                <span id="progress-state" class="text-muted"></span>
                <span id="overall-speed" class="overall-speed"></span>
            </div>
            <span id="elapsed-time" class="elapsed-time"></span>
        </div>
        <div class="progress-bar-container">
            <div id="progress-bar" class="progress-bar" style="width:0%"></div>
        </div>
        <div id="dnd-paused-banner" class="result-banner banner-cancelled" style="display:none;margin-top:8px;">
            Do Not Disturb active &mdash; all operations paused until the DnD window ends.
        </div>
        <p id="progress-message" class="text-muted" style="margin-top:8px;font-size:13px;"></p>
        <div id="disk-progress-list" class="disk-progress-list"></div>
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
{{if .Message}}<div class="result-banner banner-success" style="max-width:640px;">{{.Message}}</div>{{end}}
<form method="POST" action="/settings">
<div class="settings-grid">

    <!-- Scheduled Verification -->
    <div class="card">
        <h2>Scheduled Verification</h2>
        <div style="margin-bottom:16px;">
            <label class="opt-check">
                <input type="checkbox" name="enabled" value="yes" {{if .Config.Enabled}}checked{{end}}>
                <span>Enable scheduled verification</span>
            </label>
        </div>
        <div class="opt-group" style="margin-bottom:12px;">
            <label>Cron Schedule</label>
            <input type="text" name="schedule" value="{{.Config.Schedule}}" placeholder="0 3 * * 0"
                style="font-family:'SFMono-Regular',Consolas,monospace;">
            <p class="text-muted" style="margin-top:4px;font-size:12px;">
                Default: <code>0 3 * * 0</code> (Sunday 3:00 AM). Standard cron: min hour day month weekday.
            </p>
        </div>
        <div class="opt-group">
            <label>Verify Mode</label>
            <select name="mode">
                <option value="full" {{if eq .Config.Mode "full"}}selected{{end}}>Full -- re-hash every file</option>
                <option value="quick" {{if eq .Config.Mode "quick"}}selected{{end}}>Quick -- skip unchanged files</option>
            </select>
        </div>
    </div>

    <!-- Scan Defaults -->
    <div class="card">
        <h2>Scan Defaults</h2>
        <p class="text-muted" style="font-size:12px;margin-bottom:12px;">
            These are used when starting a scan from the Overview page. Can be overridden per-operation.
        </p>
        <div class="opt-group" style="margin-bottom:12px;">
            <label>Exclude Patterns (one regex per line)</label>
            <textarea name="scan_excludes" rows="4" placeholder="\.tmp$&#10;/\.Trash-">{{.Config.ScanExcludes}}</textarea>
        </div>
        <div style="display:flex;flex-direction:column;gap:10px;">
            <label class="opt-check">
                <input type="checkbox" name="scan_exclude_appdata" value="yes" {{if .Config.ScanExcludeAppdata}}checked{{end}}>
                <span>Exclude appdata share</span>
            </label>
            <label class="opt-check">
                <input type="checkbox" name="scan_full" value="yes" {{if .Config.ScanFull}}checked{{end}}>
                <span>Full scan (re-hash all files, ignore incremental cache)</span>
            </label>
            <label class="opt-check">
                <input type="checkbox" name="scan_hdd_two_phase" value="yes" {{if .Config.ScanHddTwoPhase}}checked{{end}}>
                <span>HDD two-phase (walk first, then hash sequentially)</span>
            </label>
        </div>
        <div class="opt-group" style="margin-top:12px;">
            <label>Disk Type Override</label>
            <select name="scan_disk_type">
                <option value="auto" {{if eq .Config.ScanDiskType "auto"}}selected{{end}}>Auto-detect</option>
                <option value="hdd" {{if eq .Config.ScanDiskType "hdd"}}selected{{end}}>HDD (all disks treated as HDD)</option>
                <option value="ssd" {{if eq .Config.ScanDiskType "ssd"}}selected{{end}}>SSD (all disks treated as SSD)</option>
            </select>
        </div>
    </div>

    <!-- Verify Defaults -->
    <div class="card">
        <h2>Verify Defaults</h2>
        <p class="text-muted" style="font-size:12px;margin-bottom:12px;">
            These are used when starting a verify from the Overview page.
        </p>
        <div class="opt-group" style="margin-bottom:12px;">
            <label>Worker Threads</label>
            <input type="number" name="verify_workers" value="{{.Config.VerifyWorkers}}" min="1" max="64" style="max-width:100px;">
            <p class="text-muted" style="margin-top:4px;font-size:12px;">
                Concurrent file verification workers. Default: 4.
            </p>
        </div>
        <label class="opt-check">
            <input type="checkbox" name="verify_quick" value="yes" {{if .Config.VerifyQuick}}checked{{end}}>
            <span>Quick verify (skip files whose size/mtime haven't changed)</span>
        </label>
    </div>

    <!-- Thermal Protection -->
    <div class="card">
        <h2>Thermal Protection</h2>
        <p class="text-muted" style="font-size:12px;margin-bottom:12px;">
            Pause hashing on individual disks when they get too hot. Uses Unraid's cached disk temps with smartctl fallback.
        </p>
        <div style="margin-bottom:12px;">
            <label class="opt-check">
                <input type="checkbox" name="thermal_enabled" value="yes" {{if .Config.ThermalEnabled}}checked{{end}}>
                <span>Enable thermal protection</span>
            </label>
        </div>
        <div class="opt-group" style="margin-bottom:12px;">
            <label>Poll Interval (seconds)</label>
            <input type="number" name="thermal_poll_secs" value="{{.Config.ThermalPollSecs}}" min="10" max="600" style="max-width:100px;">
        </div>
        <div class="options-grid" style="gap:12px 20px;">
            <div>
                <p style="font-size:12px;font-weight:600;color:#e6edf3;margin-bottom:8px;">HDD Thresholds</p>
                <div class="opt-group" style="margin-bottom:8px;">
                    <label>Pause at (&deg;C)</label>
                    <input type="number" name="thermal_hdd_pause" value="{{.Config.ThermalHddPause}}" min="30" max="80" style="max-width:80px;">
                </div>
                <div class="opt-group">
                    <label>Resume at (&deg;C)</label>
                    <input type="number" name="thermal_hdd_resume" value="{{.Config.ThermalHddResume}}" min="20" max="70" style="max-width:80px;">
                </div>
            </div>
            <div>
                <p style="font-size:12px;font-weight:600;color:#e6edf3;margin-bottom:8px;">SSD / NVMe Thresholds</p>
                <div class="opt-group" style="margin-bottom:8px;">
                    <label>Pause at (&deg;C)</label>
                    <input type="number" name="thermal_ssd_pause" value="{{.Config.ThermalSsdPause}}" min="40" max="100" style="max-width:80px;">
                </div>
                <div class="opt-group">
                    <label>Resume at (&deg;C)</label>
                    <input type="number" name="thermal_ssd_resume" value="{{.Config.ThermalSsdResume}}" min="30" max="90" style="max-width:80px;">
                </div>
            </div>
        </div>
        <p class="text-muted" style="margin-top:8px;font-size:12px;">
            Default: HDD 55/45&deg;C, SSD 70/60&deg;C. The 10&deg;C hysteresis prevents rapid on/off cycling.
        </p>
    </div>

    <!-- Do Not Disturb -->
    <div class="card">
        <h2>Do Not Disturb</h2>
        <p class="text-muted" style="font-size:12px;margin-bottom:12px;">
            Pause all hashing operations during a daily time window. Useful for media playback hours.
            The current file finishes hashing before pausing.
        </p>
        <div style="margin-bottom:12px;">
            <label class="opt-check">
                <input type="checkbox" name="dnd_enabled" value="yes" {{if .Config.DndEnabled}}checked{{end}}>
                <span>Enable Do Not Disturb schedule</span>
            </label>
        </div>
        <div class="options-grid" style="gap:12px 20px;">
            <div class="opt-group">
                <label>Start Time</label>
                <input type="time" name="dnd_start" value="{{.Config.DndStart}}" style="max-width:140px;">
            </div>
            <div class="opt-group">
                <label>End Time</label>
                <input type="time" name="dnd_end" value="{{.Config.DndEnd}}" style="max-width:140px;">
            </div>
        </div>
        <p class="text-muted" style="margin-top:8px;font-size:12px;">
            Default: 18:00&ndash;06:00. Supports overnight windows (start &gt; end wraps past midnight).
        </p>
    </div>

</div>
<div style="margin-top:16px;max-width:640px;">
    <button type="submit" class="btn btn-primary" style="padding:8px 24px;">Save All Settings</button>
</div>
</form>
{{end}}`,
}
