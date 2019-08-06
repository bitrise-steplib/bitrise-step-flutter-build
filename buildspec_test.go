package main

import (
	"reflect"
	"testing"
)

func Test_filterAndroidArtifactsBy(t *testing.T) {
	tests := []struct {
		name              string
		androidOutputType AndroidArtifactType
		artifacts         []string
		want              []string
	}{
		{
			name:              "Filter APK",
			androidOutputType: APK,
			artifacts: []string{
				"test.apk",
				"test_2.apk",
				"test.aab",
			},
			want: []string{
				"test.apk",
				"test_2.apk",
			},
		},
		{
			name:              "Filter AAB",
			androidOutputType: AppBundle,
			artifacts: []string{
				"test.apk",
				"test_2.apk",
				"test.aab",
				"test_2.aab",
			},
			want: []string{
				"test.aab",
				"test_2.aab",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterAndroidArtifactsBy(tt.androidOutputType, tt.artifacts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterAndroidArtifactsBy() = %v, want %v", got, tt.want)
			}
		})
	}
}
