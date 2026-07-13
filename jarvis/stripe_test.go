package main

import "testing"

func TestPriceForPlan(t *testing.T) {
	t.Setenv("STRIPE_PRICE_STARTER_FOUNDER", "price_starter")
	t.Setenv("STRIPE_PRICE_PRO_FOUNDER", "price_pro")
	t.Setenv("STRIPE_PRICE_AGENCY_FOUNDER", "price_agency")

	tests := []struct {
		plan string
		want string
	}{
		{plan: "starter", want: "price_starter"},
		{plan: "pro", want: "price_pro"},
		{plan: "agency", want: "price_agency"},
		{plan: "unknown", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			if got := priceForPlan(tt.plan); got != tt.want {
				t.Errorf("priceForPlan(%q) = %q; want %q", tt.plan, got, tt.want)
			}
		})
	}
}
