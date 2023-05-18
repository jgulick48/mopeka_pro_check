package mopeka_pro_check

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_DefaultBudgetAllocation(t *testing.T) {
	filled := CalculatePercentOfCircle(13, 18.5)
	assert.Equal(t, 31.356090324437847, filled)
}
func Test_DefaultBudgetAllocatio_MoreThanHalf(t *testing.T) {
	filled := CalculatePercentOfCircle(29, 18.5)
	assert.Equal(t, 84.08596089292737, filled)
}
