package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"

	"pulseops/internal/broker"
	"pulseops/internal/telemetry"
	"pulseops/internal/views"
	"pulseops/internal/views/components"
)

func render(c templ.Component) string {
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		log.Printf("[Render Error] %v\n", err)
		return ""
	}
	return buf.String()
}

func main() {
	state := telemetry.NewTelemetryState()
	sseBroker := broker.NewBroker()

	// 1. Start SSE Fan-Out Broker
	go sseBroker.Start()

	// 2. Background Real-time Telemetry Broadcaster
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if sseBroker.ClientCount() == 0 {
				continue
			}

			activeNode, logItem, alert := state.Tick()
			activeIncidents := state.GetActiveIncidents()

			// Broadcast 1: Telemetry Grid + Header Status + Header Alert Counter (OOB Swaps)
			var telePayload bytes.Buffer
			telePayload.WriteString(render(components.MetricsGrid(activeNode, false)))
			telePayload.WriteString(render(components.HeaderStatusBadge(activeNode.Metrics.Status, true)))
			telePayload.WriteString(render(components.HeaderAlertCounter(len(activeIncidents), true)))
			sseBroker.BroadcastEvent("telemetry", telePayload.String())

			// Broadcast 2: Event Logs & Real-Time Toasts (OOB)
			if logItem != nil {
				var logPayload bytes.Buffer
				logPayload.WriteString(render(components.LogRow(*logItem)))

				if alert != nil {
					// Toast melayang di pojok kanan bawah
					logPayload.WriteString(render(components.ToastOOB(*alert)))
					// Update juga panel insiden aktif secara OOB!
					logPayload.WriteString(render(components.IncidentsPanel(activeIncidents, true)))
				}

				sseBroker.BroadcastEvent("log-entry", logPayload.String())
			}
		}
	}()

	// 3. HTTP Server Endpoints
	mux := http.NewServeMux()

	// GET / (Main Dashboard)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if nodeParam := r.URL.Query().Get("node"); nodeParam != "" {
			state.SetSelectedNode(nodeParam)
		}
		activeNode := state.GetSelectedNode()
		allNodes := state.GetNodesList()
		incidents := state.GetActiveIncidents()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Dashboard(activeNode, allNodes, state.RecentLogs, incidents).Render(r.Context(), w)
	})

	// GET /events/stream (SSE Stream for HTMX)
	mux.HandleFunc("GET /events/stream", sseBroker.ServeHTTP)

	// POST /api/node/{id}/select (Cluster Switcher with OOB Swap)
	mux.HandleFunc("POST /api/node/{id}/select", func(w http.ResponseWriter, r *http.Request) {
		nodeID := r.PathValue("id")
		node := state.SetSelectedNode(nodeID)
		allNodes := state.GetNodesList()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var buf bytes.Buffer
		// Swap target: #metrics-grid
		buf.WriteString(render(components.MetricsGrid(node, false)))
		// OOB Swap: Update tab styling di #node-selector
		buf.WriteString(render(components.NodeSelector(allNodes, node.ID, true)))
		w.Write(buf.Bytes())
	})

	// POST /api/incidents/{id}/ack (Acknowledge Incident via HTMX)
	mux.HandleFunc("POST /api/incidents/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		incID := r.PathValue("id")
		inc := state.AcknowledgeIncident(incID)
		if inc == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		components.IncidentCard(*inc).Render(r.Context(), w)
	})

	// POST /api/incidents/{id}/resolve (Resolve Incident with OOB Updates)
	mux.HandleFunc("POST /api/incidents/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		incID := r.PathValue("id")
		resolved, remainingCount := state.ResolveIncident(incID)
		if resolved == nil {
			http.NotFound(w, r)
			return
		}

		// Tambahkan audit log
		nowStr := time.Now().Format("15:04:05")
		auditLog := telemetry.EventLog{
			ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     telemetry.LevelInfo,
			Service:   "incident-manager",
			Message:   fmt.Sprintf("Operator resolved incident %s (%s)", resolved.ID, resolved.Title),
		}
		state.RecentLogs = append([]telemetry.EventLog{auditLog}, state.RecentLogs...)

		// Respons HTMX:
		// Target #incident-{id} akan kosong (dihapus dari DOM)
		// + OOB Swap #header-alert-counter
		// + OOB Append baris audit log baru ke #log-table-body
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var buf bytes.Buffer
		buf.WriteString(render(components.HeaderAlertCounter(remainingCount, true)))
		buf.WriteString(fmt.Sprintf(`<tbody id="log-table-body" hx-swap-oob="afterbegin">%s</tbody>`, render(components.LogRow(auditLog))))
		w.Write(buf.Bytes())
	})

	// GET /api/logs/filter (Active Debounced Search & Severity Filters)
	mux.HandleFunc("GET /api/logs/filter", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		level := r.URL.Query().Get("level")
		filtered := state.FilterLogs(query, level)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		for _, l := range filtered {
			components.LogRow(l).Render(r.Context(), w)
		}
	})

	// Chaos Controls
	broadcastChaos := func(title, msg, service string, level telemetry.LogLevel) {
		nowStr := time.Now().Format("15:04:05")
		activeNode := state.GetSelectedNode()
		allIncidents := state.GetActiveIncidents()

		alert := &telemetry.AlertNotification{
			ID:        fmt.Sprintf("alt-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     level,
			Title:     title,
			Message:   msg,
		}

		logItem := telemetry.EventLog{
			ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     level,
			Service:   service,
			Message:   msg,
			IsSpike:   true,
		}
		state.RecentLogs = append([]telemetry.EventLog{logItem}, state.RecentLogs...)

		// Broadcast Telemetry Update OOB
		var teleBuf bytes.Buffer
		teleBuf.WriteString(render(components.MetricsGrid(activeNode, false)))
		teleBuf.WriteString(render(components.HeaderStatusBadge(activeNode.Metrics.Status, true)))
		teleBuf.WriteString(render(components.HeaderAlertCounter(len(allIncidents), true)))
		sseBroker.BroadcastEvent("telemetry", teleBuf.String())

		// Broadcast Log + Toast + Incidents Panel OOB
		var logBuf bytes.Buffer
		logBuf.WriteString(render(components.LogRow(logItem)))
		logBuf.WriteString(render(components.ToastOOB(*alert)))
		logBuf.WriteString(render(components.IncidentsPanel(allIncidents, true)))
		sseBroker.BroadcastEvent("log-entry", logBuf.String())
	}

	mux.HandleFunc("POST /api/chaos/cpu-spike", func(w http.ResponseWriter, r *http.Request) {
		node := state.GetSelectedNode()
		node.Metrics.CPUPercent = 99.2
		node.Metrics.Status = "CRITICAL"
		inc := telemetry.Incident{
			ID:        fmt.Sprintf("inc-%d", time.Now().UnixNano()%100000),
			NodeID:    node.ID,
			Title:     "CPU Thermal Overload (99.2%)",
			Severity:  telemetry.LevelCritical,
			Timestamp: time.Now().Format("15:04:05"),
		}
		state.Incidents = append([]telemetry.Incident{inc}, state.Incidents...)
		broadcastChaos("CPU Spike Injected", fmt.Sprintf("Operator triggered severe core throttling on %s", node.Name), "chaos-cpu", telemetry.LevelCritical)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/chaos/ram-leak", func(w http.ResponseWriter, r *http.Request) {
		node := state.GetSelectedNode()
		node.Metrics.RAMUsedGB = node.Metrics.RAMTotalGB * 0.94
		node.Metrics.RAMPercent = 94.0
		node.Metrics.Status = "WARNING"
		inc := telemetry.Incident{
			ID:        fmt.Sprintf("inc-%d", time.Now().UnixNano()%100000),
			NodeID:    node.ID,
			Title:     "RAM Exhaustion Leak (>90%)",
			Severity:  telemetry.LevelWarn,
			Timestamp: time.Now().Format("15:04:05"),
		}
		state.Incidents = append([]telemetry.Incident{inc}, state.Incidents...)
		broadcastChaos("RAM Leak Injected", fmt.Sprintf("Memory buffer runaway detected on %s", node.Name), "chaos-mem", telemetry.LevelWarn)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/chaos/network-drop", func(w http.ResponseWriter, r *http.Request) {
		node := state.GetSelectedNode()
		node.Metrics.NetworkRateMB = 0.4
		node.Metrics.Status = "WARNING"
		broadcastChaos("Network Flap", fmt.Sprintf("Packet loss and link degradation on %s interface eth0", node.ID), "chaos-net", telemetry.LevelWarn)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/chaos/heal", func(w http.ResponseWriter, r *http.Request) {
		node := state.GetSelectedNode()
		node.Metrics.CPUPercent = 22.0
		node.Metrics.RAMUsedGB = 4.8
		node.Metrics.RAMPercent = 30.0
		node.Metrics.NetworkRateMB = 82.5
		node.Metrics.Status = "HEALTHY"
		// Clear incidents
		for i := range state.Incidents {
			state.Incidents[i].Resolved = true
		}
		state.TotalAlerts = 0

		nowStr := time.Now().Format("15:04:05")
		healLog := telemetry.EventLog{
			ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     telemetry.LevelInfo,
			Service:   "self-heal",
			Message:   "Automated remediation completed. All cluster metrics restored to healthy nominals.",
		}
		state.RecentLogs = append([]telemetry.EventLog{healLog}, state.RecentLogs...)

		allIncidents := state.GetActiveIncidents()

		var teleBuf bytes.Buffer
		teleBuf.WriteString(render(components.MetricsGrid(node, false)))
		teleBuf.WriteString(render(components.HeaderStatusBadge("HEALTHY", true)))
		teleBuf.WriteString(render(components.HeaderAlertCounter(0, true)))
		sseBroker.BroadcastEvent("telemetry", teleBuf.String())

		var logBuf bytes.Buffer
		logBuf.WriteString(render(components.LogRow(healLog)))
		logBuf.WriteString(render(components.IncidentsPanel(allIncidents, true)))
		sseBroker.BroadcastEvent("log-entry", logBuf.String())

		w.WriteHeader(http.StatusNoContent)
	})

	// Legacy spike endpoint support
	mux.HandleFunc("POST /api/simulate/spike", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/chaos/cpu-spike", http.StatusTemporaryRedirect)
	})

	port := ":8080"
	fmt.Printf("\n======================================================\n")
	fmt.Printf(" ⚡ PulseOps Enterprise Running on http://localhost%s\n", port)
	fmt.Printf(" Features: Multi-Cluster, Live SVG Sparkline, Incidents, Chaos\n")
	fmt.Printf(" Stack: Go 1.25 + Templ + HTMX 2.0 (SSE + OOB Swaps)\n")
	fmt.Printf("======================================================\n\n")

	if err := http.ListenAndServe(port, mux); err != nil {
		if !strings.Contains(err.Error(), "Server closed") {
			log.Fatalf("Server error: %v", err)
		}
	}
}
