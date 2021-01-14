package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func Test_appState(t *testing.T) {
	tests := []struct {
		name string
		pods []apiv1.Pod
		want string
	}{
		{
			name: "app without pods",
			pods: nil,
			want: "created",
		},
		{
			name: "1 pod for each state",
			pods: []apiv1.Pod{
				apiv1.Pod{
					Status: apiv1.PodStatus{
						Phase: apiv1.PodPending,
					},
				},
				apiv1.Pod{
					Status: apiv1.PodStatus{
						Phase: apiv1.PodRunning,
					},
				},
				apiv1.Pod{
					Status: apiv1.PodStatus{
						Phase: apiv1.PodSucceeded,
					},
				},
				apiv1.Pod{
					Status: apiv1.PodStatus{
						Phase: apiv1.PodFailed,
					},
				},
				apiv1.Pod{
					Status: apiv1.PodStatus{
						Phase: apiv1.PodUnknown,
					},
				},
			},
			want: "1 deploying, 1 running, 2 error, 1 succeeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := appState(tt.pods)
			require.Equal(t, tt.want, state)
		})
	}
}

func Test_podState(t *testing.T) {
	tests := []struct {
		name string
		pod  apiv1.Pod
		want ketchv1.PodState
	}{
		{
			name: "pod without state",
			pod:  apiv1.Pod{},
			want: ketchv1.PodError,
		},
		{
			name: "pod pending",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodPending,
				},
			},
			want: ketchv1.PodDeploying,
		},
		{
			name: "pod running",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodRunning,
				},
			},
			want: ketchv1.PodRunning,
		},
		{
			name: "pod running, but container in CrashLoopBackOff",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodRunning,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							State: apiv1.ContainerState{
								Waiting: &apiv1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			want: ketchv1.PodError,
		},
		{
			name: "pod running, but container in Completed",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodRunning,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							State: apiv1.ContainerState{
								Terminated: &apiv1.ContainerStateTerminated{
									Reason: "Completed",
								},
							},
						},
					},
				},
			},
			want: ketchv1.PodSucceeded,
		},
		{
			name: "pod succeeded",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodSucceeded,
				},
			},
			want: ketchv1.PodSucceeded,
		},
		{
			name: "pod failed",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
				},
			},
			want: ketchv1.PodError,
		},
		{
			name: "pod unknown",
			pod: apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodUnknown,
				},
			},
			want: ketchv1.PodError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := podState(tt.pod)
			require.Equal(t, tt.want, state)
		})
	}
}
