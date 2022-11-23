package upgrade

import (
	"sort"
	"sync"
	"time"
)

type UpgradeRolloutTracker struct {
	mu            sync.Mutex
	rolloutWindow time.Duration
	plannedTimes  []time.Time
}

func NewUpgradeRolloutTracker(rolloutWindow time.Duration) *UpgradeRolloutTracker {
	return &UpgradeRolloutTracker{
		rolloutWindow: rolloutWindow,
	}
}

func (t *UpgradeRolloutTracker) PlanUpgrade(currentTime time.Time) time.Time {
	if t.rolloutWindow == time.Duration(0) {
		return currentTime
	}

	t.cleanupPlannedTimes(currentTime)
	nextUpgradeTime := t.findNextUpgradeTime(currentTime)
	t.AddUpgradeTime(nextUpgradeTime)
	return nextUpgradeTime
}

func (t *UpgradeRolloutTracker) AddUpgradeTime(upgradeTime time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.plannedTimes = append(t.plannedTimes, upgradeTime)
}

func (t *UpgradeRolloutTracker) findNextUpgradeTime(currentTime time.Time) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.plannedTimes) == 0 {
		return currentTime
	}

	var upgradeTimes = append([]time.Time{currentTime}, t.plannedTimes...)
	upgradeTimes = append(upgradeTimes, currentTime.Add(t.rolloutWindow))

	var biggestGap time.Duration
	var previousTime time.Time
	for i := 0; i < len(upgradeTimes)-1; i++ {
		timeBetweenUpgrades := upgradeTimes[i+1].Sub(upgradeTimes[i])
		if timeBetweenUpgrades > biggestGap {
			biggestGap = timeBetweenUpgrades
			previousTime = upgradeTimes[i]
		}
	}
	return previousTime.Add(biggestGap / 2)
}

func (t *UpgradeRolloutTracker) cleanupPlannedTimes(currentTime time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sort.Slice(t.plannedTimes, func(i, j int) bool {
		return t.plannedTimes[i].Before(t.plannedTimes[j])
	})

	for _, upgradeTime := range t.plannedTimes {
		if upgradeTime.Before(currentTime) {
			t.plannedTimes = t.plannedTimes[1:]
		} else {
			break
		}
	}
}
