package rooms

import "time"

type Room struct {
	ID          int64
	Number      int
	Floor       int
	Status      Status
	Observation string
	GuestCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var OfficialNumbers = func() []int {
	result := make([]int, 0, 42)
	for n := 101; n <= 116; n++ {
		result = append(result, n)
	}
	for n := 201; n <= 226; n++ {
		result = append(result, n)
	}
	return result
}()
