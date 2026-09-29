package file

import (
	"context"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/stretchr/testify/require"
	"github.com/tencentyun/cos-go-sdk-v5"
)

func TestConfiguredProviderCareerObjectKeysAreStableAndTenantScoped(t *testing.T) {
	name := "career_source_request-1.pdf"
	require.True(t, isCareerStableName(name))
	require.Equal(t, "career_source_request-1.pdf", careerObjectName(name))
	// Provider key layouts preserve their configured prefixes and tenant scope.
	for _, prefix := range []string{"root", "root/"} {
		firstAttempt := careerExportObjectKey(prefix, 7, name)
		retry := careerExportObjectKey(prefix, 7, name)
		require.Equal(t, "root/7/exports/"+name, firstAttempt)
		require.Equal(t, firstAttempt, retry)
	}
	require.Equal(t, "7/exports/"+name, careerExportObjectKey("", 7, name))
	require.False(t, isCareerStableName("resume.pdf"))
}

func TestCloudSaveBytesAdaptersSelectStableCareerKeys(t *testing.T) {
	tests := []struct {
		name string
		save func(hook fileBytesPutHook) func(fileName string, temp bool) (string, error)
	}{
		{
			name: "COS",
			save: func(hook fileBytesPutHook) func(string, bool) (string, error) {
				svc := &cosFileService{bucketName: "main", region: "region", cosPathPrefix: "root/", tempClient: new(cos.Client), tempBucketURL: "https://temp.example/", putBytesHook: hook}
				return func(name string, temp bool) (string, error) {
					return svc.SaveBytes(context.Background(), []byte("body"), 7, name, temp)
				}
			},
		},
		{
			name: "TOS",
			save: func(hook fileBytesPutHook) func(string, bool) (string, error) {
				svc := &tosFileService{bucketName: "main", tempBucketName: "temporary", pathPrefix: "root/", putBytesHook: hook}
				return func(name string, temp bool) (string, error) {
					return svc.SaveBytes(context.Background(), []byte("body"), 7, name, temp)
				}
			},
		},
		{
			name: "OSS",
			save: func(hook fileBytesPutHook) func(string, bool) (string, error) {
				svc := &ossFileService{bucketName: "main", tempClient: new(oss.Client), tempBucketName: "temporary", pathPrefix: "root/", putBytesHook: hook}
				return func(name string, temp bool) (string, error) {
					return svc.SaveBytes(context.Background(), []byte("body"), 7, name, temp)
				}
			},
		},
		{
			name: "OBS",
			save: func(hook fileBytesPutHook) func(string, bool) (string, error) {
				svc := &obsFileService{bucketName: "main", pathPrefix: "root", putBytesHook: hook}
				return func(name string, temp bool) (string, error) {
					return svc.SaveBytes(context.Background(), []byte("body"), 7, name, temp)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys, buckets []string
			hook := func(_ context.Context, bucket, key string, _ []byte) error {
				buckets = append(buckets, bucket)
				keys = append(keys, key)
				return nil
			}
			save := tt.save(hook)
			stableName := "career_source_request-1.pdf"
			_, err := save(stableName, false)
			require.NoError(t, err)
			_, err = save(stableName, false)
			require.NoError(t, err)
			require.Equal(t, keys[0], keys[1], "same Career caller name must target same provider key")
			require.Equal(t, "root/7/exports/"+stableName, keys[0])

			_, err = save("resume.pdf", false)
			require.NoError(t, err)
			_, err = save("resume.pdf", false)
			require.NoError(t, err)
			require.NotEqual(t, keys[2], keys[3], "ordinary uploads retain unique key behavior")

			_, err = save(stableName, true)
			require.NoError(t, err)
			_, err = save(stableName, true)
			require.NoError(t, err)
			require.NotEqual(t, keys[4], keys[5], "temporary uploads retain unique key behavior")
			if tt.name == "TOS" {
				require.Equal(t, "temporary", buckets[4])
				require.Equal(t, "temporary", buckets[5])
			}
		})
	}
}
