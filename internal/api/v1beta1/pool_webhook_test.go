package v1beta1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/internal/api/v1beta1/mocks"
)

func TestPool_ValidateDelete(t *testing.T) {
	tests := []struct {
		name    string
		pool    Pool
		wantErr error
	}{
		{
			name: "pool with running apps",
			pool: Pool{
				Status: PoolStatus{
					Apps: []string{"ketch"},
				},
			},
			wantErr: ErrDeletePoolWithRunningApps,
		},
		{
			name: "pool with no apps",
			pool: Pool{
				Status: PoolStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pool.ValidateDelete(); err != tt.wantErr {
				t.Errorf("ValidateDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type mockManager struct {
	client *mocks.MockClient
}

func (m *mockManager) GetClient() client.Client {
	return m.client
}

func TestPool_ValidateCreate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name    string
		pool    Pool
		client  *mocks.MockClient
		wantErr error
	}{
		{
			name: "error getting a list of pools",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			wantErr: listError,
		},
		{
			name: "namespace is used",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					pools := list.(*PoolList)
					pools.Items = []Pool{
						{Spec: PoolSpec{NamespaceName: "ketch-namespace"}},
						{Spec: PoolSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			pool: Pool{
				Spec: PoolSpec{
					NamespaceName: "ketch-namespace",
				},
			},
			wantErr: ErrNamespaceIsUsedByAnotherPool,
		},
		{
			name: "namespace is used",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					pools := list.(*PoolList)
					pools.Items = []Pool{
						{Spec: PoolSpec{NamespaceName: "ketch-namespace"}},
					}
					return nil
				},
			},
			pool: Pool{
				Spec: PoolSpec{
					NamespaceName: "theketch-namespace",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolmgr = &mockManager{client: tt.client}
			if err := tt.pool.ValidateCreate(); err != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPool_ValidateUpdate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name    string
		pool    Pool
		old     runtime.Object
		client  *mocks.MockClient
		wantErr error
	}{
		{
			name: "error getting a list of pools",
			pool: Pool{
				Spec: PoolSpec{NamespaceName: "ketch-namespace"},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			old: &Pool{
				Spec: PoolSpec{NamespaceName: "theketch-namespace"},
			},
			wantErr: listError,
		},
		{
			name: "namespace is used",
			pool: Pool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool-1"},
				Spec:       PoolSpec{NamespaceName: "ketch-namespace"},
				Status:     PoolStatus{},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					pools := list.(*PoolList)
					pools.Items = []Pool{
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-2"}, Spec: PoolSpec{NamespaceName: "ketch-namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-1"}, Spec: PoolSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			old: &Pool{
				Spec: PoolSpec{NamespaceName: "theketch-namespace"},
			},
			wantErr: ErrNamespaceIsUsedByAnotherPool,
		},
		{
			name: "everything is ok",
			pool: Pool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool-1"},
				Spec:       PoolSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: 2},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					pools := list.(*PoolList)
					pools.Items = []Pool{
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-2"}, Spec: PoolSpec{NamespaceName: "namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-1"}, Spec: PoolSpec{NamespaceName: "theketch-namespace"}},
					}
					return nil
				},
			},
			old: &Pool{
				Spec: PoolSpec{NamespaceName: "theketch-namespace"},
			},
		},
		{
			name: "failed to descrease quota",
			pool: Pool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool-1"},
				Spec:       PoolSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: 1},
				Status: PoolStatus{
					Apps: []string{"app-1", "app-2"},
				},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					pools := list.(*PoolList)
					pools.Items = []Pool{
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-2"}, Spec: PoolSpec{NamespaceName: "theketch-namespace"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "pool-1"}, Spec: PoolSpec{NamespaceName: "ketch-namespace"}},
					}
					return nil
				},
			},
			old: &Pool{
				Spec: PoolSpec{NamespaceName: "ketch-namespace", AppQuotaLimit: 5},
				Status: PoolStatus{
					Apps: []string{"app-1", "app-2"},
				},
			},
			wantErr: ErrDecreaseQuota,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolmgr = &mockManager{client: tt.client}
			if err := tt.pool.ValidateUpdate(tt.old); err != tt.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
