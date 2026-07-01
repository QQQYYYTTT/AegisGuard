package audit

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store 审计日志存储
// 使用 JSON Lines 格式（每行一个 JSON 对象），增量追加写入
// 非并发安全：外部需持有 Store 引用，Append 内部通过 Mutex 保护
type Store struct {
	mu   sync.Mutex
	file string
	f    *os.File // 保持打开的文件句柄，避免每次 Append 都 open/close
}

// NewStore 创建审计存储
// 如果文件不存在，自动创建空文件
// 以追加模式打开文件句柄
func NewStore(file string) (*Store, error) {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create audit dir %s: %w", dir, err)
	}

	// O_APPEND | O_CREAT | O_WRONLY：追加写入，不存在则创建
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file %s: %w", file, err)
	}

	return &Store{file: file, f: f}, nil
}

// Append 向审计文件追加一条 JSON Lines 记录
// 线程安全
func (s *Store) Append(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// 每行一个 JSON 对象 + 换行符
	line = append(line, '\n')

	if _, err := s.f.Write(line); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// ReadAll 读取所有审计事件（按时间降序，最新的在最前面）
// 从文件按行读取，每行解析为一个 AuditEvent
func (s *Store) ReadAll() ([]AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 刷新写入缓冲区，确保读取内容是完整的
	if err := s.f.Sync(); err != nil {
		return nil, fmt.Errorf("failed to sync audit file: %w", err)
	}

	raw, err := os.ReadFile(s.file)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit file: %w", err)
	}

	if len(raw) == 0 {
		return []AuditEvent{}, nil
	}

	var events []AuditEvent
	lines := splitLines(raw)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev AuditEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			// 单行解析失败不中止，跳过损坏行
			continue
		}
		events = append(events, ev)
	}

	// 按时间降序（最新的在前）
	sortEventsByTimestampDesc(events)

	return events, nil
}

// QuerySince 返回指定时间之后的所有审计事件
func (s *Store) QuerySince(since time.Time) ([]AuditEvent, error) {
	all, err := s.ReadAll()
	if err != nil {
		return nil, err
	}

	filtered := make([]AuditEvent, 0, len(all))
	for _, ev := range all {
		if !ev.Timestamp.Before(since) {
			filtered = append(filtered, ev)
		}
	}
	return filtered, nil
}

// AggregateThreatSources 从 JSONL 中聚合高危/阻断事件的源 IP（jsonl 兜底实现）
func (s *Store) AggregateThreatSources(since time.Time) ([]ThreatSourceRow, error) {
	all, err := s.ReadAll()
	if err != nil {
		return nil, err
	}
	var rows []ThreatSourceRow
	for _, ev := range all {
		if ev.Timestamp.Before(since) {
			continue
		}
		if !isThreatSourceEvent(ev) {
			continue
		}
		rows = append(rows, ThreatSourceRow{
			ClientIP:  ev.ClientIP,
			RiskLevel: ev.RiskLevel,
			Decision:  ev.Decision,
			Timestamp: ev.Timestamp,
		})
	}
	return rows, nil
}

// Close 关闭审计文件句柄
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}

// --- 辅助函数 ---

// splitLines 将字节切片按换行符分割
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	// 最后一行（可能没有换行符结尾）
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// sortEventsByTimestampDesc 按时间戳降序排序
func sortEventsByTimestampDesc(events []AuditEvent) {
	sort.Slice(events, func(i, j int) bool {
		return events[j].Timestamp.Before(events[i].Timestamp)
	})
}

func isThreatSourceEvent(ev AuditEvent) bool {
	level := strings.ToLower(strings.TrimSpace(ev.RiskLevel))
	if level != "high" && level != "critical" {
		return false
	}
	decision := strings.ToLower(strings.TrimSpace(ev.Decision))
	if decision != "block" && decision != "deny" {
		return false
	}
	return ev.ClientIP != "" && !isPrivateIP(ev.ClientIP)
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return ip.IsPrivate() || ip.IsLoopback()
}
