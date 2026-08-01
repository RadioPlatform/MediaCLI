package upload

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"radioplatform-media-ci/pkg/api"
)

type UploadResult struct {
	Item    UploadItem
	Success bool
	Error   string
	Media   *api.MediaItem
}

type UploadSummary struct {
	StationUUID   string
	StationName   string
	Items         []UploadResult
	TotalBytes    int64
	UploadedBytes int64
	FailedCount   int
	SuccessCount  int
}

type Executor struct {
	client       *api.Client
	concurrency  int
	showProgress bool
}

func NewExecutor(client *api.Client, concurrency int, showProgress bool) *Executor {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 20 {
		concurrency = 20
	}
	return &Executor{
		client:       client,
		concurrency:  concurrency,
		showProgress: showProgress,
	}
}

func (e *Executor) Execute(ctx context.Context, plan *UploadPlan) *UploadSummary {
	summary := &UploadSummary{
		StationUUID: plan.StationUUID,
		StationName: plan.StationName,
		TotalBytes:  plan.TotalBytes,
	}

	items := make([]UploadItem, len(plan.Items))
	copy(items, plan.Items)

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, e.concurrency)
		results = make([]UploadResult, 0, len(items))
	)

	if e.showProgress {
		e.executeWithProgress(ctx, items, plan, &results, &mu, &wg, sem)
	} else {
		e.executeSimple(ctx, items, plan, &results, &mu, &wg, sem)
	}

	wg.Wait()

	summary.Items = results

	sort.Slice(summary.Items, func(i, j int) bool {
		return summary.Items[i].Item.LocalPath < summary.Items[j].Item.LocalPath
	})

	tallyResults(summary, results)

	return summary
}

func tallyResults(summary *UploadSummary, results []UploadResult) {
	for _, r := range results {
		if r.Success {
			summary.SuccessCount++
			summary.UploadedBytes += r.Item.Size
		} else {
			summary.FailedCount++
		}
	}
}

func (e *Executor) executeSimple(ctx context.Context, items []UploadItem, plan *UploadPlan, results *[]UploadResult, mu *sync.Mutex, wg *sync.WaitGroup, sem chan struct{}) {
	for _, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(item UploadItem) {
			defer wg.Done()
			defer func() { <-sem }()

			result := e.uploadItem(ctx, item, plan, nil)
			mu.Lock()
			*results = append(*results, result)
			mu.Unlock()
		}(item)
	}
}

func (e *Executor) executeWithProgress(ctx context.Context, items []UploadItem, plan *UploadPlan, results *[]UploadResult, mu *sync.Mutex, wg *sync.WaitGroup, sem chan struct{}) {
	p := mpb.New(mpb.WithWaitGroup(wg), mpb.WithWidth(90))

	for _, item := range items {
		bar := p.AddBar(item.Size,
			mpb.PrependDecorators(
				decor.Name(progressName(item.DestinationName)+" ", decor.WCSyncWidth),
				decor.CountersKibiByte("% .1f / % .1f", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(
				decor.OnComplete(
					decor.NewPercentage("%.0f%%", decor.WCSyncWidth),
					" done",
				),
				decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncWidth),
			),
		)
		wg.Add(1)
		sem <- struct{}{}
		go func(item UploadItem, bar *mpb.Bar) {
			defer wg.Done()
			defer func() { <-sem }()

			result := e.uploadItem(ctx, item, plan, func(progress api.UploadProgress) {
				bar.SetCurrent(progress.BytesReceived)
			})
			if result.Success {
				bar.SetCurrent(item.Size)
			} else {
				bar.Abort(false)
			}

			mu.Lock()
			*results = append(*results, result)
			mu.Unlock()
		}(item, bar)
	}

	p.Wait()
}

func (e *Executor) uploadItem(ctx context.Context, item UploadItem, plan *UploadPlan, onProgress api.UploadProgressFunc) UploadResult {
	input := api.UploadMediaInput{
		FilePath: item.LocalPath,
		Folder:   item.DestinationFolder,
		IsJingle: item.IsJingle,
	}

	result, err := e.client.UploadMediaChunked(ctx, plan.StationUUID, input, onProgress)
	if err != nil {
		return UploadResult{
			Item:    item,
			Success: false,
			Error:   err.Error(),
		}
	}

	if !result.Success {
		return UploadResult{
			Item:    item,
			Success: false,
			Error:   result.Error,
		}
	}

	return UploadResult{
		Item:    item,
		Success: true,
		Media:   result.Media,
	}
}

func progressName(filename string) string {
	const maximumRunes = 28
	runes := []rune(filename)
	if len(runes) <= maximumRunes {
		return filename
	}
	return string(runes[:maximumRunes-1]) + "…"
}

type UploadError struct {
	Item UploadItem
	Err  string
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("upload failed for %s: %s", e.Item.LocalPath, e.Err)
}

func FormatDestinationFolder(folder string) string {
	if folder == "" {
		return "Media root"
	}
	return folder
}

func FolderBreakdown(items []UploadItem) map[string]int {
	breakdown := make(map[string]int)
	for _, item := range items {
		folder := item.DestinationFolder
		if folder == "" {
			folder = "Media root"
		}
		breakdown[folder]++
	}
	return breakdown
}

func FormatFolderBreakdown(items []UploadItem) string {
	breakdown := FolderBreakdown(items)
	var parts []string
	for _, folder := range sortedKeys(breakdown) {
		parts = append(parts, fmt.Sprintf("  %-12s %d files", folder, breakdown[folder]))
	}
	return strings.Join(parts, "\n")
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
