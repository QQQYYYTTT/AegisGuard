package audit

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// ThreatTarget 大屏地图中央汇聚点（客户网关所在地）
type ThreatTarget struct {
	Name  string     `json:"name"`
	Coord [2]float64 `json:"coord"`
}

// ThreatMapStats 威胁源统计
type ThreatMapStats struct {
	Total     int `json:"total"`
	Critical  int `json:"critical"`
	High      int `json:"high"`
	Sources   int `json:"sources"`
	Provinces int `json:"provinces"`
}

// ThreatMapProvince 省份聚合
type ThreatMapProvince struct {
	Name     string `json:"name"`
	Value    int    `json:"value"`
	Critical int    `json:"critical"`
}

// ThreatMapCity 城市坐标点
type ThreatMapCity struct {
	Name  string     `json:"name"`
	Coord [2]float64 `json:"coord"`
	Value int        `json:"value"`
	Level string     `json:"level"`
}

// ThreatMapLine 源城市到网关的攻击路径
type ThreatMapLine struct {
	From   [2]float64 `json:"from"`
	To     [2]float64 `json:"to"`
	Count  int        `json:"count"`
	Level  string     `json:"level"`
	Latest string     `json:"latest"`
}

// ThreatMapData 威胁地图全量数据
type ThreatMapData struct {
	Target      ThreatTarget        `json:"target"`
	Stats       ThreatMapStats      `json:"stats"`
	Provinces   []ThreatMapProvince `json:"provinces"`
	Cities      []ThreatMapCity     `json:"cities"`
	Lines       []ThreatMapLine     `json:"lines"`
	GeneratedAt string              `json:"generatedAt"`
}

// ThreatSourceRow 聚合查询原始行
type ThreatSourceRow struct {
	ClientIP  string
	RiskLevel string
	Decision  string
	Timestamp time.Time
}

// ProvinceCenter 省份中心坐标（ECharts 中国地图使用）
var ProvinceCenter = map[string][2]float64{
	"北京":  {116.407, 39.904},
	"天津":  {117.201, 39.084},
	"上海":  {121.473, 31.230},
	"重庆":  {106.551, 29.563},
	"河北":  {114.514, 38.042},
	"山西":  {112.548, 37.870},
	"辽宁":  {123.428, 41.796},
	"吉林":  {125.323, 43.812},
	"黑龙江": {126.661, 45.742},
	"江苏":  {118.762, 32.061},
	"浙江":  {120.152, 30.267},
	"安徽":  {117.285, 31.861},
	"福建":  {119.295, 26.099},
	"江西":  {115.857, 28.675},
	"山东":  {117.020, 36.668},
	"河南":  {113.625, 34.746},
	"湖北":  {114.342, 30.546},
	"湖南":  {112.982, 28.116},
	"广东":  {113.264, 23.129},
	"海南":  {110.348, 20.019},
	"四川":  {104.066, 30.657},
	"贵州":  {106.630, 26.647},
	"云南":  {102.710, 25.045},
	"陕西":  {108.939, 34.341},
	"甘肃":  {103.826, 36.059},
	"青海":  {101.778, 36.620},
	"台湾":  {121.509, 25.044},
	"内蒙古": {111.765, 40.817},
	"广西":  {108.327, 22.815},
	"西藏":  {91.117, 29.646},
	"宁夏":  {106.278, 38.466},
	"新疆":  {87.627, 43.793},
	"香港":  {114.171, 22.319},
	"澳门":  {113.549, 22.198},
}

// CityProvince 城市 -> 省份映射（部分主要城市）
var CityProvince = map[string]string{
	"北京": "北京", "上海": "上海", "天津": "天津", "重庆": "重庆",
	"广州": "广东", "深圳": "广东", "佛山": "广东", "东莞": "广东",
	"杭州": "浙江", "宁波": "浙江", "温州": "浙江",
	"南京": "江苏", "苏州": "江苏", "无锡": "江苏",
	"成都": "四川", "武汉": "湖北", "西安": "陕西",
	"郑州": "河南", "长沙": "湖南", "青岛": "山东", "济南": "山东",
	"沈阳": "辽宁", "大连": "辽宁", "哈尔滨": "黑龙江", "长春": "吉林",
	"石家庄": "河北", "太原": "山西", "福州": "福建", "厦门": "福建",
	"合肥": "安徽", "南昌": "江西", "昆明": "云南", "贵阳": "贵州",
	"南宁": "广西", "海口": "海南", "兰州": "甘肃", "银川": "宁夏",
	"西宁": "青海", "乌鲁木齐": "新疆", "拉萨": "西藏",
	"呼和浩特": "内蒙古", "包头": "内蒙古",
	"香港": "香港", "澳门": "澳门", "台北": "台湾",
}

// IPLocator 把 IP 解析为地理位置的接口（实现可替换为 ip2region / MaxMind）
type IPLocator interface {
	Locate(ip string) (province, city string, ok bool)
}

// StaticLocator 简易静态定位器：只识别示例 IP，用于无 ip2region 时的演示
type StaticLocator struct {
	Rules []StaticRule
}

type StaticRule struct {
	Prefix   string
	Province string
	City     string
}

func NewStaticLocator() *StaticLocator {
	return &StaticLocator{
		Rules: []StaticRule{
			{Prefix: "1.202", Province: "北京", City: "北京"},
			{Prefix: "58.246", Province: "北京", City: "北京"},
			{Prefix: "116.236", Province: "北京", City: "北京"},
			{Prefix: "124.202", Province: "上海", City: "上海"},
			{Prefix: "180.173", Province: "上海", City: "上海"},
			{Prefix: "58.32", Province: "浙江", City: "杭州"},
			{Prefix: "60.191", Province: "浙江", City: "杭州"},
			{Prefix: "119.147", Province: "广东", City: "深圳"},
			{Prefix: "121.35", Province: "广东", City: "深圳"},
			{Prefix: "113.105", Province: "广东", City: "广州"},
			{Prefix: "14.215", Province: "广东", City: "广州"},
			{Prefix: "61.160", Province: "江苏", City: "南京"},
			{Prefix: "180.96", Province: "江苏", City: "南京"},
			{Prefix: "222.20", Province: "湖北", City: "武汉"},
			{Prefix: "61.128", Province: "四川", City: "成都"},
			{Prefix: "117.22", Province: "陕西", City: "西安"},
		},
	}
}

func (l *StaticLocator) Locate(ip string) (string, string, bool) {
	if net.ParseIP(ip) == nil {
		return "", "", false
	}
	for _, rule := range l.Rules {
		if strings.HasPrefix(ip, rule.Prefix) {
			return rule.Province, rule.City, true
		}
	}
	return "", "", false
}

// Ip2RegionLocator 基于 ip2region xdb 离线库的 IP 定位器
type Ip2RegionLocator struct {
	searcher *xdb.Searcher
}

func NewIp2RegionLocator(dbPath string) (*Ip2RegionLocator, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("ip2region xdb not found at %s: %w", dbPath, err)
	}
	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, dbPath)
	if err != nil {
		return nil, fmt.Errorf("load ip2region xdb failed: %w", err)
	}
	return &Ip2RegionLocator{searcher: searcher}, nil
}

func (l *Ip2RegionLocator) Locate(ip string) (string, string, bool) {
	if net.ParseIP(ip) == nil {
		return "", "", false
	}
	region, err := l.searcher.Search(ip)
	if err != nil || region == "" || region == "0|0|0|0|0" {
		return "", "", false
	}
	parts := strings.Split(region, "|")
	if len(parts) < 5 {
		return "", "", false
	}
	province := normalizeRegion(parts[2])
	city := normalizeRegion(parts[3])
	if city == "" || city == "0" {
		city = province
	}
	if province == "" || province == "0" {
		return "", "", false
	}
	return province, city, true
}

func (l *Ip2RegionLocator) Close() {
	if l.searcher != nil {
		l.searcher.Close()
	}
}

func normalizeRegion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "中国")
	s = strings.TrimSuffix(s, "市")
	s = strings.TrimSuffix(s, "省")
	s = strings.TrimSuffix(s, "自治区")
	s = strings.TrimSuffix(s, "回族自治区")
	s = strings.TrimSuffix(s, "维吾尔自治区")
	s = strings.TrimSuffix(s, "壮族自治区")
	s = strings.TrimSuffix(s, "特别行政区")
	return strings.TrimSpace(s)
}

// NewLocator 根据 xdb 文件存在与否自动选择真实库或静态兜底
type LocatorOptions struct {
	XDBPath string
}

func NewLocator(opts LocatorOptions) (IPLocator, error) {
	if opts.XDBPath != "" {
		if locator, err := NewIp2RegionLocator(opts.XDBPath); err == nil {
			return locator, nil
		}
	}
	return NewStaticLocator(), nil
}

// DetectServerLocation 根据本机第一个公网网卡 IP 推断部署位置；失败则返回广州
type ServerLocationDetector struct {
	locator IPLocator
}

func NewServerLocationDetector(locator IPLocator) *ServerLocationDetector {
	return &ServerLocationDetector{locator: locator}
}

func (d *ServerLocationDetector) Detect() ThreatTarget {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return defaultThreatTarget()
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsPrivate() {
			continue
		}
		ip := ipNet.IP.String()
		province, city, ok := d.locator.Locate(ip)
		if ok {
			coord := ProvinceCenter[city]
			if coord[0] == 0 && coord[1] == 0 {
				coord = ProvinceCenter[province]
			}
			return ThreatTarget{Name: city, Coord: coord}
		}
	}
	return defaultThreatTarget()
}

func defaultThreatTarget() ThreatTarget {
	return ThreatTarget{Name: "广州", Coord: [2]float64{113.264, 23.129}}
}

// ThreatMapBuilder 聚合审计事件为地图数据，带 30s 缓存
type ThreatMapBuilder struct {
	store    Storer
	locator  IPLocator
	target   ThreatTarget
	cacheMu  sync.RWMutex
	cache    *ThreatMapData
	cacheExp time.Time
	cacheTTL time.Duration
}

func NewThreatMapBuilder(store Storer, locator IPLocator, target ThreatTarget) *ThreatMapBuilder {
	if target.Name == "" {
		target = defaultThreatTarget()
	}
	return &ThreatMapBuilder{
		store:    store,
		locator:  locator,
		target:   target,
		cacheTTL: 30 * time.Second,
	}
}

func (b *ThreatMapBuilder) Target() ThreatTarget {
	return b.target
}

func (b *ThreatMapBuilder) Build(_ context.Context, window time.Duration) (*ThreatMapData, error) {
	b.cacheMu.RLock()
	if b.cache != nil && time.Now().Before(b.cacheExp) {
		cached := b.cache
		b.cacheMu.RUnlock()
		return cached, nil
	}
	b.cacheMu.RUnlock()

	since := time.Now().Add(-window)
	rows, err := b.store.AggregateThreatSources(since)
	if err != nil {
		return nil, err
	}

	data := b.aggregate(rows)

	b.cacheMu.Lock()
	b.cache = data
	b.cacheExp = time.Now().Add(b.cacheTTL)
	b.cacheMu.Unlock()
	return data, nil
}

func (b *ThreatMapBuilder) aggregate(rows []ThreatSourceRow) *ThreatMapData {
	provinceAgg := make(map[string]*ThreatMapProvince)
	cityAgg := make(map[string]*ThreatMapCity)
	lineAgg := make(map[string]*ThreatMapLine)
	sources := make(map[string]struct{})
	stats := ThreatMapStats{}

	for _, r := range rows {
		level := strings.ToLower(strings.TrimSpace(r.RiskLevel))
		if level != "high" && level != "critical" {
			continue
		}
		decision := strings.ToLower(strings.TrimSpace(r.Decision))
		if decision != "block" && decision != "deny" {
			continue
		}

		stats.Total++
		if level == "critical" {
			stats.Critical++
		} else {
			stats.High++
		}
		sources[r.ClientIP] = struct{}{}

		province, city, ok := b.locator.Locate(r.ClientIP)
		if !ok || province == "" {
			continue
		}

		p := provinceAgg[province]
		if p == nil {
			p = &ThreatMapProvince{Name: province}
			provinceAgg[province] = p
		}
		p.Value++
		if level == "critical" {
			p.Critical++
		}

		coord := ProvinceCenter[city]
		if coord[0] == 0 && coord[1] == 0 {
			coord = ProvinceCenter[province]
		}

		c := cityAgg[city]
		if c == nil {
			c = &ThreatMapCity{Name: city, Coord: coord, Value: 0, Level: level}
			cityAgg[city] = c
		}
		c.Value++
		if level == "critical" && c.Level != "critical" {
			c.Level = "critical"
		}

		lineKey := city
		l := lineAgg[lineKey]
		if l == nil {
			l = &ThreatMapLine{From: coord, To: b.target.Coord, Level: level}
			lineAgg[lineKey] = l
		}
		l.Count++
		latest := r.Timestamp.Format(time.RFC3339)
		if latest > l.Latest {
			l.Latest = latest
		}
		if level == "critical" && l.Level != "critical" {
			l.Level = "critical"
		}
	}

	provinces := make([]ThreatMapProvince, 0, len(provinceAgg))
	for _, p := range provinceAgg {
		provinces = append(provinces, *p)
	}
	sort.Slice(provinces, func(i, j int) bool { return provinces[i].Value > provinces[j].Value })

	cities := make([]ThreatMapCity, 0, len(cityAgg))
	for _, c := range cityAgg {
		cities = append(cities, *c)
	}
	sort.Slice(cities, func(i, j int) bool { return cities[i].Value > cities[j].Value })
	if len(cities) > 30 {
		cities = cities[:30]
	}

	lines := make([]ThreatMapLine, 0, len(lineAgg))
	for _, l := range lineAgg {
		lines = append(lines, *l)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Count > lines[j].Count })
	if len(lines) > 50 {
		lines = lines[:50]
	}

	stats.Sources = len(sources)
	stats.Provinces = len(provinceAgg)

	return &ThreatMapData{
		Target:      b.target,
		Stats:       stats,
		Provinces:   provinces,
		Cities:      cities,
		Lines:       lines,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}
