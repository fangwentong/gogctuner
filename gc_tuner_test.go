package gogctuner

import (
	"math"
	"testing"
)

// func getGOGC(previousGOGC int , memoryLimitInPercent, memPercent float64) int {
type GetGOGCTestCase struct {
	MemoryLimitInPercent float64
	TotalSize            uint64
	LiveSize             float64
	ExpectedGOGC         int
}

func TestGetGOGCBasics(t *testing.T) {
	cases := []GetGOGCTestCase{
		{
			MemoryLimitInPercent: 40,
			TotalSize:            10000,
			LiveSize:             6000,
			ExpectedGOGC:         int(math.Max(58, minGOGCValue)),
		},
		{
			MemoryLimitInPercent: 40,
			TotalSize:            10000,
			LiveSize:             9000,
			ExpectedGOGC:         minGOGCValue,
		},
		{
			MemoryLimitInPercent: 80,
			TotalSize:            10000,
			LiveSize:             1000,
			ExpectedGOGC:         700,
		},
		{
			MemoryLimitInPercent: 80,
			TotalSize:            10000,
			LiveSize:             3000,
			ExpectedGOGC:         166,
		},
	}
	for i, _ := range cases {
		result := getGOGC(cases[i].MemoryLimitInPercent, cases[i].TotalSize, cases[i].LiveSize)
		if result != cases[i].ExpectedGOGC {
			t.Errorf("Failed Test Case #%v - Expected: %v Found: %v", i+1, cases[i].ExpectedGOGC, result)
		}
	}
}

func TestGcConfig(t *testing.T) {
	testGcConfigCheck(t, 0, true)
	testGcConfigCheck(t, 90, true)
	testGcConfigCheck(t, 100, true)
	testGcConfigCheck(t, 101, false)
	testGcConfigCheck(t, -1, false)
}

func testGcConfigCheck(t *testing.T, maxRamPercentage float64, expectValid bool) {
	config := Config{MaxRAMPercentage: maxRamPercentage}
	expect := "invalid"
	if expectValid {
		expect = "valid"
	}
	if config.CheckValid() == nil != expectValid {
		t.Errorf("%f for MaxRAMPercentage should be %s", maxRamPercentage, expect)
	}
}
