package telemetry

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelInfo     LogLevel = "INFO"
	LevelWarn     LogLevel = "WARN"
	LevelCritical LogLevel = "CRIT"
)

type SystemMetrics struct {
	CPUPercent     float64
	CPUHistory     []float64 // Untuk render SVG Sparkline real-time di server
	RAMUsedGB      float64
	RAMTotalGB     float64
	RAMPercent     float64
	NetworkRateMB  float64
	ActiveTasks    int
	Status         string // "HEALTHY", "WARNING", "CRITICAL"
	UpdatedAt      time.Time
}

type ClusterNode struct {
	ID        string
	Name      string
	Region    string
	Role      string
	Metrics   SystemMetrics
	IsActive  bool
}

type Incident struct {
	ID           string
	NodeID       string
	Title        string
	Severity     LogLevel
	Timestamp    string
	Acknowledged bool
	Resolved     bool
}

type EventLog struct {
	ID        string
	Timestamp string
	Level     LogLevel
	Service   string
	Message   string
	IsSpike   bool
}

type AlertNotification struct {
	ID        string
	Timestamp string
	Level     LogLevel
	Title     string
	Message   string
}

type TelemetryState struct {
	mu           sync.RWMutex
	SelectedNode string
	Nodes        map[string]*ClusterNode
	RecentLogs   []EventLog
	Incidents    []Incident
	TotalAlerts  int
	LastAlert    *AlertNotification
}

func NewTelemetryState() *TelemetryState {
	initHistory := func(base float64) []float64 {
		pts := make([]float64, 16)
		for i := range pts {
			pts[i] = base + (rand.Float64()-0.5)*10.0
			if pts[i] < 5 {
				pts[i] = 5
			}
		}
		return pts
	}

	nodes := map[string]*ClusterNode{
		"srv-sg-core-01": {
			ID:     "srv-sg-core-01",
			Name:   "Singapore Core Hub",
			Region: "ap-southeast-1",
			Role:   "API Gateway & Orchestrator",
			Metrics: SystemMetrics{
				CPUPercent:    28.4,
				CPUHistory:    initHistory(28.0),
				RAMUsedGB:     5.2,
				RAMTotalGB:    16.0,
				RAMPercent:    32.5,
				NetworkRateMB: 84.5,
				ActiveTasks:   142,
				Status:        "HEALTHY",
				UpdatedAt:     time.Now(),
			},
			IsActive: true,
		},
		"srv-us-edge-02": {
			ID:     "srv-us-edge-02",
			Name:   "US-East Edge Shard",
			Region: "us-east-1",
			Role:   "Ingress Proxy & CDN Cache",
			Metrics: SystemMetrics{
				CPUPercent:    42.1,
				CPUHistory:    initHistory(42.0),
				RAMUsedGB:     11.4,
				RAMTotalGB:    32.0,
				RAMPercent:    35.6,
				NetworkRateMB: 210.8,
				ActiveTasks:   310,
				Status:        "HEALTHY",
				UpdatedAt:     time.Now(),
			},
			IsActive: false,
		},
		"srv-eu-db-03": {
			ID:     "srv-eu-db-03",
			Name:   "Frankfurt Storage Core",
			Region: "eu-central-1",
			Role:   "Distributed TiKV Database",
			Metrics: SystemMetrics{
				CPUPercent:    58.7,
				CPUHistory:    initHistory(58.0),
				RAMUsedGB:     24.8,
				RAMTotalGB:    32.0,
				RAMPercent:    77.5,
				NetworkRateMB: 145.2,
				ActiveTasks:   188,
				Status:        "WARNING",
				UpdatedAt:     time.Now(),
			},
			IsActive: false,
		},
	}

	return &TelemetryState{
		SelectedNode: "srv-sg-core-01",
		Nodes:        nodes,
		RecentLogs:   make([]EventLog, 0),
		Incidents: []Incident{
			{
				ID:           "inc-101",
				NodeID:       "srv-eu-db-03",
				Title:        "Memory buffer allocation exceeded 75%",
				Severity:     LevelWarn,
				Timestamp:    time.Now().Add(-4 * time.Minute).Format("15:04:05"),
				Acknowledged: false,
				Resolved:     false,
			},
		},
		TotalAlerts: 1,
	}
}

func (s *TelemetryState) GetSelectedNode() *ClusterNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n, ok := s.Nodes[s.SelectedNode]; ok {
		return n
	}
	return s.Nodes["srv-sg-core-01"]
}

func (s *TelemetryState) SetSelectedNode(nodeID string) *ClusterNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, n := range s.Nodes {
		n.IsActive = (id == nodeID)
	}
	s.SelectedNode = nodeID
	return s.Nodes[nodeID]
}

func (s *TelemetryState) GetNodesList() []*ClusterNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order := []string{"srv-sg-core-01", "srv-us-edge-02", "srv-eu-db-03"}
	var list []*ClusterNode
	for _, id := range order {
		if n, ok := s.Nodes[id]; ok {
			list = append(list, n)
		}
	}
	return list
}

func (s *TelemetryState) GetActiveIncidents() []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []Incident
	for _, inc := range s.Incidents {
		if !inc.Resolved {
			active = append(active, inc)
		}
	}
	return active
}

func (s *TelemetryState) ResolveIncident(id string) (*Incident, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var resolved *Incident
	activeCount := 0
	for i := range s.Incidents {
		if s.Incidents[i].ID == id {
			s.Incidents[i].Resolved = true
			resolved = &s.Incidents[i]
		}
		if !s.Incidents[i].Resolved {
			activeCount++
		}
	}
	s.TotalAlerts = activeCount
	return resolved, activeCount
}

func (s *TelemetryState) AcknowledgeIncident(id string) *Incident {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Incidents {
		if s.Incidents[i].ID == id {
			s.Incidents[i].Acknowledged = true
			return &s.Incidents[i]
		}
	}
	return nil
}

func (s *TelemetryState) FilterLogs(query string, level string) []EventLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []EventLog
	for _, log := range s.RecentLogs {
		matchLevel := (level == "" || level == "ALL" || string(log.Level) == level)
		matchQuery := true
		if query != "" {
			matchQuery = fmt.Sprintf("%s %s %s", log.Service, log.Message, log.Timestamp) != ""
		}
		if matchLevel && matchQuery {
			result = append(result, log)
		}
	}
	return result
}

// Tick memajukan status semua cluster nodes
func (s *TelemetryState) Tick() (*ClusterNode, *EventLog, *AlertNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newAlert *AlertNotification
	var newLog *EventLog
	nowStr := time.Now().Format("15:04:05")

	for _, node := range s.Nodes {
		// Fluctuasi CPU
		delta := (rand.Float64() - 0.49) * 8.0
		node.Metrics.CPUPercent += delta
		if node.Metrics.CPUPercent < 8.0 {
			node.Metrics.CPUPercent = 8.0
		} else if node.Metrics.CPUPercent > 99.0 {
			node.Metrics.CPUPercent = 99.0
		}

		// Update CPU History Buffer (16 points untuk sparkline)
		if len(node.Metrics.CPUHistory) >= 16 {
			node.Metrics.CPUHistory = append(node.Metrics.CPUHistory[1:], node.Metrics.CPUPercent)
		} else {
			node.Metrics.CPUHistory = append(node.Metrics.CPUHistory, node.Metrics.CPUPercent)
		}

		// Fluctuasi RAM
		deltaRAM := (rand.Float64() - 0.5) * 0.2
		node.Metrics.RAMUsedGB += deltaRAM
		if node.Metrics.RAMUsedGB < 2.0 {
			node.Metrics.RAMUsedGB = 2.0
		} else if node.Metrics.RAMUsedGB > node.Metrics.RAMTotalGB {
			node.Metrics.RAMUsedGB = node.Metrics.RAMTotalGB - 0.5
		}
		node.Metrics.RAMPercent = (node.Metrics.RAMUsedGB / node.Metrics.RAMTotalGB) * 100.0

		// Status
		if node.Metrics.CPUPercent > 85.0 {
			node.Metrics.Status = "CRITICAL"
		} else if node.Metrics.CPUPercent > 65.0 || node.Metrics.RAMPercent > 75.0 {
			node.Metrics.Status = "WARNING"
		} else {
			node.Metrics.Status = "HEALTHY"
		}
		node.Metrics.UpdatedAt = time.Now()
	}

	activeNode := s.Nodes[s.SelectedNode]

	// Peluang acak membuat log atau incident
	if rand.Float64() < 0.25 && activeNode.Metrics.CPUPercent > 82.0 {
		incID := fmt.Sprintf("inc-%d", time.Now().UnixNano()%100000)
		inc := Incident{
			ID:        incID,
			NodeID:    activeNode.ID,
			Title:     fmt.Sprintf("High Thermal Load Throttling (%.1f%%)", activeNode.Metrics.CPUPercent),
			Severity:  LevelCritical,
			Timestamp: nowStr,
		}
		s.Incidents = append([]Incident{inc}, s.Incidents...)
		s.TotalAlerts++

		newAlert = &AlertNotification{
			ID:        incID,
			Timestamp: nowStr,
			Level:     LevelCritical,
			Title:     "Cluster Anomaly Detected",
			Message:   fmt.Sprintf("%s experiencing sustained CPU load > 85%%", activeNode.Name),
		}
		s.LastAlert = newAlert

		newLog = &EventLog{
			ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     LevelCritical,
			Service:   "thermal-guard",
			Message:   fmt.Sprintf("Kernel frequency governor engaged on %s", activeNode.ID),
			IsSpike:   true,
		}
	} else {
		// Log normal
		services := []string{"gateway-envoy", "auth-vault", "etcd-sync", "redis-session"}
		srv := services[rand.Intn(len(services))]
		msgs := []string{
			"Keep-alive TCP handshake completed",
			"Consensus raft leader heartbeat ACK",
			"Prometheus metrics scrape latency: 2.1ms",
			"Ingress TLS 1.3 session ticket refreshed",
		}
		newLog = &EventLog{
			ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
			Timestamp: nowStr,
			Level:     LevelInfo,
			Service:   srv,
			Message:   fmt.Sprintf("[%s] %s", activeNode.ID, msgs[rand.Intn(len(msgs))]),
		}
	}

	if newLog != nil {
		s.RecentLogs = append([]EventLog{*newLog}, s.RecentLogs...)
		if len(s.RecentLogs) > 40 {
			s.RecentLogs = s.RecentLogs[:40]
		}
	}

	return activeNode, newLog, newAlert
}
