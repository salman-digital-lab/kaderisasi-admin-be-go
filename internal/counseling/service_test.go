package counseling

import "testing"

func TestEligibleCounselor(t *testing.T) {
	tests := []struct {
		role   string
		active bool
		want   bool
	}{
		{role: "super_admin", active: true, want: true},
		{role: "admin", active: true, want: true},
		{role: "konselor", active: true, want: true},
		{role: "member_manager", active: true, want: false},
		{role: "konselor", active: false, want: false},
	}

	for _, test := range tests {
		t.Run(test.role, func(t *testing.T) {
			if got := eligibleCounselor(&test.role, test.active); got != test.want {
				t.Fatalf("eligibleCounselor(%q, %t) = %t, want %t", test.role, test.active, got, test.want)
			}
		})
	}
}
