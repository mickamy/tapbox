(() => {
    "use strict";

    const app = document.getElementById("app");
    const detailPanel = document.getElementById("detail-panel");
    const sseStatus = document.getElementById("ws-status");
    let currentTraceID = null;
    let selectedSpanID = null;
    let traces = [];

    // --- SSE ---
    function connectSSE() {
        const es = new EventSource("/events");

        es.onopen = () => {
            sseStatus.textContent = "connected";
            sseStatus.className = "connected";
        };

        es.onerror = () => {
            sseStatus.textContent = "disconnected";
            sseStatus.className = "disconnected";
        };

        es.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            if (msg.type === "trace") {
                updateTrace(msg.data);
            }
        };
    }

    function updateTrace(trace) {
        const idx = traces.findIndex((t) => t.trace_id === trace.trace_id);
        if (idx >= 0) {
            traces[idx] = trace;
        } else {
            traces.push(trace);
        }
        renderList();
        if (currentTraceID === trace.trace_id) {
            renderDetail(trace);
        }
    }

    // --- API ---
    async function fetchTraces() {
        const resp = await fetch("/api/traces?limit=100");
        traces = await resp.json();
        if (!traces) traces = [];
        traces.reverse();
        renderList();
    }

    async function fetchTrace(traceID) {
        const resp = await fetch(`/api/traces/${traceID}`);
        return resp.json();
    }

    async function fetchExplain(query, analyze) {
        const resp = await fetch("/api/explain", {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify({query, analyze}),
        });
        return resp.json();
    }

    // --- Rendering ---
    function renderList() {
        if (!traces || traces.length === 0) {
            app.innerHTML = `
        <div class="empty-state">
          <h2>No traces yet</h2>
          <p>Send requests through the proxy to see them here.</p>
        </div>`;
            return;
        }

        const rows = traces
            .map((t) => {
                const root = findRoot(t);
                const method = root ? rootMethod(root) : "";
                const path = root ? rootPath(root) : "";
                const status = root ? rootStatus(root) : "";
                const statusClass = root && root.status === "error" ? "status-error" : "status-ok";
                const methodClass = `method-${method.toLowerCase()}`;
                const spanCount = t.spans ? t.spans.length : 0;
                const kinds = t.spans
                    ? [...new Set(t.spans.map((s) => kindName(s.kind)))]
                    : [];
                const badges = kinds
                    .map((k) => `<span class="badge badge-${k}">${k}</span>`)
                    .join(" ");
                const selClass = t.trace_id === currentTraceID ? "selected" : "";

                return `<tr data-trace-id="${t.trace_id}" class="${selClass}">
          <td><span class="method ${methodClass}">${escapeHtml(method)}</span></td>
          <td>${escapeHtml(path)}</td>
          <td><span class="${statusClass}">${status}</span></td>
          <td class="duration">${formatDuration(t.duration_ms)}</td>
          <td>${spanCount}</td>
          <td>${badges}</td>
        </tr>`;
            })
            .join("");

        app.innerHTML = `
      <div class="trace-list">
        <table class="trace-table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Path</th>
              <th>Status</th>
              <th>Duration</th>
              <th>Spans</th>
              <th>Types</th>
            </tr>
          </thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`;

        app.querySelectorAll("tr[data-trace-id]").forEach((row) => {
            row.addEventListener("click", () => {
                showDetail(row.dataset.traceId);
            });
        });
    }

    async function showDetail(traceID) {
        if (currentTraceID === traceID) {
            closeDetail();
            return;
        }
        currentTraceID = traceID;
        selectedSpanID = null;
        const trace = await fetchTrace(traceID);
        renderList();
        renderDetail(trace);
    }

    function closeDetail() {
        currentTraceID = null;
        selectedSpanID = null;
        detailPanel.classList.add("hidden");
        detailPanel.innerHTML = "";
        renderList();
    }

    function renderDetail(trace) {
        if (!trace || !trace.spans) return;

        const spans = trace.spans.sort(
            (a, b) => new Date(a.start) - new Date(b.start)
        );
        if (!selectedSpanID || !spans.find((s) => s.span_id === selectedSpanID)) {
            selectedSpanID = spans.length > 0 ? spans[0].span_id : null;
        }

        const traceStart = parseTimeMs(trace.start);
        const traceDuration = trace.duration_ms || 1;

        // Build span tree and flatten to DFS order so that tree rows
        // and timeline rows are rendered in the same order.
        const tree = buildTree(spans);
        const treeHTML = renderTree(tree, 0);
        const orderedSpans = flattenTree(tree);

        // Timeline (uses orderedSpans to match tree row order)
        const timelineHTML = orderedSpans
            .map((s) => {
                const spanStart = parseTimeMs(s.start);
                const offset = ((spanStart - traceStart) / traceDuration) * 100;
                const width = Math.max((s.duration_ms / traceDuration) * 100, 0.5);
                const kindClass = `kind-${kindName(s.kind)}`;
                const statusClass = s.status === "error" ? "status-error" : "";
                const selClass = s.span_id === selectedSpanID ? "selected" : "";
                return `<div class="timeline-bar-container">
          <div class="timeline-bar ${kindClass} ${statusClass} ${selClass}"
               data-span-id="${s.span_id}"
               style="left:${offset}%;width:${width}%"></div>
          <span class="timeline-label">${formatDuration(s.duration_ms)}</span>
        </div>`;
            })
            .join("");

        // Span detail
        const sel = spans.find((s) => s.span_id === selectedSpanID);
        const detailHTML = sel ? renderSpanDetail(sel) : "<p>Select a span</p>";

        detailPanel.classList.remove("hidden");
        detailPanel.innerHTML = `
      <div class="trace-detail">
        <div class="detail-header">
          <a class="back" id="close-btn">&times; Close</a>
          <span>Trace: ${trace.trace_id.substring(0, 12)}...</span>
          <span class="duration">${formatDuration(trace.duration_ms)}</span>
        </div>
        <div class="detail-body">
          <div class="span-tree">${treeHTML}</div>
          <div class="timeline">${timelineHTML}</div>
          <div class="span-detail">${detailHTML}</div>
        </div>
      </div>`;

        // Event handlers
        document.getElementById("close-btn").addEventListener("click", closeDetail);

        detailPanel.querySelectorAll(".span-tree-item").forEach((el) => {
            el.addEventListener("click", () => {
                selectedSpanID = el.dataset.spanId;
                renderDetail(trace);
            });
        });

        detailPanel.querySelectorAll(".timeline-bar").forEach((el) => {
            el.addEventListener("click", () => {
                selectedSpanID = el.dataset.spanId;
                renderDetail(trace);
            });
        });

        detailPanel.querySelectorAll(".explain-btn").forEach((btn) => {
            btn.addEventListener("click", async () => {
                const query = btn.dataset.query;
                const analyze = btn.id === "explain-analyze-btn";
                const container = btn.closest(".explain-buttons");
                let resultEl = container.querySelector(".explain-result");
                if (!resultEl) {
                    resultEl = document.createElement("pre");
                    resultEl.className = "explain-result";
                    container.after(resultEl);
                }
                resultEl.textContent = analyze ? "Running EXPLAIN ANALYZE..." : "Running EXPLAIN...";
                try {
                    const result = await fetchExplain(query, analyze);
                    resultEl.textContent = result.error || result.plan || "No plan returned";
                } catch (e) {
                    resultEl.textContent = "Error: " + e.message;
                }
            });
        });
    }

    function buildTree(spans) {
        const byId = {};
        const roots = [];
        for (const s of spans) {
            byId[s.span_id] = {span: s, children: []};
        }
        for (const s of spans) {
            if (s.parent_id && byId[s.parent_id]) {
                byId[s.parent_id].children.push(byId[s.span_id]);
            } else {
                roots.push(byId[s.span_id]);
            }
        }
        return roots;
    }

    function flattenTree(nodes) {
        const result = [];
        for (const n of nodes) {
            result.push(n.span);
            result.push(...flattenTree(n.children));
        }
        return result;
    }

    function renderTree(nodes, depth) {
        return nodes
            .map((n) => {
                const s = n.span;
                const selClass = s.span_id === selectedSpanID ? "selected" : "";
                const kindBadge = `<span class="badge badge-${kindName(s.kind)}">${kindName(s.kind)}</span>`;
                const statusClass = s.status === "error" ? "status-error" : "";
                const indent = '<span class="indent"></span>'.repeat(depth);
                const name = escapeHtml(s.name || "");
                return `<div class="span-tree-item ${selClass}" data-span-id="${s.span_id}">
          ${indent}${kindBadge} <span class="${statusClass}">${name}</span>
        </div>${renderTree(n.children, depth + 1)}`;
            })
            .join("");
    }

    function renderSpanDetail(s) {
        let html = `<h3>${kindName(s.kind).toUpperCase()} Span</h3>`;
        html += field("Span ID", s.span_id);
        html += field("Parent ID", s.parent_id || "(root)");
        html += field("Duration", formatDuration(s.duration_ms));
        html += field("Status", s.status === "error" ? '<span class="status-error">error</span>' : '<span class="status-ok">ok</span>');

        if (s.kind === "http") {
            // HTTP
            html += field("Method", s.http_method);
            html += field("Path", s.http_path);
            html += field("Status Code", s.http_status_code);
            if (s.http_headers) {
                html += field("Headers", `<pre>${escapeHtml(JSON.stringify(s.http_headers, null, 2))}</pre>`);
            }
            if (s.http_request_body) {
                html += field("Request Body", `<pre>${escapeHtml(formatBody(s.http_request_body))}</pre>`);
            }
            if (s.http_response_body) {
                html += field("Response Body", `<pre>${escapeHtml(formatBody(s.http_response_body))}</pre>`);
            }
        } else if (s.kind === "connect") {
            // Connect RPC
            html += field("Service", s.connect_service);
            html += field("Method", s.connect_method);
            html += field("Content-Type", s.connect_content_type);
            html += field("HTTP Status", s.connect_http_status);
            if (s.connect_is_streaming) html += field("Streaming", "yes");
            if (s.connect_timeout_ms) html += field("Timeout", s.connect_timeout_ms + "ms");
            if (s.connect_error_code) html += field("Error Code", `<span class="status-error">${escapeHtml(s.connect_error_code)}</span>`);
            if (s.connect_error_message) html += field("Error Message", `<span class="status-error">${escapeHtml(s.connect_error_message)}</span>`);
            if (s.connect_headers) {
                html += field("Headers", `<pre>${escapeHtml(JSON.stringify(s.connect_headers, null, 2))}</pre>`);
            }
            if (s.connect_request_body) {
                html += field("Request Body", `<pre>${escapeHtml(formatBody(s.connect_request_body))}</pre>`);
            }
            if (s.connect_response_body) {
                html += field("Response Body", `<pre>${escapeHtml(formatBody(s.connect_response_body))}</pre>`);
            }
        } else if (s.kind === "grpc") {
            // gRPC
            html += field("Service", s.grpc_service);
            html += field("Method", s.grpc_method);
            html += field("Code", s.grpc_code);
            if (s.grpc_metadata) {
                html += field("Metadata", `<pre>${escapeHtml(JSON.stringify(s.grpc_metadata, null, 2))}</pre>`);
            }
            if (s.grpc_request_body) {
                html += field("Request (hex)", `<pre>${s.grpc_request_body}</pre>`);
            }
            if (s.grpc_response_body) {
                html += field("Response (hex)", `<pre>${s.grpc_response_body}</pre>`);
            }
        } else if (s.kind === "sql") {
            // SQL
            html += field("Query", `<pre>${escapeHtml(s.sql_query)}</pre>`);
            if (s.sql_row_count) html += field("Rows", s.sql_row_count);
            if (s.sql_error) html += field("Error", `<span class="status-error">${escapeHtml(s.sql_error)}</span>`);
            if (s.sql_query) {
                html += `<div class="explain-buttons">`;
                html += `<button class="explain-btn" id="explain-btn" data-query="${escapeAttr(s.sql_query)}">EXPLAIN</button>`;
                html += `<button class="explain-btn" id="explain-analyze-btn" data-query="${escapeAttr(s.sql_query)}">EXPLAIN ANALYZE</button>`;
                html += `</div>`;
            }
        }

        return html;
    }

    // --- Helpers ---
    function findRoot(t) {
        if (!t.spans || t.spans.length === 0) return null;
        const root = t.spans.find((s) => !s.parent_id);
        return root || t.spans[0];
    }

    function rootMethod(root) {
        if (root.kind === "connect") return root.connect_method || "";
        return root.http_method || root.grpc_method || "";
    }

    function rootPath(root) {
        if (root.kind === "connect") return root.connect_service ? root.connect_service + "/" + (root.connect_method || "") : root.name || "";
        return root.http_path || root.name || "";
    }

    function rootStatus(root) {
        if (root.kind === "connect") return root.connect_http_status || "";
        return root.http_status_code || root.grpc_code || "";
    }

    function kindName(kind) {
        return kind || "unknown";
    }

    function formatDuration(ms) {
        if (ms === undefined || ms === null) return "-";
        if (ms < 1) return `${(ms * 1000).toFixed(0)}us`;
        if (ms < 1000) return `${ms.toFixed(1)}ms`;
        return `${(ms / 1000).toFixed(2)}s`;
    }

    function formatBody(body) {
        try {
            return JSON.stringify(JSON.parse(body), null, 2);
        } catch {
            return body;
        }
    }

    function field(label, value) {
        if (!value && value !== 0) return "";
        return `<div class="field">
      <div class="field-label">${label}</div>
      <div class="field-value">${value}</div>
    </div>`;
    }

    function escapeHtml(s) {
        if (typeof s !== "string") return String(s);
        return s
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;");
    }

    function escapeAttr(s) {
        return escapeHtml(s).replace(/'/g, "&#39;");
    }

    // Parse RFC3339Nano timestamp with sub-millisecond precision.
    // JavaScript Date truncates to integer milliseconds, which causes
    // up to 1ms error per timestamp. For short traces (2-5ms) this
    // makes child span bars visually extend beyond their parent.
    function parseTimeMs(iso) {
        const d = new Date(iso);
        const epochSec = Math.floor(d.getTime() / 1000);
        const m = iso.match(/\.(\d+)/);
        if (!m) return d.getTime();
        const fracMs = parseInt(m[1].padEnd(9, "0").slice(0, 9), 10) / 1e6;
        return epochSec * 1000 + fracMs;
    }

    // --- Init ---
    connectSSE();
    fetchTraces();
})();
