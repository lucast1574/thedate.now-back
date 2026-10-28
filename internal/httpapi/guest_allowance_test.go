package httpapi

import "testing"

func TestGuestAllowsOnlyTitularAndOptionalPlusOne(t *testing.T) {
	for _, seats := range []int{0, 1, 2, 3, 20} {
		in := guestInput{Name: "Ana", Phone: "+51987654321", Seats: seats}
		if validGuest(in) != (seats == 1 || seats == 2) {
			t.Fatalf("unexpected allowance for %d seats", seats)
		}
	}
}
