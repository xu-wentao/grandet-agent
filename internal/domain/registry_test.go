package domain

import "testing"

func TestModelExecutionProfileEligibleForAutomaticRouting(t *testing.T) {
	for _, test := range []struct {
		name      string
		profile   ModelExecutionProfile
		allowPaid bool
		want      bool
	}{
		{name: "active free", profile: ModelExecutionProfile{Enabled: true, LifecycleState: "ACTIVE", IsFree: true}, want: true},
		{name: "active priced", profile: ModelExecutionProfile{Enabled: true, LifecycleState: "ACTIVE", PriceKnown: true}, want: true},
		{name: "active unknown paid allowed", profile: ModelExecutionProfile{Enabled: true, LifecycleState: "ACTIVE"}, allowPaid: true, want: true},
		{name: "discovered", profile: ModelExecutionProfile{Enabled: true, LifecycleState: "DISCOVERED", IsFree: true}},
		{name: "degraded", profile: ModelExecutionProfile{Enabled: true, LifecycleState: "DEGRADED", IsFree: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.profile.EligibleForAutomaticRouting(test.allowPaid); got != test.want {
				t.Fatalf("EligibleForAutomaticRouting() = %t, want %t", got, test.want)
			}
		})
	}
}
