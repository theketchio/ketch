package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCondition(t *testing.T) {
	tests := []struct {
		description   string
		jobStatus     *JobStatus
		conditionType ConditionType
		expected      *Condition
	}{
		{
			description:   "found",
			jobStatus:     &JobStatus{Conditions: []Condition{{Type: "type1", Status: "active"}, {Type: "type2", Status: "failed"}}},
			conditionType: ConditionType("type1"),
			expected:      &Condition{Type: "type1", Status: "active"},
		},
		{
			description:   "not found",
			jobStatus:     &JobStatus{Conditions: []Condition{{Type: "type1", Status: "active"}, {Type: "type2", Status: "failed"}}},
			conditionType: ConditionType("complete"),
			expected:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			res := tt.jobStatus.Condition(tt.conditionType)
			require.Equal(t, tt.expected, res)
		})
	}
}

func TestSetCondition(t *testing.T) {
	now := metav1.Time{}
	j := &Job{
		Status: JobStatus{
			Conditions: []Condition{
				{
					Type:    ConditionType("type1"),
					Status:  "active",
					Message: "message-1",
				},
			},
		},
	}
	expected := &Job{
		Status: JobStatus{
			Conditions: []Condition{
				{
					Type:    ConditionType("type1"),
					Status:  "active",
					Message: "message-1",
				},
				{
					Type:               ConditionType("type2"),
					Status:             "failed",
					Message:            "message-2",
					LastTransitionTime: &now,
				},
			},
		},
	}

	j.SetCondition("type2", v1.ConditionStatus("failed"), "message-2", now)
	require.Equal(t, expected, j)
}
