package upgrade_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/internal/upgrade"
	"time"
)

var _ = Describe("Tracker", func() {
	var startTime = time.Now()
	DescribeTable("picking a time in the middle of the largest gap in the rollout window", func(startTime, expectedTime time.Time, existingTimes []time.Time) {
		tracker := upgrade.NewUpgradeRolloutTracker(10 * time.Minute)
		for _, existingTime := range existingTimes {
			tracker.AddUpgradeTime(existingTime)
		}
		Expect(tracker.PlanUpgrade(startTime)).To(Equal(expectedTime))
	},
		Entry("there are no upgrades planned in the window", startTime, startTime, []time.Time{}),
		Entry("an upgrade is already planned", startTime, startTime.Add(5*time.Minute), []time.Time{startTime}),
		Entry("there are several equal gaps", startTime, startTime.Add(1*time.Minute), []time.Time{
			startTime,
			startTime.Add(2 * time.Minute),
			startTime.Add(4 * time.Minute),
			startTime.Add(6 * time.Minute),
			startTime.Add(8 * time.Minute),
		}),
		Entry("there are several unequal gaps", startTime, startTime.Add(6*time.Minute+30*time.Second), []time.Time{
			startTime,
			startTime.Add(3 * time.Minute),
			startTime.Add(4 * time.Minute),
			startTime.Add(9 * time.Minute),
		}),
		Entry("there are some times before the current time", startTime, startTime.Add(2*time.Minute+30*time.Second), []time.Time{
			startTime.Add(-30 * time.Minute),
			startTime.Add(-20 * time.Minute),
			startTime,
			startTime.Add(5 * time.Minute),
			startTime.Add(7 * time.Minute),
		}),
		Entry("there are only times before the current time", startTime, startTime, []time.Time{
			startTime.Add(-30 * time.Minute),
			startTime.Add(-20 * time.Minute),
		}),
	)
})
