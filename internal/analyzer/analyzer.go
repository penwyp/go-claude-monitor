package analyzer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/penwyp/go-claude-monitor/internal/data/aggregator"
	"github.com/penwyp/go-claude-monitor/internal/data/cache"
	"github.com/penwyp/go-claude-monitor/internal/data/parser"
	"github.com/penwyp/go-claude-monitor/internal/data/scanner"
	"github.com/penwyp/go-claude-monitor/internal/presentation/formatter"
	"github.com/penwyp/go-claude-monitor/internal/util"
)

type Config struct {
	DataDir      string
	CacheDir     string
	OutputFormat string
	Timezone     string
	Duration     string
	GroupBy      string
	Limit        int
	Breakdown    bool
	Concurrency  int
	// Pricing configuration
	PricingSource      string // default, litellm
	PricingOfflineMode bool   // Enable offline pricing mode
}

type Analyzer struct {
	config     *Config
	cache      cache.Cache
	scanner    *scanner.FileScanner
	parser     *parser.Parser
	aggregator *aggregator.Aggregator
}

// extractSessionId extracts the session ID from a file path.
// For example: "/path/to/00aec530-0614-436f-a53b-faaa0b32f123.jsonl" -> "00aec530-0614-436f-a53b-faaa0b32f123"
func extractSessionId(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func New(config *Config) *Analyzer {
	if config.Concurrency == 0 {
		config.Concurrency = runtime.NumCPU()
	}

	fileCache, _ := cache.NewFileCache(config.CacheDir)

	// Create aggregator with pricing configuration
	agg, err := aggregator.NewAggregatorWithConfig(
		config.PricingSource,
		config.PricingOfflineMode,
		config.CacheDir,
		config.Timezone,
	)
	if err != nil {
		util.LogError("Failed to create aggregator with pricing config: " + err.Error())
		// Fallback to default aggregator
		agg = aggregator.NewAggregatorWithTimezone(config.Timezone)
	}

	return &Analyzer{
		config:     config,
		cache:      fileCache,
		scanner:    scanner.NewFileScanner(config.DataDir),
		parser:     parser.NewParser(config.Concurrency),
		aggregator: agg,
	}
}

func (a *Analyzer) Run() error {
	startTime := time.Now()
	util.LogInfo("Starting analysis of Claude usage...")

	// Phase 1: Preload cache into memory
	preloadStart := time.Now()
	if err := a.cache.Preload(); err != nil {
		util.LogWarn(fmt.Sprintf("Cache preload failed: %v", err))
	}
	preloadDuration := time.Since(preloadStart)
	util.LogDebug(fmt.Sprintf("Phase 1 - Cache preload duration: %v", preloadDuration))

	// Phase 2: Scan files
	scanStart := time.Now()
	files, err := a.scanner.Scan()
	if err != nil {
		return fmt.Errorf("Failed to scan files: %w", err)
	}
	scanDuration := time.Since(scanStart)
	util.LogDebug(fmt.Sprintf("Phase 2 - File scan duration: %v, found %d files", scanDuration, len(files)))

	if len(files) == 0 {
		return fmt.Errorf("No JSONL files found")
	}

	util.LogInfo(fmt.Sprintf("Found %d JSONL files", len(files)))

	// Phase 3: Batch validate cache and process files
	parseStart := time.Now()
	stats := NewCacheStats()
	var allHourlyData []aggregator.HourlyData

	// Create session ID mapping
	sessionIdMap := make(map[string]string, len(files))
	sessionIds := make([]string, 0, len(files))
	for _, file := range files {
		sessionId := extractSessionId(file)
		sessionIdMap[file] = sessionId
		sessionIds = append(sessionIds, sessionId)
	}

	// Batch validate cache
	batchStart := time.Now()
	validCache := a.cache.BatchValidate(sessionIds)
	batchDuration := time.Since(batchStart)
	util.LogDebug(fmt.Sprintf("Batch cache validation duration: %v", batchDuration))

	// Separate files to parse and files to use from cache
	var filesToParse []string
	var cachedFiles []string
	// Store miss reasons for files that need parsing
	fileMissReasons := make(map[string]cache.CacheMissReason)

	for _, file := range files {
		sessionId := sessionIdMap[file]
		validateResult := validCache[sessionId]
		if validateResult.Valid {
			cachedFiles = append(cachedFiles, file)
		} else {
			filesToParse = append(filesToParse, file)
			fileMissReasons[file] = validateResult.MissReason
		}
	}

	util.LogDebug(fmt.Sprintf("Cache hit for %d files, need to parse %d files",
		len(cachedFiles), len(filesToParse)))

	// Process files with cache hits
	cacheStart := time.Now()
	for _, file := range cachedFiles {
		sessionId := sessionIdMap[file]
		cacheResult := a.cache.Get(sessionId)
		if cacheResult.Found && cacheResult.Data != nil {
			stats.IncrementHit()
			allHourlyData = append(allHourlyData, cacheResult.Data.HourlyStats...)
		}
		stats.IncrementTotal()
	}
	cacheProcessDuration := time.Since(cacheStart)
	util.LogDebug(fmt.Sprintf("Cache data processing duration: %v", cacheProcessDuration))

	// Concurrently parse files that need processing
	if len(filesToParse) > 0 {
		parseFileStart := time.Now()
		parseResults := a.parser.ParseFiles(filesToParse)

		processed := int64(len(cachedFiles)) // Number of cache files already processed
		cacheMisses := int64(0)

		for result := range parseResults {
			stats.IncrementTotal()
			processed++

			if result.Error != nil {
				stats.IncrementFailure()
				util.LogWarn(fmt.Sprintf("Failed to parse file %s: %v", result.File, result.Error))
				continue
			}

			sessionId := sessionIdMap[result.File]
			// Use the actual miss reason from cache validation
			missReason := fileMissReasons[result.File]
			stats.IncrementMiss(result.File, missReason)
			cacheMisses++

			projectName := aggregator.ExtractProjectName(result.File)
			hourlyData := a.aggregator.AggregateByHourAndModel(result.Logs, projectName)

			aggregatedData := &aggregator.AggregatedData{
				FileHash:    sessionId, // Using sessionId for now, will rename field later
				FilePath:    result.File,
				ProjectName: projectName,
				HourlyStats: hourlyData,
				SessionId:   sessionId,
			}

			if err := a.cache.Set(sessionId, aggregatedData); err != nil {
				util.LogWarn(fmt.Sprintf("Failed to save cache for %s: %v", result.File, err))
			}

			allHourlyData = append(allHourlyData, hourlyData...)

			if processed%100 == 0 {
				stats.PrintProgress(processed)
				stats.PrintPeriodicStats()
			}
		}

		parseFilesDuration := time.Since(parseFileStart)
		util.LogDebug(fmt.Sprintf("File parsing duration: %v", parseFilesDuration))
	}

	parseDuration := time.Since(parseStart)
	util.LogDebug(fmt.Sprintf("Phase 3 - File parsing and processing duration: %v, total records: %d", parseDuration, len(allHourlyData)))

	// Print final cache statistics
	stats.PrintPeriodicStats()
	stats.PrintFinalStats()

	if len(allHourlyData) == 0 {
		return fmt.Errorf("No valid API usage data found")
	}

	// Phase 4: Filter by date range
	filterStart := time.Now()
	filteredData := a.filterByDateRange(allHourlyData)
	filterDuration := time.Since(filterStart)
	util.LogDebug(fmt.Sprintf("Phase 4 - Date filtering duration: %v, records after filtering: %d", filterDuration, len(filteredData)))

	// Phase 5: Group data
	groupStart := time.Now()
	groupedData := a.groupData(filteredData)
	groupDuration := time.Since(groupStart)
	util.LogDebug(fmt.Sprintf("Phase 5 - Data grouping duration: %v, number of groups: %d", groupDuration, len(groupedData)))

	// Phase 6: Sort data
	sortStart := time.Now()
	sortedData := a.sortData(groupedData)
	sortDuration := time.Since(sortStart)
	util.LogDebug(fmt.Sprintf("Phase 6 - Data sorting duration: %v", sortDuration))

	if a.config.Limit > 0 && len(sortedData) > a.config.Limit {
		util.LogDebug(fmt.Sprintf("Applying result limit: %d -> %d", len(sortedData), a.config.Limit))
		sortedData = sortedData[:a.config.Limit]
	}

	// Phase 7: Format and output
	outputStart := time.Now()
	err = a.formatAndOutput(sortedData)
	outputDuration := time.Since(outputStart)
	util.LogDebug(fmt.Sprintf("Phase 7 - Formatting and output duration: %v", outputDuration))

	totalDuration := time.Since(startTime)
	util.LogDebug(fmt.Sprintf("Total duration: %v (preload:%v scan:%v parse:%v filter:%v group:%v sort:%v output:%v)",
		totalDuration, preloadDuration, scanDuration, parseDuration,
		filterDuration, groupDuration, sortDuration, outputDuration))

	return err
}

func (a *Analyzer) filterByDateRange(data []aggregator.HourlyData) []aggregator.HourlyData {
	if a.config.Duration == "" {
		return data
	}

	loc, _ := time.LoadLocation(a.config.Timezone)
	fromTime, err := parseDuration(a.config.Duration, loc)
	if err != nil {
		util.LogError(fmt.Sprintf("Failed to parse duration: %v", err))
		return data
	}

	// toTime is the current time in the specified timezone
	toTime := time.Now().In(loc)

	var filtered []aggregator.HourlyData
	for _, item := range data {
		itemTime := time.Unix(item.Hour, 0).In(loc)
		if !itemTime.Before(fromTime) && !itemTime.After(toTime) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func (a *Analyzer) groupData(data []aggregator.HourlyData) []formatter.GroupedData {
	groupMap := make(map[string]*formatter.GroupedData)
	modelDetailsMap := make(map[string]map[string]*formatter.ModelDetail)

	for _, item := range data {
		// Calculate cost in real-time instead of using cached cost
		cost, err := a.aggregator.CalculateCost(&item)
		if err != nil {
			util.LogWarn(fmt.Sprintf("Failed to calculate cost for model %s: %v", item.Model, err))
			cost = 0 // Use 0 as the default value if calculation fails
		}

		groupKey := a.getGroupKey(item)

		if _, ok := groupMap[groupKey]; !ok {
			groupMap[groupKey] = &formatter.GroupedData{
				Date:          groupKey,
				ShowBreakdown: a.config.Breakdown || a.config.OutputFormat == "summary",
			}
			modelDetailsMap[groupKey] = make(map[string]*formatter.ModelDetail)
		}

		group := groupMap[groupKey]
		group.InputTokens += item.InputTokens
		group.OutputTokens += item.OutputTokens
		group.CacheCreation += item.CacheCreation
		group.CacheRead += item.CacheRead
		group.TotalTokens += item.TotalTokens
		group.Cost += cost // Use real-time calculated cost

		if !contains(group.Models, item.Model) {
			group.Models = append(group.Models, item.Model)
		}

		if a.config.Breakdown || a.config.OutputFormat == "summary" {
			if _, ok := modelDetailsMap[groupKey][item.Model]; !ok {
				modelDetailsMap[groupKey][item.Model] = &formatter.ModelDetail{
					Model: item.Model,
				}
			}
			detail := modelDetailsMap[groupKey][item.Model]
			detail.InputTokens += item.InputTokens
			detail.OutputTokens += item.OutputTokens
			detail.CacheCreation += item.CacheCreation
			detail.CacheRead += item.CacheRead
			detail.TotalTokens += item.TotalTokens
			detail.Cost += cost // Use real-time calculated cost
		}
	}

	var result []formatter.GroupedData
	for key, group := range groupMap {
		// Sort models by specified order
		group.Models = util.SortModels(group.Models)

		if a.config.Breakdown || a.config.OutputFormat == "summary" {
			for _, detail := range modelDetailsMap[key] {
				group.ModelDetails = append(group.ModelDetails, *detail)
			}
			sort.Slice(group.ModelDetails, func(i, j int) bool {
				return util.GetModelOrder(group.ModelDetails[i].Model) < util.GetModelOrder(group.ModelDetails[j].Model)
			})
		}

		result = append(result, *group)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result
}

func (a *Analyzer) getGroupKey(item aggregator.HourlyData) string {
	switch a.config.GroupBy {
	case "model":
		return item.Model
	case "project":
		return item.ProjectName
	case "hour":
		return time.Unix(item.Hour, 0).Format("2006-01-02 15:00")
	case "week":
		t := time.Unix(item.Hour, 0)
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "month":
		return time.Unix(item.Hour, 0).Format("2006-01")
	default:
		return time.Unix(item.Hour, 0).Format("2006-01-02")
	}
}

func (a *Analyzer) sortData(data []formatter.GroupedData) []formatter.GroupedData {
	sort.Slice(data, func(i, j int) bool {
		return data[i].Date < data[j].Date
	})
	return data
}

func (a *Analyzer) formatAndOutput(data []formatter.GroupedData) error {
	switch a.config.OutputFormat {
	case "json":
		return formatter.NewJSONFormatter().Format(data)
	case "csv":
		return formatter.NewCSVFormatter().Format(data)
	case "summary":
		return formatter.NewSummaryFormatter().Format(data)
	default:
		return formatter.NewTableFormatter().Format(data)
	}
}

func parseDuration(durationStr string, loc *time.Location) (time.Time, error) {
	if durationStr == "" {
		return time.Time{}, nil
	}

	now := time.Now().In(loc)

	// Regular expression to match duration components
	re := regexp.MustCompile(`(\d+)([hymwd])`)
	matches := re.FindAllStringSubmatch(durationStr, -1)

	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	var totalDuration time.Duration

	for _, match := range matches {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number in duration: %s", match[1])
		}

		unit := match[2]
		switch unit {
		case "h":
			totalDuration += time.Duration(value) * time.Hour
		case "d":
			totalDuration += time.Duration(value) * 24 * time.Hour
		case "w":
			totalDuration += time.Duration(value) * 7 * 24 * time.Hour
		case "m":
			// For months, we approximate as 30 days
			totalDuration += time.Duration(value) * 30 * 24 * time.Hour
		case "y":
			// For years, we approximate as 365 days
			totalDuration += time.Duration(value) * 365 * 24 * time.Hour
		default:
			return time.Time{}, fmt.Errorf("unsupported time unit: %s", unit)
		}
	}

	return now.Add(-totalDuration), nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
