package main

import (
	"fmt"
	"image"
	"runtime"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

type batchCropResult struct {
	ok  bool
	err error
}

func startBatchCrop(
	w fyne.Window,
	progressUpdater func(float64),
	finished func(okCount, total int, lastErr error),
	filesToProcess []string,
	outDir string,
	cropRect image.Rectangle,
) {
	go func() {
		total := len(filesToProcess)
		okCount := 0
		var lastErr error
		if total == 0 {
			fyne.Do(func() {
				if finished != nil {
					finished(0, 0, nil)
					return
				}
				dialog.ShowInformation("เสร็จสิ้น", "ไม่มีไฟล์ให้ประมวลผล", w)
			})
			return
		}

		workerCount := runtime.GOMAXPROCS(0)
		if workerCount > total {
			workerCount = total
		}
		if workerCount < 1 {
			workerCount = 1
		}

		jobs := make(chan string)
		results := make(chan batchCropResult, total)

		var wg sync.WaitGroup
		wg.Add(workerCount)
		for i := 0; i < workerCount; i++ {
			go func() {
				defer wg.Done()
				for path := range jobs {
					err := cropAndSave(path, outDir, cropRect)
					results <- batchCropResult{
						ok:  err == nil,
						err: err,
					}
				}
			}()
		}

		go func() {
			for _, path := range filesToProcess {
				jobs <- path
			}
			close(jobs)
			wg.Wait()
			close(results)
		}()

		doneCount := 0
		for res := range results {
			doneCount++
			if res.ok {
				okCount++
			} else {
				lastErr = res.err
			}

			if progressUpdater != nil {
				pct := float64(doneCount) / float64(total)
				fyne.Do(func() {
					progressUpdater(pct)
				})
			}
		}

		if finished != nil {
			fyne.Do(func() {
				finished(okCount, total, lastErr)
			})
			return
		}

		fyne.Do(func() {
			if lastErr != nil {
				dialog.ShowInformation("เสร็จสิ้น (มีข้อผิดพลาดบางไฟล์)",
					fmt.Sprintf("ครอปสำเร็จ %d/%d ไฟล์\nข้อผิดพลาดล่าสุด: %v", okCount, total, lastErr), w)
			} else {
				dialog.ShowInformation("เสร็จสิ้น", fmt.Sprintf("ครอปสำเร็จทั้งหมด %d ไฟล์ ✅", okCount), w)
			}
		})
	}()
}
