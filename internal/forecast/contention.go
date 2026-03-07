package forecast

import (
	"sort"
	"time"
)

// ContentionWindow represents a period where concurrent tasks exceed worker capacity.
type ContentionWindow struct {
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	PeakConcurrent int      `json:"peak_concurrent"`
	WorkerCount    int       `json:"worker_count"`
	Overflow       int       `json:"overflow"`
	Tasks          []string  `json:"tasks"`
}

type event struct {
	time     time.Time
	taskName string
	isStart  bool
}

// DetectContention finds time windows where concurrent tasks exceed workerCount.
func DetectContention(summary ForecastSummary, workerCount int) []ContentionWindow {
	if workerCount <= 0 {
		workerCount = 1
	}

	// Build events from forecast runs
	var events []event
	for _, r := range summary.Runs {
		duration := r.EstimatedDuration
		if duration <= 0 {
			duration = 1 * time.Minute // assume at least 1 minute for contention purposes
		}

		events = append(events, event{
			time:     r.ScheduledTime,
			taskName: r.TaskName,
			isStart:  true,
		})
		events = append(events, event{
			time:     r.ScheduledTime.Add(duration),
			taskName: r.TaskName,
			isStart:  false,
		})
	}

	// Sort by time, with end events before start events at the same time
	sort.Slice(events, func(i, j int) bool {
		if events[i].time.Equal(events[j].time) {
			return !events[i].isStart && events[j].isStart
		}
		return events[i].time.Before(events[j].time)
	})

	// Sweep line to find contention windows
	var windows []ContentionWindow
	activeTasks := make(map[string]int) // task name -> count
	concurrent := 0

	var currentWindow *ContentionWindow

	for _, ev := range events {
		if ev.isStart {
			concurrent++
			activeTasks[ev.taskName]++
		} else {
			concurrent--
			activeTasks[ev.taskName]--
			if activeTasks[ev.taskName] <= 0 {
				delete(activeTasks, ev.taskName)
			}
		}

		if concurrent > workerCount {
			if currentWindow == nil {
				taskNames := make([]string, 0, len(activeTasks))
				for name := range activeTasks {
					taskNames = append(taskNames, name)
				}
				sort.Strings(taskNames)
				currentWindow = &ContentionWindow{
					Start:          ev.time,
					PeakConcurrent: concurrent,
					WorkerCount:    workerCount,
					Tasks:          taskNames,
				}
			} else {
				if concurrent > currentWindow.PeakConcurrent {
					currentWindow.PeakConcurrent = concurrent
					taskNames := make([]string, 0, len(activeTasks))
					for name := range activeTasks {
						taskNames = append(taskNames, name)
					}
					sort.Strings(taskNames)
					currentWindow.Tasks = taskNames
				}
			}
		} else if currentWindow != nil {
			currentWindow.End = ev.time
			currentWindow.Overflow = currentWindow.PeakConcurrent - workerCount
			windows = append(windows, *currentWindow)
			currentWindow = nil
		}
	}

	// Close any open window
	if currentWindow != nil {
		currentWindow.End = summary.EndTime
		currentWindow.Overflow = currentWindow.PeakConcurrent - workerCount
		windows = append(windows, *currentWindow)
	}

	return windows
}
