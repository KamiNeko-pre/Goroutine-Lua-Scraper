package worker

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Stream:        "crawler:tasks",
				Group:         "crawler-workers",
				Consumer:      "worker-1",
				ReadBlock:     time.Second,
				ReadCount:     1,
				ClaimMinIdle:  time.Minute,
				ClaimInterval: 10 * time.Second,
			},
		},
		{
			name: "missing stream",
			config: Config{
				Group:         "crawler-workers",
				Consumer:      "worker-1",
				ReadBlock:     time.Second,
				ReadCount:     1,
				ClaimMinIdle:  time.Minute,
				ClaimInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing group",
			config: Config{
				Stream:        "crawler:tasks",
				Consumer:      "worker-1",
				ReadBlock:     time.Second,
				ReadCount:     1,
				ClaimMinIdle:  time.Minute,
				ClaimInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing consumer",
			config: Config{
				Stream:        "crawler:tasks",
				Group:         "crawler-workers",
				ReadBlock:     time.Second,
				ReadCount:     1,
				ClaimMinIdle:  time.Minute,
				ClaimInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "non-positive read count",
			config: Config{
				Stream:        "crawler:tasks",
				Group:         "crawler-workers",
				Consumer:      "worker-1",
				ReadBlock:     time.Second,
				ClaimMinIdle:  time.Minute,
				ClaimInterval: 10 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
